package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func sameSequence(t *testing.T, a, b []kernel.DomainEvent) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("event sequences differ in length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("event %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestAddItemIsDeterministic(t *testing.T) {
	a := newOpenOrder(t, 2, 3)
	b := newOpenOrder(t, 2, 3)
	cmd := domain.AddItem{SKU: "ABC", Quantity: 1, At: at}

	accA, rejA := a.AddItem(cmd)
	accB, rejB := b.AddItem(cmd)

	if rejA != nil || rejB != nil {
		t.Fatalf("expected both Accepted: %v / %v", rejA, rejB)
	}
	if accA.Response() != accB.Response() {
		t.Fatalf("responses differ: %+v vs %+v", accA.Response(), accB.Response())
	}
	sameSequence(t, accA.Events(), accB.Events())
	if !a.Snapshot().Equal(b.Snapshot()) {
		t.Fatalf("final snapshots differ:\n%+v\n%+v", a.Snapshot(), b.Snapshot())
	}
}

func TestPlaceIsDeterministic(t *testing.T) {
	a := newOpenOrder(t, 2, 3)
	b := newOpenOrder(t, 2, 3)
	cmd := domain.PlaceOrder{At: at}

	accA, rejA := a.Place(cmd)
	accB, rejB := b.Place(cmd)

	if rejA != nil || rejB != nil {
		t.Fatalf("expected both Accepted: %v / %v", rejA, rejB)
	}
	if accA.Response() != accB.Response() {
		t.Fatalf("responses differ: %+v vs %+v", accA.Response(), accB.Response())
	}
	sameSequence(t, accA.Events(), accB.Events())
	if !a.Snapshot().Equal(b.Snapshot()) {
		t.Fatalf("final snapshots differ:\n%+v\n%+v", a.Snapshot(), b.Snapshot())
	}
}

func mustBeComparable[T comparable]() {}

// Responses and events are comparable value types (no slice, map or func); the
// absence of pointers is a review item, as domain documents (DEC-12).
var (
	_ = mustBeComparable[domain.ItemAccepted]
	_ = mustBeComparable[domain.PlacedResponse]
	_ = mustBeComparable[domain.ItemAdded]
	_ = mustBeComparable[domain.OrderPlaced]
	_ = mustBeComparable[domain.AddItem]
	_ = mustBeComparable[domain.PlaceOrder]
)
