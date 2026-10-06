package golden

import (
	"encoding/hex"
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var specs = []tb.Spec[proto.Message]{orderPlacedSpec, itemAddedSpec}

func TestGolden(t *testing.T) {
	tb.GoldenSuite(t, specs...)
}

func TestUpdateGolden(t *testing.T) { tb.UpdateGolden(t, specs...) }

func TestUnknownEnumValueDecodes(t *testing.T) {
	c := findCase(t, orderPlacedSpec.Load(t), "enum-unknown-value")
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(decodeHex(t, c.PayloadBytesHex), &decoded); err != nil {
		t.Fatal(err)
	}
	if int32(decoded.GetChannel()) != 99 {
		t.Fatalf("channel = %d, want 99 preserved numerically", decoded.GetChannel())
	}
}

func TestInt64BeyondDoublePrecision(t *testing.T) {
	c := findCase(t, orderPlacedSpec.Load(t), "total-cents-beyond-double")
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

func findCase(t *testing.T, doc golden.Fixture, name string) golden.Case {
	t.Helper()
	c, ok := doc.FindCase(name)
	if !ok {
		t.Fatalf("case %q not in fixture", name)
	}
	return c
}
