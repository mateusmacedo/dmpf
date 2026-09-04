package golden

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"google.golang.org/protobuf/proto"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	cloudeventsv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/io/cloudevents/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
)

var errFormatVersion = errors.New("golden: unsupported format_version")

// parseFixture fails on any format_version it does not know (FIX-09): reading a
// newer fixture with an older loader would report a conformance nobody checked.
func parseFixture(data []byte) (fixtureDoc, error) {
	var doc fixtureDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if doc.FormatVersion != formatVersion {
		return doc, fmt.Errorf("%w: %q", errFormatVersion, doc.FormatVersion)
	}
	return doc, nil
}

func loadFixture(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := parseFixture(data)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return doc
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
		if _, err := parseFixture(data); !errors.Is(err, errFormatVersion) {
			t.Fatalf("err = %v, want errFormatVersion", err)
		}
	})
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
		for _, c := range allCases(doc) {
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
		for _, c := range allCases(doc) {
			values[c.Payload[s.enum.field]] = true
		}
		for _, want := range s.enum.values {
			if !values[want] {
				t.Errorf("enum discriminator %q missing on field %q (ORA-08)", want, s.enum.field)
			}
		}
	})
}

func TestGoldenCases(t *testing.T) {
	eachSpec(t, func(t *testing.T, s fixtureSpec) {
		for _, c := range allCases(loadFixture(t, s)) {
			t.Run(c.Name, func(t *testing.T) {
				transported := decodeHex(t, c.PayloadBytesHex)

				if got := payloadhash.Sum(transported); got != c.PayloadHash {
					t.Fatalf("payload_hash: Go computed %s, fixture declares %s", got, c.PayloadHash)
				}

				decoded := unmarshalCase(t, s, transported)
				// The unknown-field discriminator carries bytes the contract does not
				// declare, while the payload map describes only the known fields.
				known := proto.Clone(decoded)
				known.ProtoReflect().SetUnknown(nil)
				if want := s.messageFromFields(t, c.Payload); !proto.Equal(known, want) {
					t.Fatalf("decoded payload %v differs from declared fields %v", known, want)
				}

				in := envelopeFromFields(t, c.Envelope, transported)
				ce, err := envelope.Encode(in)
				if err != nil {
					t.Fatalf("Encode: %v", err)
				}
				wire, err := proto.Marshal(ce)
				if err != nil {
					t.Fatalf("Marshal envelope: %v", err)
				}
				var back cloudeventsv1.CloudEvent
				if err := proto.Unmarshal(wire, &back); err != nil {
					t.Fatalf("Unmarshal envelope: %v", err)
				}
				out, err := envelope.Decode(&back)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				if !bytes.Equal(out.Payload, transported) {
					t.Fatal("Any.value changed across Encode/Decode")
				}
				if payloadhash.Sum(out.Payload) != c.PayloadHash {
					t.Fatal("payload_hash changed across Encode/Decode")
				}
				if out.ID != in.ID || out.Source != in.Source || out.SpecVersion != in.SpecVersion ||
					out.Type != in.Type || out.Subject != in.Subject || out.DataSchema != in.DataSchema ||
					out.DataContentType != in.DataContentType || out.CorrelationID != in.CorrelationID ||
					out.CausationID != in.CausationID || out.PartitionKey != in.PartitionKey ||
					out.TraceParent != in.TraceParent || !out.Time.AsTime().Equal(in.Time.AsTime()) ||
					!sameInt32(out.AggregateVersion, in.AggregateVersion) ||
					!sameString(out.TenantID, in.TenantID) || !sameString(out.TraceState, in.TraceState) {
					t.Fatalf("envelope changed across Encode/Decode:\n in=%+v\nout=%+v", in, out)
				}

				reserialized, err := proto.MarshalOptions{Deterministic: true}.Marshal(decoded)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(reserialized, transported) {
					t.Logf("oracle 3 (byte identity): reserialization matches the transported bytes")
				} else {
					t.Logf("oracle 3 (byte identity): reserialization differs from the transported bytes (informative, §8.3)")
				}
			})
		}
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
		if !proto.Equal(decoded, s.messageFromFields(t, c.Payload)) {
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

func sameString(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameInt32(a, b *int32) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func allCases(doc fixtureDoc) []fixtureCase {
	return append(append([]fixtureCase(nil), doc.Cases...), doc.Discriminators...)
}

func findCase(t *testing.T, doc fixtureDoc, name string) fixtureCase {
	t.Helper()
	for _, c := range allCases(doc) {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not in fixture", name)
	return fixtureCase{}
}
