package golden_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
)

func TestCatalogRefusesASecondFixtureForTheSameContractMajor(t *testing.T) {
	f := readItemAdded(t)
	var c golden.Catalog
	if err := c.Add("orders/event/v1/item-added.golden", f); err != nil {
		t.Fatal(err)
	}
	other := f
	other.Identity.Fixture = "orders/event/v1/item-added-copy"
	err := c.Add("orders/event/v1/item-added-copy.golden", other)
	if !errors.Is(err, golden.ErrAmbiguousFixture) {
		t.Fatalf("err = %v, want ErrAmbiguousFixture", err)
	}
	for _, path := range []string{"item-added.golden", "item-added-copy.golden"} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error does not name %s: %v", path, err)
		}
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d after a refused Add", c.Len())
	}
}

func TestCatalogKeepsDistinctMajorsApart(t *testing.T) {
	f := readItemAdded(t)
	v2 := f
	v2.Covers.ContractMajor = "v2"
	var c golden.Catalog
	if err := errors.Join(c.Add("v1.golden", f), c.Add("v2.golden", v2)); err != nil {
		t.Fatal(err)
	}
	if _, path, ok := c.Lookup("company.orders.event.v1", "ItemAdded", "v2"); !ok || path != "v2.golden" {
		t.Fatalf("Lookup v2 = (%q, %v)", path, ok)
	}
	if keys := c.Keys(); len(keys) != 2 || keys[0] != "company.orders.event.v1.ItemAdded@v1" {
		t.Fatalf("Keys = %v", keys)
	}
}
