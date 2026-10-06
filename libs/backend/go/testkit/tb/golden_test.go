package tb_test

import (
	"encoding/hex"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var fieldSpec = tb.Spec[*descriptorpb.FieldDescriptorProto]{
	Identity: golden.Identity{
		Fixture:    "kit/v1/field",
		Contract:   golden.Contract{Package: "google.protobuf", Message: "FieldDescriptorProto"},
		Type:       "kit.field.v1",
		DataSchema: "type.googleapis.com/google.protobuf.FieldDescriptorProto",
	},
	Source:  "urn:dmpf:kit",
	Subject: "field/sku",
	Unknown: 999,
	New:     func() *descriptorpb.FieldDescriptorProto { return &descriptorpb.FieldDescriptorProto{} },
	FromFields: func(fields map[string]string) (*descriptorpb.FieldDescriptorProto, error) {
		number, err := tb.ParseInt(fields, "number", 32)
		if err != nil {
			return nil, err
		}
		return &descriptorpb.FieldDescriptorProto{
			Name:    proto.String(fields["name"]),
			Number:  proto.Int32(int32(number)),
			Options: &descriptorpb.FieldOptions{Deprecated: proto.Bool(true)},
		}, nil
	},
}

var fieldFields = map[string]string{"name": "sku", "number": "3"}

func payloadOf(t *testing.T, c golden.Case) []byte {
	t.Helper()
	b, err := hex.DecodeString(c.PayloadBytesHex)
	if err != nil {
		t.Fatal(err)
	}
	if payloadhash.Sum(b) != c.PayloadHash {
		t.Fatal("payload_hash does not cover the payload bytes")
	}
	return b
}

func TestNonCanonicalCaseCoversAMessageField(t *testing.T) {
	c := fieldSpec.NonCanonicalCase(t, "non-canonical-field-order", "doc", fieldSpec.BaseEnvelope(), fieldFields)
	transported := payloadOf(t, c)

	var decoded descriptorpb.FieldDescriptorProto
	if err := proto.Unmarshal(transported, &decoded); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(&decoded, fieldSpec.Read(t, fieldFields)) {
		t.Fatalf("decoded = %v, want the message the fields describe", &decoded)
	}
	reserialized, err := proto.Marshal(&decoded)
	if err != nil {
		t.Fatal(err)
	}
	if payloadhash.Sum(reserialized) == c.PayloadHash {
		t.Fatal("the non-canonical bytes hash like a reserialization (ENV-18)")
	}
}

func TestUnknownFieldCaseKeepsTheUndeclaredNumber(t *testing.T) {
	c := fieldSpec.UnknownFieldCase(t, "unknown-field", "doc", fieldSpec.BaseEnvelope(), fieldFields)

	var decoded descriptorpb.FieldDescriptorProto
	if err := proto.Unmarshal(payloadOf(t, c), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.ProtoReflect().GetUnknown()) == 0 {
		t.Fatal("the unknown field was dropped on decode (PTB-10)")
	}
}

func TestFixtureAndEnvelopeCarryTheIdentity(t *testing.T) {
	env := tb.WithConditionals(fieldSpec.BaseEnvelope())
	packed := fieldSpec.PackedCase(t, "all-conditionals-present", "doc", env, fieldFields)

	doc := fieldSpec.Fixture([]golden.Case{packed}, nil)

	if doc.FormatVersion != golden.FormatVersion || doc.Identity != fieldSpec.Identity || doc.Covers.ContractMajor != "v1" {
		t.Fatalf("fixture = %+v, want the spec identity in the kit format", doc)
	}
	if env["type"] != "kit.field.v1" || env["dataschema"] != fieldSpec.Identity.DataSchema || env["tenantid"] != "tenant-a" {
		t.Fatalf("envelope = %v, want the spec type and schema with the conditionals", env)
	}
}

func TestParseIntNamesTheField(t *testing.T) {
	if _, err := tb.ParseInt(map[string]string{"number": "x"}, "number", 32); err == nil || errors.Unwrap(err) == nil {
		t.Fatalf("ParseInt() = %v, want the parse error wrapped with the field", err)
	}
}

func TestTheBaseEnvelopeCarriesTheSourceAndSubjectOfTheSpec(t *testing.T) {
	env := fieldSpec.BaseEnvelope()

	if env["source"] != "urn:dmpf:kit" || env["subject"] != "field/sku" {
		t.Fatalf("BaseEnvelope() source = %q, subject = %q, want the spec's", env["source"], env["subject"])
	}
}
