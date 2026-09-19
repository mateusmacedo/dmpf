//go:build integration

package appkit_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
)

const orderUnderTest = domain.OrderID("o-appkit-1")

func harness(t *testing.T) appkit.Harness {
	t.Helper()
	return appkit.NewOrders(t, clock.New(ports.Instant(1)), &ids.Sequence{Prefix: "m-"})
}

func TestTheUseCaseLeavesOneOutboxRecordPerDecision(t *testing.T) {
	h := harness(t)
	ctx := context.Background()

	added, err := h.Service.AddItem(ctx, application.AddItem{Order: orderUnderTest, SKU: "sku-1", Quantity: 2})
	if err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}
	if rejection, refused := added.Rejection(); refused {
		t.Fatalf("AddItem() was rejected with %v, want accepted", rejection)
	}
	placed, err := h.Service.PlaceOrder(ctx, application.PlaceOrder{Order: orderUnderTest})
	if err != nil {
		t.Fatalf("PlaceOrder() = %v, want nil", err)
	}
	if rejection, refused := placed.Rejection(); refused {
		t.Fatalf("PlaceOrder() was rejected with %v, want accepted", rejection)
	}

	enqueued := h.Outbox(t)

	if len(enqueued) != 2 {
		t.Fatalf("outbox holds %d records, want 2: the event goes to the outbox in the same transaction as the state", len(enqueued))
	}
	wantTypes := []string{"com.company.orders.item-added.v1", "com.company.orders.order-placed.v1"}
	for i, want := range wantTypes {
		if enqueued[i].MessageType != want {
			t.Fatalf("record %d is %q, want %q in the order the decisions were taken", i, enqueued[i].MessageType, want)
		}
		if enqueued[i].Status != "pending" {
			t.Fatalf("record %d is %q, want pending: nothing drained it yet", i, enqueued[i].Status)
		}
		if enqueued[i].Destination != application.Destination {
			t.Fatalf("record %d goes to %q, want %q", i, enqueued[i].Destination, application.Destination)
		}
		if enqueued[i].AggregateVersion != int64(i+1) {
			t.Fatalf("record %d carries version %d, want %d", i, enqueued[i].AggregateVersion, i+1)
		}
	}
}

func TestARejectedDecisionLeavesNothingBehind(t *testing.T) {
	h := harness(t)
	ctx := context.Background()
	if _, err := h.Service.AddItem(ctx, application.AddItem{Order: orderUnderTest, SKU: "sku-1", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}
	if _, err := h.Service.PlaceOrder(ctx, application.PlaceOrder{Order: orderUnderTest}); err != nil {
		t.Fatalf("PlaceOrder() = %v, want nil", err)
	}
	before := len(h.Outbox(t))

	// Placing twice is the rejection the aggregate declares (CodeOrderNotOpen),
	// and a rejection never reaches the outbox.
	again, err := h.Service.PlaceOrder(ctx, application.PlaceOrder{Order: orderUnderTest})

	if err != nil {
		t.Fatalf("PlaceOrder() = %v, want the rejection on the business channel, not an error", err)
	}
	if _, refused := again.Rejection(); !refused {
		t.Fatal("PlaceOrder() accepted an order that was already placed")
	}
	if after := len(h.Outbox(t)); after != before {
		t.Fatalf("outbox went from %d to %d records after a rejection, want no change: the transaction never committed", before, after)
	}
}
