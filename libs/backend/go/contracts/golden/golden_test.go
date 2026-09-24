package golden

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The spec paths climb to the repository root; tb.ReadFixture wants them from it.
const repoRootPrefix = "../../../../../"

func loadFixture(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()
	rel, ok := strings.CutPrefix(s.path, repoRootPrefix)
	if !ok {
		t.Fatalf("fixture path %q does not climb to the repository root with %q", s.path, repoRootPrefix)
	}
	doc, err := golden.Decode(tb.ReadFixture(t, rel))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return doc
}

// subject hands the kit the same reader the generator uses (FIX-02); a payload
// the reader cannot parse reaches the oracle as DMPF-R001, not as a dead test.
func subject(s fixtureSpec) golden.Subject {
	return golden.Subject{NewMessage: s.newMessage, MessageFromFields: s.messageFromFields}
}

// eachSpec runs body once per contract, so a fixture added to specs is covered
// by every oracle without a second edit.
func eachSpec(t *testing.T, body func(t *testing.T, s fixtureSpec)) {
	t.Helper()
	for _, s := range specs {
		t.Run(s.identity.Fixture, func(t *testing.T) { body(t, s) })
	}
}

func TestFixtureIsInSyncWithGenerator(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		gotJSON, _ := json.MarshalIndent(loadFixture(t, s), "", "  ")
		wantJSON, _ := json.MarshalIndent(buildFixture(t, s), "", "  ")
		if !bytes.Equal(gotJSON, wantJSON) {
			t.Fatal("committed fixture differs from the generator; run GOLDEN_UPDATE=1 go test ./golden/ -run TestUpdateGolden and review the diff")
		}
	})
}

func TestFixtureRejectsUnknownFormatVersion(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		doc := buildFixture(t, s)
		doc.FormatVersion = "2"
		data, _ := json.Marshal(doc)
		if _, err := golden.Decode(data); !errors.Is(err, golden.ErrFormatVersion) {
			t.Fatalf("err = %v, want golden.ErrFormatVersion", err)
		}
	})
}

// FIX-11 over the real set: one canonical fixture per contract major.
func TestOneFixturePerContractMajor(t *testing.T) {
	var catalog golden.Catalog
	for _, s := range specs {
		if err := catalog.Add(s.path, loadFixture(t, s)); err != nil {
			t.Fatal(err)
		}
	}
	if catalog.Len() != len(specs) {
		t.Fatalf("catalog holds %d fixtures, want %d", catalog.Len(), len(specs))
	}
}

func TestFixtureShape(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		doc := loadFixture(t, s)
		if doc.Identity != s.identity {
			t.Fatalf("identity = %+v, want %+v", doc.Identity, s.identity)
		}
		if doc.Covers.ProfileMajor != "1" || doc.Covers.ContractMajor != "v1" {
			t.Fatalf("covers = %+v", doc.Covers)
		}
		if len(doc.Cases) != s.wantCases || len(doc.Discriminators) != s.wantDiscriminators {
			t.Fatalf("cases = %d, discriminators = %d; want %d and %d",
				len(doc.Cases), len(doc.Discriminators), s.wantCases, s.wantDiscriminators)
		}

		present := map[string]int{}
		absent := map[string]int{}
		for _, c := range doc.AllCases() {
			for _, name := range []string{"aggregateversion", "tenantid", "tracestate"} {
				if _, ok := c.Envelope[name]; ok {
					present[name]++
				} else {
					absent[name]++
				}
			}
		}
		for _, name := range []string{"aggregateversion", "tenantid", "tracestate"} {
			if present[name] == 0 || absent[name] == 0 {
				t.Errorf("conditional %q lacks a case in one of the two states (present=%d absent=%d)", name, present[name], absent[name])
			}
		}

		if s.enum.field == "" {
			return
		}
		values := map[string]bool{}
		for _, c := range doc.AllCases() {
			values[c.Payload[s.enum.field]] = true
		}
		for _, want := range s.enum.values {
			if !values[want] {
				t.Errorf("enum discriminator %q missing on field %q (ORA-08)", want, s.enum.field)
			}
		}
	})
}

// Both directions, three oracles apart (ORA-01..ORA-07): every case and
// discriminator as a consumer, every canonical case as a producer, and oracle 3
// reproves — the producer's bytes must be the fixture's (ENV-24).
func TestGoldenRoundTrip(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		doc := loadFixture(t, s)
		report := golden.Evaluate(doc, subject(s))
		if want := 3*len(doc.AllCases()) + 3*len(doc.Cases); len(report.Outcomes) != want {
			t.Fatalf("%d outcomes, want %d (three oracles per direction)", len(report.Outcomes), want)
		}
		tb.RequireReport(t, report)
		evidence.RecordReport(t, "golden", strings.ReplaceAll(s.identity.Fixture, "/", "-"), report)
	})
}

func TestUnknownFieldIsPreservedAndHashed(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		c := findCase(t, loadFixture(t, s), "unknown-field")
		transported := decodeHex(t, c.PayloadBytesHex)
		decoded := unmarshalCase(t, s, transported)
		if len(decoded.ProtoReflect().GetUnknown()) == 0 {
			t.Fatalf("unknown field %d was dropped on decode (PTB-10)", s.fieldNumbers.unknown)
		}
		if payloadhash.Sum(transported) != c.PayloadHash {
			t.Fatal("payload_hash must cover the unknown field bytes (ENV-17)")
		}
	})
}

// The ENV-18 oracle: hashing the bytes matches the fixture; hashing a
// reserialization of the decoded message does not.
func TestHashOverBytesNotOverStructure(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		c := findCase(t, loadFixture(t, s), "non-canonical-field-order")
		transported := decodeHex(t, c.PayloadBytesHex)
		if payloadhash.Sum(transported) != c.PayloadHash {
			t.Fatal("Sum(transported) must equal the declared payload_hash")
		}
		decoded := unmarshalCase(t, s, transported)
		if !proto.Equal(decoded, s.read(t, c.Payload)) {
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

func TestUnknownEnumValueDecodes(t *testing.T) {
	c := findCase(t, loadFixture(t, orderPlacedSpec), "enum-unknown-value")
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(decodeHex(t, c.PayloadBytesHex), &decoded); err != nil {
		t.Fatal(err)
	}
	if int32(decoded.GetChannel()) != 99 {
		t.Fatalf("channel = %d, want 99 preserved numerically", decoded.GetChannel())
	}
}

func TestInt64BeyondDoublePrecision(t *testing.T) {
	c := findCase(t, loadFixture(t, orderPlacedSpec), "total-cents-beyond-double")
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(decodeHex(t, c.PayloadBytesHex), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GetTotalCents() != 9007199254740993 {
		t.Fatalf("total_cents = %d, want 9007199254740993", decoded.GetTotalCents())
	}
	if c.Payload["total_cents"] != "9007199254740993" {
		t.Fatalf("fixture must carry the value as a string (FIX-07): %q", c.Payload["total_cents"])
	}
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("payload_bytes_hex: %v", err)
	}
	return b
}

func unmarshalCase(t *testing.T, s fixtureSpec, transported []byte) proto.Message {
	t.Helper()
	msg := s.newMessage()
	if err := proto.Unmarshal(transported, msg); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	return msg
}

func findCase(t *testing.T, doc fixtureDoc, name string) fixtureCase {
	t.Helper()
	c, ok := doc.FindCase(name)
	if !ok {
		t.Fatalf("case %q not in fixture", name)
	}
	return c
}
