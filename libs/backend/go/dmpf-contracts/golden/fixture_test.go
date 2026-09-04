package golden

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
)

const formatVersion = "1"

// Every scalar is a JSON string (FIX-07): int64 and timestamps must survive a
// TypeScript reader without going through a double.
type fixtureDoc struct {
	FormatVersion  string        `json:"format_version"`
	Identity       identity      `json:"identity"`
	Covers         covers        `json:"covers"`
	Cases          []fixtureCase `json:"cases"`
	Discriminators []fixtureCase `json:"discriminators"`
}

type identity struct {
	Fixture    string   `json:"fixture"`
	Contract   contract `json:"contract"`
	Type       string   `json:"type"`
	DataSchema string   `json:"dataschema"`
}

type contract struct {
	Package string `json:"package"`
	Message string `json:"message"`
}

type covers struct {
	ProfileMajor  string `json:"profile_major"`
	ContractMajor string `json:"contract_major"`
}

type fixtureCase struct {
	Name            string            `json:"name"`
	Doc             string            `json:"doc"`
	Envelope        map[string]string `json:"envelope"`
	Payload         map[string]string `json:"payload"`
	PayloadBytesHex string            `json:"payload_bytes_hex"`
	PayloadHash     string            `json:"payload_hash"`
}

// fixtureSpec is one contract in the suite: where its fixture lives, what it
// declares, and how the shared oracles read its string-typed payload back.
type fixtureSpec struct {
	path               string
	identity           identity
	fieldNumbers       fieldNumbers
	enum               enumDiscriminator
	newMessage         func() proto.Message
	messageFromFields  func(t *testing.T, fields map[string]string) proto.Message
	build              func(t *testing.T, s fixtureSpec) fixtureDoc
	wantCases          int
	wantDiscriminators int
}

// unknown must be a number the contract never declares, so the discriminator
// stays unknown to the decoder no matter how the message grows (PTB-10).
type fieldNumbers struct {
	unknown protowire.Number
}

// enumDiscriminator names the payload field whose values cover the ORA-08
// oracle. A contract without an enum leaves it zero and the check is skipped.
type enumDiscriminator struct {
	field  string
	values []string
}

var specs = []fixtureSpec{orderPlacedSpec, itemAddedSpec}

func parseInt(t *testing.T, fields map[string]string, name string, bitSize int) int64 {
	t.Helper()
	v, err := strconv.ParseInt(fields[name], 10, bitSize)
	if err != nil {
		t.Fatalf("%s %q: %v", name, fields[name], err)
	}
	return v
}

func (s fixtureSpec) baseEnvelope() map[string]string {
	return map[string]string{
		"id":              "evt-0001",
		"source":          "urn:lidercap:orders",
		"specversion":     envelope.SpecVersion,
		"type":            s.identity.Type,
		"subject":         "order/o-1001",
		"time":            "2026-09-02T12:00:00Z",
		"dataschema":      s.identity.DataSchema,
		"datacontenttype": envelope.ContentType,
		"correlationid":   "corr-0001",
		"causationid":     "evt-0001",
		"partitionkey":    "c-42",
		"traceparent":     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}
}

func withConditionals(env map[string]string) map[string]string {
	env["aggregateversion"] = "7"
	env["tenantid"] = "tenant-a"
	env["tracestate"] = "vendor=1"
	return env
}

func envelopeFromFields(t *testing.T, fields map[string]string, payload []byte) envelope.Envelope {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, fields["time"])
	if err != nil {
		t.Fatalf("time %q: %v", fields["time"], err)
	}
	e := envelope.Envelope{
		ID:              fields["id"],
		Source:          fields["source"],
		SpecVersion:     fields["specversion"],
		Type:            fields["type"],
		Subject:         fields["subject"],
		Time:            timestamppb.New(ts),
		DataSchema:      fields["dataschema"],
		DataContentType: fields["datacontenttype"],
		CorrelationID:   fields["correlationid"],
		CausationID:     fields["causationid"],
		PartitionKey:    fields["partitionkey"],
		TraceParent:     fields["traceparent"],
		Payload:         payload,
	}
	if v, ok := fields["aggregateversion"]; ok {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			t.Fatalf("aggregateversion %q: %v", v, err)
		}
		av := int32(n)
		e.AggregateVersion = &av
	}
	if v, ok := fields["tenantid"]; ok {
		tenant := v
		e.TenantID = &tenant
	}
	if v, ok := fields["tracestate"]; ok {
		state := v
		e.TraceState = &state
	}
	return e
}

func (s fixtureSpec) packedCase(t *testing.T, name, doc string, env, fields map[string]string) fixtureCase {
	t.Helper()
	payload, typeURL, err := envelope.Pack(s.messageFromFields(t, fields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if typeURL != s.identity.DataSchema {
		t.Fatalf("Pack type URL = %q, want %q", typeURL, s.identity.DataSchema)
	}
	return rawCase(name, doc, env, fields, payload)
}

func (s fixtureSpec) unknownFieldCase(t *testing.T, name, doc string, env, fields map[string]string) fixtureCase {
	t.Helper()
	canonical, _, err := envelope.Pack(s.messageFromFields(t, fields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	payload := appendVarint(append([]byte(nil), canonical...), s.fieldNumbers.unknown, 42)
	return rawCase(name, doc, env, fields, payload)
}

func (s fixtureSpec) nonCanonicalCase(t *testing.T, name, doc string, env, fields map[string]string) fixtureCase {
	t.Helper()
	return rawCase(name, doc, env, fields, nonCanonicalPayload(t, s.messageFromFields(t, fields)))
}

func rawCase(name, doc string, env, fields map[string]string, payload []byte) fixtureCase {
	return fixtureCase{
		Name:            name,
		Doc:             doc,
		Envelope:        env,
		Payload:         fields,
		PayloadBytesHex: hex.EncodeToString(payload),
		PayloadHash:     payloadhash.Sum(payload),
	}
}

func appendString(b []byte, num protowire.Number, v string) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendString(b, v)
}

func appendVarint(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// Fields emitted in descending number order: valid wire, same decoded message,
// but never what a Go reserialization produces — the ENV-18 discriminator.
func nonCanonicalPayload(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	type populated struct {
		descriptor protoreflect.FieldDescriptor
		value      protoreflect.Value
	}
	var fields []populated
	msg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		fields = append(fields, populated{fd, v})
		return true
	})
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].descriptor.Number() > fields[j].descriptor.Number()
	})
	if len(fields) < 2 {
		t.Fatalf("non-canonical order needs at least two populated fields, got %d", len(fields))
	}
	var b []byte
	for _, f := range fields {
		num := f.descriptor.Number()
		switch f.descriptor.Kind() {
		case protoreflect.StringKind:
			b = appendString(b, num, f.value.String())
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			b = appendVarint(b, num, uint64(f.value.Int()))
		case protoreflect.EnumKind:
			b = appendVarint(b, num, uint64(f.value.Enum()))
		default:
			t.Fatalf("field %d: kind %v is not handled by the non-canonical builder", num, f.descriptor.Kind())
		}
	}
	return b
}

func buildFixture(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()
	return s.build(t, s)
}

// TestUpdateGolden rewrites the fixtures from code; it only runs with GOLDEN_UPDATE=1
// so the committed files stay the reviewed oracle, never a side effect of `go test`.
func TestUpdateGolden(t *testing.T) {
	if os.Getenv("GOLDEN_UPDATE") != "1" {
		t.Skip("set GOLDEN_UPDATE=1 to regenerate the golden fixtures")
	}
	for _, s := range specs {
		t.Run(s.identity.Fixture, func(t *testing.T) {
			data, err := json.MarshalIndent(buildFixture(t, s), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, '\n')
			if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("golden fixture written to %s (%d bytes)", s.path, len(data))
		})
	}
}
