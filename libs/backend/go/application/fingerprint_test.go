package application_test

import (
	"bytes"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

func TestTheFingerprintKeepsItsCanonicalEncoding(t *testing.T) {
	got := usecase.NewFingerprint("op").String("ab").Int(-1).Canonical()

	golden := []byte{'s', 0x02, 'o', 'p', 's', 0x02, 'a', 'b', 'i', 0x01, 0x01}
	if !bytes.Equal(got, golden) {
		t.Fatalf("canonical = %x, want %x: a retry after a release must fingerprint like the first attempt", got, golden)
	}
}

func TestTheFingerprintIsDeterministic(t *testing.T) {
	first := usecase.NewFingerprint("orders.AddItem").String("o-1").String("sku-9").Int(2).Canonical()
	again := usecase.NewFingerprint("orders.AddItem").String("o-1").String("sku-9").Int(2).Canonical()

	if !bytes.Equal(first, again) {
		t.Fatalf("the same command encodes to %x and %x", first, again)
	}
}

func TestTheFingerprintTellsFieldBoundariesApart(t *testing.T) {
	joined := usecase.NewFingerprint("op").String("ab").String("c").Canonical()
	split := usecase.NewFingerprint("op").String("a").String("bc").Canonical()

	if bytes.Equal(joined, split) {
		t.Fatal(`("ab","c") and ("a","bc") collide; every field has to carry its length`)
	}
}

func TestTheFingerprintTellsFieldKindsApart(t *testing.T) {
	asText := usecase.NewFingerprint("op").String("1").Canonical()
	asNumber := usecase.NewFingerprint("op").Int(1).Canonical()

	if bytes.Equal(asText, asNumber) {
		t.Fatal(`"1" and 1 collide; the kind of a field is part of its encoding`)
	}
}

func TestTheFingerprintCoversTheOperation(t *testing.T) {
	add := usecase.NewFingerprint("orders.AddItem").String("o-1").Canonical()
	place := usecase.NewFingerprint("orders.PlaceOrder").String("o-1").Canonical()

	if bytes.Equal(add, place) {
		t.Fatal("two operations over the same fields collide; a key reused across operations must be a mismatch")
	}
}

func TestTheFingerprintChangesWithAnyField(t *testing.T) {
	base := usecase.NewFingerprint("op").String("o-1").Int(1).Canonical()
	for name, other := range map[string][]byte{
		"another string":    usecase.NewFingerprint("op").String("o-2").Int(1).Canonical(),
		"another number":    usecase.NewFingerprint("op").String("o-1").Int(2).Canonical(),
		"a missing field":   usecase.NewFingerprint("op").String("o-1").Canonical(),
		"a negative number": usecase.NewFingerprint("op").String("o-1").Int(-1).Canonical(),
	} {
		if bytes.Equal(other, base) {
			t.Errorf("%s encodes like the base command", name)
		}
	}
}

func TestCanonicalIsACopy(t *testing.T) {
	fingerprint := usecase.NewFingerprint("op")
	fingerprint.Canonical()[0] = 'x'

	if got := fingerprint.Canonical()[0]; got != 's' {
		t.Fatalf("canonical[0] = %q after the caller wrote to its copy, want 's'", got)
	}
}
