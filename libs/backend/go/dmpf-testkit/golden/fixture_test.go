package golden_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/golden"
)

const itemAddedPath = "../../../../../contracts/fixtures/orders/event/v1/item-added.golden"

func readItemAdded(t *testing.T) golden.Fixture {
	t.Helper()
	raw, err := os.ReadFile(itemAddedPath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := golden.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return f
}

func TestDecodeReadsTheCommittedFixture(t *testing.T) {
	f := readItemAdded(t)
	if f.Identity.Contract.Message != "ItemAdded" || f.Covers.ContractMajor != "v1" {
		t.Fatalf("identity = %+v covers = %+v", f.Identity, f.Covers)
	}
	if len(f.Cases) != 3 || len(f.Discriminators) != 2 || len(f.AllCases()) != 5 {
		t.Fatalf("cases = %d, discriminators = %d", len(f.Cases), len(f.Discriminators))
	}
	if _, ok := f.FindCase("unknown-field"); !ok {
		t.Fatal("FindCase did not see a discriminator")
	}
}

func TestDecodeRejectsUnknownFormatVersion(t *testing.T) {
	_, err := golden.Decode([]byte(`{"format_version": "2", "identity": {}, "cases": []}`))
	if !errors.Is(err, golden.ErrFormatVersion) || !strings.Contains(err.Error(), `"2"`) {
		t.Fatalf("err = %v, want ErrFormatVersion naming the version", err)
	}
}

func TestDecodeRejectsANonStringScalarNamingItsPath(t *testing.T) {
	doc := `{"format_version": "1", "identity": {"fixture": "x"}, "cases": [{"name": "a", "payload": {"quantity": 2}}]}`
	_, err := golden.Decode([]byte(doc))
	if !errors.Is(err, golden.ErrNonStringScalar) || !strings.Contains(err.Error(), "cases[0].payload.quantity") {
		t.Fatalf("err = %v, want ErrNonStringScalar at cases[0].payload.quantity", err)
	}
	_, err = golden.Decode([]byte(`{"format_version": 1}`))
	if !errors.Is(err, golden.ErrNonStringScalar) || !strings.Contains(err.Error(), "format_version") {
		t.Fatalf("err = %v, want ErrNonStringScalar at format_version", err)
	}
}
