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

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/gen/go/company/orders/event/v1"
	cloudeventsv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/gen/go/io/cloudevents/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/payloadhash"
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

func loadFixture(t *testing.T) fixtureDoc {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := parseFixture(data)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return doc
}

func TestFixtureIsInSyncWithGenerator(t *testing.T) {
	got := loadFixture(t)
	want := buildFixture(t)
	gotJSON, _ := json.MarshalIndent(got, "", "  ")
	wantJSON, _ := json.MarshalIndent(want, "", "  ")
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatal("committed fixture differs from the generator; run GOLDEN_UPDATE=1 go test ./golden/ -run TestUpdateGolden and review the diff")
	}
}

func TestFixtureRejectsUnknownFormatVersion(t *testing.T) {
	doc := buildFixture(t)
	doc.FormatVersion = "2"
	data, _ := json.Marshal(doc)
	if _, err := parseFixture(data); !errors.Is(err, errFormatVersion) {
		t.Fatalf("err = %v, want errFormatVersion", err)
	}
}

func TestFixtureShape(t *testing.T) {
	doc := loadFixture(t)
	if doc.Identity.Type != envelopeType || doc.Identity.DataSchema != dataSchema ||
		doc.Identity.Contract.Package != contractPkg || doc.Identity.Contract.Message != contractMsg {
		t.Fatalf("identity = %+v", doc.Identity)
	}
	if doc.Covers.ProfileMajor != "1" || doc.Covers.ContractMajor != "v1" {
		t.Fatalf("covers = %+v", doc.Covers)
	}
	if len(doc.Cases) != 5 || len(doc.Discriminators) != 3 {
		t.Fatalf("cases = %d, discriminators = %d; want 5 and 3", len(doc.Cases), len(doc.Discriminators))
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

	channels := map[string]bool{}
	for _, c := range allCases(doc) {
		channels[c.Payload["channel"]] = true
	}
	for _, want := range []string{"ORDER_CHANNEL_UNSPECIFIED", "ORDER_CHANNEL_WEB", "99"} {
		if !channels[want] {
			t.Errorf("enum discriminator %q missing (ORA-08)", want)
		}
	}
}

func TestGoldenCases(t *testing.T) {
	doc := loadFixture(t)
	for _, c := range allCases(doc) {
		t.Run(c.Name, func(t *testing.T) {
			transported, err := hex.DecodeString(c.PayloadBytesHex)
			if err != nil {
				t.Fatalf("payload_bytes_hex: %v", err)
			}

			if got := payloadhash.Sum(transported); got != c.PayloadHash {
				t.Fatalf("payload_hash: Go computed %s, fixture declares %s", got, c.PayloadHash)
			}

			var decoded eventv1.OrderPlaced
			if err := proto.Unmarshal(transported, &decoded); err != nil {
				t.Fatalf("payload does not decode: %v", err)
			}
			want := messageFromFields(t, c.Payload)
			if decoded.GetOrderId() != want.GetOrderId() || decoded.GetCustomerId() != want.GetCustomerId() ||
				decoded.GetTotalCents() != want.GetTotalCents() || decoded.GetChannel() != want.GetChannel() {
				t.Fatalf("decoded payload %v differs from declared fields %v", &decoded, c.Payload)
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
			if out.ID != in.ID || out.Type != in.Type || out.DataSchema != in.DataSchema ||
				!out.Time.AsTime().Equal(in.Time.AsTime()) ||
				(out.TenantID == nil) != (in.TenantID == nil) ||
				(out.TraceState == nil) != (in.TraceState == nil) ||
				(out.AggregateVersion == nil) != (in.AggregateVersion == nil) {
				t.Fatalf("envelope changed across Encode/Decode:\n in=%+v\nout=%+v", in, out)
			}

			reserialized, err := proto.MarshalOptions{Deterministic: true}.Marshal(&decoded)
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
}

func TestUnknownFieldIsPreservedAndHashed(t *testing.T) {
	c := findCase(t, loadFixture(t), "unknown-field")
	transported, _ := hex.DecodeString(c.PayloadBytesHex)
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(transported, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.ProtoReflect().GetUnknown()) == 0 {
		t.Fatal("unknown field 7 was dropped on decode (PTB-10)")
	}
	if payloadhash.Sum(transported) != c.PayloadHash {
		t.Fatal("payload_hash must cover the unknown field bytes (ENV-17)")
	}
}

func TestUnknownEnumValueDecodes(t *testing.T) {
	c := findCase(t, loadFixture(t), "enum-unknown-value")
	transported, _ := hex.DecodeString(c.PayloadBytesHex)
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(transported, &decoded); err != nil {
		t.Fatal(err)
	}
	if int32(decoded.GetChannel()) != 99 {
		t.Fatalf("channel = %d, want 99 preserved numerically", decoded.GetChannel())
	}
}

func TestInt64BeyondDoublePrecision(t *testing.T) {
	c := findCase(t, loadFixture(t), "total-cents-beyond-double")
	transported, _ := hex.DecodeString(c.PayloadBytesHex)
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(transported, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GetTotalCents() != 9007199254740993 {
		t.Fatalf("total_cents = %d, want 9007199254740993", decoded.GetTotalCents())
	}
	if c.Payload["total_cents"] != "9007199254740993" {
		t.Fatalf("fixture must carry the value as a string (FIX-07): %q", c.Payload["total_cents"])
	}
}

// The ENV-18 oracle: hashing the bytes matches the fixture; hashing a
// reserialization of the decoded message does not.
func TestHashOverBytesNotOverStructure(t *testing.T) {
	c := findCase(t, loadFixture(t), "non-canonical-field-order")
	transported, _ := hex.DecodeString(c.PayloadBytesHex)
	if payloadhash.Sum(transported) != c.PayloadHash {
		t.Fatal("Sum(transported) must equal the declared payload_hash")
	}
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(transported, &decoded); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(&decoded, messageFromFields(t, c.Payload)) {
		t.Fatalf("non-canonical bytes decoded to %v", &decoded)
	}
	reserialized, err := proto.Marshal(&decoded)
	if err != nil {
		t.Fatal(err)
	}
	if payloadhash.Sum(reserialized) == c.PayloadHash {
		t.Fatal("hash of the reserialized structure must differ from the transported hash; the oracle would not catch ENV-18 violations")
	}
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
