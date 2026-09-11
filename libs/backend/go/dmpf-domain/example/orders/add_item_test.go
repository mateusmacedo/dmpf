package orders_test

import (
	"slices"
	"testing"

	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/orders"
)

func TestAddItemAccepts(t *testing.T) {
	o := newOpenOrder(t, 2, 3)

	acc, rej := o.AddItem(orders.AddItem{SKU: "ABC", Quantity: 1, At: at})

	requireAccepted[orders.ItemAccepted](t, rej)
	if got, want := acc.Response(), (orders.ItemAccepted{Order: orderID, Items: 3}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := orders.ItemAdded{Order: orderID, SKU: "ABC", Quantity: 1, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := o.Snapshot(); len(got.Items) != 3 || got.Items[2] != (orders.Item{SKU: "ABC", Quantity: 1}) {
		t.Fatalf("Snapshot().Items = %+v", got.Items)
	}
}

func TestAddItemRejects(t *testing.T) {
	tests := []struct {
		name    string
		order   func(t *testing.T) *orders.Order
		code    dmpfdomain.Code
		details []dmpfdomain.Detail
	}{
		{
			name:    "item limit exceeded",
			order:   func(t *testing.T) *orders.Order { return newOpenOrder(t, 3, 3) },
			code:    orders.CodeItemLimitExceeded,
			details: []dmpfdomain.Detail{{Key: "limit", Value: "3"}, {Key: "attempted", Value: "4"}},
		},
		{
			name: "order not open",
			order: func(t *testing.T) *orders.Order {
				o := newOpenOrder(t, 1, 3)
				if _, rej := o.Place(orders.PlaceOrder{At: at}); rej != nil {
					t.Fatalf("setup: Place rejected: %v", rej)
				}
				return o
			},
			code:    orders.CodeOrderNotOpen,
			details: []dmpfdomain.Detail{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.order(t)
			before := o.Snapshot()

			acc, rej := o.AddItem(orders.AddItem{SKU: "XYZ", Quantity: 1, At: at})

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

	first.Items[0] = orders.Item{SKU: "MUTATED", Quantity: 999}

	second := o.Snapshot()
	if second.Items[0].SKU == "MUTATED" {
		t.Fatal("mutating Snapshot().Items reached the aggregate")
	}
	if second.Items[0] != (orders.Item{SKU: "A", Quantity: 1}) {
		t.Fatalf("Snapshot().Items[0] = %+v", second.Items[0])
	}
}

func TestSnapshotEqualDetectsEachField(t *testing.T) {
	base := orders.Snapshot{ID: orderID, Status: orders.Open, ItemLimit: 3,
		Items: []orders.Item{{SKU: "A", Quantity: 1}}}
	mutations := map[string]func(s orders.Snapshot) orders.Snapshot{
		"id":         func(s orders.Snapshot) orders.Snapshot { s.ID = "P-200"; return s },
		"status":     func(s orders.Snapshot) orders.Snapshot { s.Status = orders.Placed; return s },
		"item limit": func(s orders.Snapshot) orders.Snapshot { s.ItemLimit = 4; return s },
		"item value": func(s orders.Snapshot) orders.Snapshot {
			s.Items = []orders.Item{{SKU: "A", Quantity: 2}}
			return s
		},
		"item count": func(s orders.Snapshot) orders.Snapshot {
			s.Items = []orders.Item{{SKU: "A", Quantity: 1}, {SKU: "B", Quantity: 1}}
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
