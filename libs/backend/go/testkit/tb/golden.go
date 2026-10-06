package tb

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
)

// Enum names the payload field whose values cover the ORA-08 oracle. A
// contract without an enum leaves it zero and the check is skipped.
type Enum struct {
	Field  string
	Values []string
}

// Spec is one contract in a golden suite: where its fixture lives, relative to
// the repository root, what it declares, and how the generator and the oracles
// read its string-typed payload back (FIX-02).
type Spec[M proto.Message] struct {
	Path     string
	Identity golden.Identity
	Source   string
	Subject  string
	// Unknown must be a number the contract never declares, so the
	// discriminator stays unknown to the decoder however the message grows (PTB-10).
	Unknown            protowire.Number
	Enum               Enum
	New                func() M
	FromFields         func(fields map[string]string) (M, error)
	Build              func(t *testing.T, s Spec[M]) golden.Fixture
	WantCases          int
	WantDiscriminators int
}

func (s Spec[M]) Load(t testing.TB) golden.Fixture {
	t.Helper()
	doc, err := golden.Decode(ReadFixture(t, s.Path))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return doc
}

// Read is the generator's reading of the payload: a fixture the generator
// cannot build is a defect of the spec, so it fails the test. The oracles use
// the same reader through golden.Subject and get the error instead.
func (s Spec[M]) Read(t testing.TB, fields map[string]string) M {
	t.Helper()
	msg, err := s.FromFields(fields)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func (s Spec[M]) BaseEnvelope() map[string]string {
	return map[string]string{
		"id":              "evt-0001",
		"source":          s.Source,
		"specversion":     envelope.SpecVersion,
		"type":            s.Identity.Type,
		"subject":         s.Subject,
		"time":            "2026-09-02T12:00:00Z",
		"dataschema":      s.Identity.DataSchema,
		"datacontenttype": envelope.ContentType,
		"correlationid":   "corr-0001",
		"causationid":     "evt-0001",
		"partitionkey":    "c-42",
		"traceparent":     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}
}

func (s Spec[M]) Fixture(cases, discriminators []golden.Case) golden.Fixture {
	return golden.Fixture{
		FormatVersion:  golden.FormatVersion,
		Identity:       s.Identity,
		Covers:         golden.Covers{ProfileMajor: "1", ContractMajor: "v1"},
		Cases:          cases,
		Discriminators: discriminators,
	}
}

func WithConditionals(env map[string]string) map[string]string {
	env["aggregateversion"] = "7"
	env["tenantid"] = "tenant-a"
	env["tracestate"] = "vendor=1"
	return env
}

func ParseInt(fields map[string]string, name string, bitSize int) (int64, error) {
	v, err := strconv.ParseInt(fields[name], 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("%s %q: %w", name, fields[name], err)
	}
	return v, nil
}

func (s Spec[M]) PackedCase(t testing.TB, name, doc string, env, fields map[string]string) golden.Case {
	t.Helper()
	payload, typeURL, err := envelope.Pack(s.Read(t, fields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if typeURL != s.Identity.DataSchema {
		t.Fatalf("Pack type URL = %q, want %q", typeURL, s.Identity.DataSchema)
	}
	return rawCase(name, doc, env, fields, payload)
}

func (s Spec[M]) UnknownFieldCase(t testing.TB, name, doc string, env, fields map[string]string) golden.Case {
	t.Helper()
	canonical, _, err := envelope.Pack(s.Read(t, fields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	payload := appendVarint(append([]byte(nil), canonical...), s.Unknown, 42)
	return rawCase(name, doc, env, fields, payload)
}

func (s Spec[M]) NonCanonicalCase(t testing.TB, name, doc string, env, fields map[string]string) golden.Case {
	t.Helper()
	return rawCase(name, doc, env, fields, nonCanonicalPayload(t, s.Read(t, fields)))
}

func (s Spec[M]) subject() golden.Subject {
	return golden.Subject{
		NewMessage:        func() proto.Message { return s.New() },
		MessageFromFields: func(fields map[string]string) (proto.Message, error) { return s.FromFields(fields) },
	}
}

func (s Spec[M]) unmarshal(t testing.TB, transported []byte) M {
	t.Helper()
	msg := s.New()
	if err := proto.Unmarshal(transported, msg); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	return msg
}

func rawCase(name, doc string, env, fields map[string]string, payload []byte) golden.Case {
	return golden.Case{
		Name:            name,
		Doc:             doc,
		Envelope:        env,
		Payload:         fields,
		PayloadBytesHex: hex.EncodeToString(payload),
		PayloadHash:     payloadhash.Sum(payload),
	}
}

func appendVarint(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// Fields emitted in descending number order — a lone field emitted twice, since
// one field has no order to invert: valid wire, same decoded message, but never
// what a Go reserialization produces — the ENV-18 discriminator.
func nonCanonicalPayload(t testing.TB, msg proto.Message) []byte {
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
			b = protowire.AppendTag(b, num, protowire.BytesType)
			b = protowire.AppendString(b, f.value.String())
		case protoreflect.Int32Kind, protoreflect.Int64Kind:
			b = appendVarint(b, num, uint64(f.value.Int()))
		case protoreflect.EnumKind:
			b = appendVarint(b, num, uint64(f.value.Enum()))
		case protoreflect.MessageKind:
			nested, err := proto.Marshal(f.value.Message().Interface())
			if err != nil {
				t.Fatalf("field %d: %v", num, err)
			}
			b = protowire.AppendTag(b, num, protowire.BytesType)
			b = protowire.AppendBytes(b, nested)
		default:
			t.Fatalf("field %d: kind %v is not handled by the non-canonical builder", num, f.descriptor.Kind())
		}
	}
	return b
}

// GoldenSuite runs the golden oracles over every spec, so a contract added to
// specs is covered by all of them without a second edit. record receives each
// round-trip report: the evidence package imports tb, so the caller records.
func GoldenSuite[M proto.Message](t *testing.T, record func(t testing.TB, name string, r golden.Report), specs ...Spec[M]) {
	t.Helper()
	each := func(name string, body func(t *testing.T, s Spec[M])) {
		t.Run(name, func(t *testing.T) {
			for _, s := range specs {
				t.Run(s.Identity.Fixture, func(t *testing.T) { body(t, s) })
			}
		})
	}

	each("FixtureIsInSyncWithGenerator", func(t *testing.T, s Spec[M]) {
		gotJSON, _ := json.MarshalIndent(s.Load(t), "", "  ")
		wantJSON, _ := json.MarshalIndent(s.Build(t, s), "", "  ")
		if !bytes.Equal(gotJSON, wantJSON) {
			t.Fatal("committed fixture differs from the generator; run GOLDEN_UPDATE=1 go test -run TestUpdateGolden and review the diff")
		}
	})

	each("FixtureRejectsUnknownFormatVersion", func(t *testing.T, s Spec[M]) {
		doc := s.Build(t, s)
		doc.FormatVersion = "2"
		data, _ := json.Marshal(doc)
		if _, err := golden.Decode(data); !errors.Is(err, golden.ErrFormatVersion) {
			t.Fatalf("err = %v, want golden.ErrFormatVersion", err)
		}
	})

	t.Run("OneFixturePerContractMajor", func(t *testing.T) {
		var catalog golden.Catalog
		for _, s := range specs {
			if err := catalog.Add(s.Path, s.Load(t)); err != nil {
				t.Fatal(err)
			}
		}
		if catalog.Len() != len(specs) {
			t.Fatalf("catalog holds %d fixtures, want %d", catalog.Len(), len(specs))
		}
	})

	each("FixtureShape", func(t *testing.T, s Spec[M]) { requireShape(t, s.Load(t), s) })

	each("RoundTrip", func(t *testing.T, s Spec[M]) {
		doc := s.Load(t)
		report := golden.Evaluate(doc, s.subject())
		if want := 3*len(doc.AllCases()) + 3*len(doc.Cases); len(report.Outcomes) != want {
			t.Fatalf("%d outcomes, want %d (three oracles per direction)", len(report.Outcomes), want)
		}
		RequireReport(t, report)
		if record != nil {
			record(t, strings.ReplaceAll(s.Identity.Fixture, "/", "-"), report)
		}
	})

	each("UnknownFieldIsPreservedAndHashed", func(t *testing.T, s Spec[M]) {
		c := requireCase(t, s.Load(t), "unknown-field")
		transported := decodeHex(t, c.PayloadBytesHex)
		if len(s.unmarshal(t, transported).ProtoReflect().GetUnknown()) == 0 {
			t.Fatalf("unknown field %d was dropped on decode (PTB-10)", s.Unknown)
		}
		if payloadhash.Sum(transported) != c.PayloadHash {
			t.Fatal("payload_hash must cover the unknown field bytes (ENV-17)")
		}
	})

	each("HashOverBytesNotOverStructure", func(t *testing.T, s Spec[M]) {
		c := requireCase(t, s.Load(t), "non-canonical-field-order")
		transported := decodeHex(t, c.PayloadBytesHex)
		if payloadhash.Sum(transported) != c.PayloadHash {
			t.Fatal("Sum(transported) must equal the declared payload_hash")
		}
		decoded := s.unmarshal(t, transported)
		if !proto.Equal(decoded, s.Read(t, c.Payload)) {
			t.Fatalf("non-canonical bytes decoded to %v", decoded)
		}
		reserialized, err := proto.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if payloadhash.Sum(reserialized) == c.PayloadHash {
			t.Fatal("hash of the reserialized structure must differ from the transported hash; the oracle would not catch ENV-18 violations")
		}
	})
}

// UpdateGolden rewrites the fixtures from code; it only runs with GOLDEN_UPDATE=1
// so the committed files stay the reviewed oracle, never a side effect of `go test`.
func UpdateGolden[M proto.Message](t *testing.T, specs ...Spec[M]) {
	t.Helper()
	if os.Getenv("GOLDEN_UPDATE") != "1" {
		t.Skip("set GOLDEN_UPDATE=1 to regenerate the golden fixtures")
	}
	for _, s := range specs {
		t.Run(s.Identity.Fixture, func(t *testing.T) {
			data, err := json.MarshalIndent(s.Build(t, s), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, '\n')
			path := filepath.Join(RepoRoot(t), filepath.FromSlash(s.Path))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("golden fixture written to %s (%d bytes)", s.Path, len(data))
		})
	}
}

func requireShape[M proto.Message](t *testing.T, doc golden.Fixture, s Spec[M]) {
	t.Helper()
	if doc.Identity != s.Identity {
		t.Fatalf("identity = %+v, want %+v", doc.Identity, s.Identity)
	}
	if doc.Covers.ProfileMajor != "1" || doc.Covers.ContractMajor != "v1" {
		t.Fatalf("covers = %+v", doc.Covers)
	}
	if len(doc.Cases) != s.WantCases || len(doc.Discriminators) != s.WantDiscriminators {
		t.Fatalf("cases = %d, discriminators = %d; want %d and %d",
			len(doc.Cases), len(doc.Discriminators), s.WantCases, s.WantDiscriminators)
	}

	conditionals := []string{"aggregateversion", "tenantid", "tracestate"}
	present := map[string]int{}
	absent := map[string]int{}
	for _, c := range doc.AllCases() {
		for _, name := range conditionals {
			if _, ok := c.Envelope[name]; ok {
				present[name]++
			} else {
				absent[name]++
			}
		}
	}
	for _, name := range conditionals {
		if present[name] == 0 || absent[name] == 0 {
			t.Errorf("conditional %q lacks a case in one of the two states (present=%d absent=%d)", name, present[name], absent[name])
		}
	}

	if s.Enum.Field == "" {
		return
	}
	values := map[string]bool{}
	for _, c := range doc.AllCases() {
		values[c.Payload[s.Enum.Field]] = true
	}
	for _, want := range s.Enum.Values {
		if !values[want] {
			t.Errorf("enum discriminator %q missing on field %q (ORA-08)", want, s.Enum.Field)
		}
	}
}

func requireCase(t testing.TB, doc golden.Fixture, name string) golden.Case {
	t.Helper()
	c, ok := doc.FindCase(name)
	if !ok {
		t.Fatalf("case %q not in fixture", name)
	}
	return c
}

func decodeHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("payload_bytes_hex: %v", err)
	}
	return b
}
