package golden

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
)

// The fixture shape is the kit's (FIX-02): the generator writes exactly what
// golden.Decode reads, so the committed file stays the single source.
const formatVersion = golden.FormatVersion

type (
	fixtureDoc  = golden.Fixture
	identity    = golden.Identity
	contract    = golden.Contract
	covers      = golden.Covers
	fixtureCase = golden.Case
)

// fixtureSpec is one contract in the suite: where its fixture lives, what it
// declares, and how the shared oracles read its string-typed payload back.
type fixtureSpec struct {
	path               string
	identity           identity
	fieldNumbers       fieldNumbers
	enum               enumDiscriminator
	newMessage         func() proto.Message
	messageFromFields  func(fields map[string]string) (proto.Message, error)
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

var specs = []fixtureSpec{orderPlacedSpec, itemAddedSpec, reservationConfirmedSpec, reservationCancelledSpec}

func parseInt(fields map[string]string, name string, bitSize int) (int64, error) {
	v, err := strconv.ParseInt(fields[name], 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("%s %q: %w", name, fields[name], err)
	}
	return v, nil
}

// read is the generator's reading of the string-typed payload: a fixture the
// generator cannot build is a defect of the spec, so it fails the test. The
// oracles use the same reader through golden.Subject and get the error instead.
func (s fixtureSpec) read(t *testing.T, fields map[string]string) proto.Message {
	t.Helper()
	msg, err := s.messageFromFields(fields)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func (s fixtureSpec) baseEnvelope() map[string]string {
	return map[string]string{
		"id":              "evt-0001",
		"source":          "urn:dmpf:orders",
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

func (s fixtureSpec) packedCase(t *testing.T, name, doc string, env, fields map[string]string) fixtureCase {
	t.Helper()
	payload, typeURL, err := envelope.Pack(s.read(t, fields))
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
	canonical, _, err := envelope.Pack(s.read(t, fields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	payload := appendVarint(append([]byte(nil), canonical...), s.fieldNumbers.unknown, 42)
	return rawCase(name, doc, env, fields, payload)
}

func (s fixtureSpec) nonCanonicalCase(t *testing.T, name, doc string, env, fields map[string]string) fixtureCase {
	t.Helper()
	return rawCase(name, doc, env, fields, nonCanonicalPayload(t, s.read(t, fields)))
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

// Fields emitted in descending number order — a lone field emitted twice, since
// one field has no order to invert: valid wire, same decoded message, but never
// what a Go reserialization produces — the ENV-18 discriminator.
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
	switch len(fields) {
	case 0:
		t.Fatal("non-canonical encoding needs at least one populated field")
	case 1:
		fields = append(fields, fields[0])
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
