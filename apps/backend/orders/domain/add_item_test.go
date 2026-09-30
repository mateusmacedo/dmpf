package domain_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestAddItemAccepts(t *testing.T) {
	o := newOpenOrder(t, 2, 3)

	acc, rej := o.AddItem(domain.AddItem{SKU: "ABC", Quantity: 1, At: at})

	requireAccepted[domain.ItemAccepted](t, rej)
	if got, want := acc.Response(), (domain.ItemAccepted{Order: orderID, Items: 3}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := domain.ItemAdded{Order: orderID, SKU: "ABC", Quantity: 1, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := o.Snapshot(); len(got.Items) != 3 || got.Items[2] != (domain.Item{SKU: "ABC", Quantity: 1}) {
		t.Fatalf("Snapshot().Items = %+v", got.Items)
	}
}

func TestAddItemRejects(t *testing.T) {
	tests := []struct {
		name    string
		order   func(t *testing.T) *domain.Order
		code    kernel.Code
		details []kernel.Detail
	}{
		{
			name:    "item limit exceeded",
			order:   func(t *testing.T) *domain.Order { return newOpenOrder(t, 3, 3) },
			code:    domain.CodeOrderItemLimitExceeded,
			details: []kernel.Detail{{Key: "limit", Value: "3"}, {Key: "attempted", Value: "4"}},
		},
		{
			name: "order not open",
			order: func(t *testing.T) *domain.Order {
				o := newOpenOrder(t, 1, 3)
				if _, rej := o.Place(domain.PlaceOrder{At: at}); rej != nil {
					t.Fatalf("setup: Place rejected: %v", rej)
				}
				return o
			},
			code:    domain.CodeOrderNotOpen,
			details: []kernel.Detail{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.order(t)
			before := o.Snapshot()

			acc, rej := o.AddItem(domain.AddItem{SKU: "XYZ", Quantity: 1, At: at})

			requireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireUnchanged(t, before, o.Snapshot())
		})
	}
}

func TestSnapshotItemsAreNotAliased(t *testing.T) {
	o := newOpenOrder(t, 2, 3)
	first := o.Snapshot()

	first.Items[0] = domain.Item{SKU: "MUTATED", Quantity: 999}

	second := o.Snapshot()
	if second.Items[0].SKU == "MUTATED" {
		t.Fatal("mutating Snapshot().Items reached the aggregate")
	}
	if second.Items[0] != (domain.Item{SKU: "A", Quantity: 1}) {
		t.Fatalf("Snapshot().Items[0] = %+v", second.Items[0])
	}
}

func TestSnapshotEqualDetectsEachField(t *testing.T) {
	base := domain.Snapshot{ID: orderID, Status: domain.Open, ItemLimit: 3,
		Items: []domain.Item{{SKU: "A", Quantity: 1}}}
	mutations := map[string]func(s domain.Snapshot) domain.Snapshot{
		"id":         func(s domain.Snapshot) domain.Snapshot { s.ID = "P-200"; return s },
		"status":     func(s domain.Snapshot) domain.Snapshot { s.Status = domain.Placed; return s },
		"item limit": func(s domain.Snapshot) domain.Snapshot { s.ItemLimit = 4; return s },
		"item value": func(s domain.Snapshot) domain.Snapshot {
			s.Items = []domain.Item{{SKU: "A", Quantity: 2}}
			return s
		},
		"item count": func(s domain.Snapshot) domain.Snapshot {
			s.Items = []domain.Item{{SKU: "A", Quantity: 1}, {SKU: "B", Quantity: 1}}
			return s
		},
	}
	if !base.Equal(base) {
		t.Fatal("a snapshot must equal itself")
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			if base.Equal(mutate(base)) {
				t.Fatalf("Equal did not detect a change in %s", name)
			}
		})
	}
}
