package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestPlaceAccepts(t *testing.T) {
	o := newOpenOrder(t, 1, 3)

	acc, rej := o.Place(domain.PlaceOrder{At: at})

	requireAccepted[domain.PlacedResponse](t, rej)
	if got, want := acc.Response(), (domain.PlacedResponse{Order: orderID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := domain.OrderPlaced{Order: orderID, Items: 1, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := o.Snapshot().Status; got != domain.Placed {
		t.Fatalf("Snapshot().Status = %v, want Placed", got)
	}
}

func TestPlaceRejects(t *testing.T) {
	tests := []struct {
		name  string
		order func(t *testing.T) *domain.Order
		code  kernel.Code
	}{
		{
			name:  "empty order",
			order: func(t *testing.T) *domain.Order { return newOpenOrder(t, 0, 3) },
			code:  domain.CodeEmptyOrder,
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
			code: domain.CodeOrderNotOpen,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.order(t)
			before := o.Snapshot()

			acc, rej := o.Place(domain.PlaceOrder{At: at})

			requireRejected(t, acc, rej, tt.code)
			requireUnchanged(t, before, o.Snapshot())
		})
	}
}
