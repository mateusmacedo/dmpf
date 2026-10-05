//go:build integration

package provider_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const e2eOccurred = ports.Instant(1_755_432_000_000_000_000)

// WHY: provider → application é célula proibida, mas _test.go fica fora do
// universo do verificador e a composition root é justamente o papel que um
// teste encena. Nenhum arquivo de produção deste módulo alcança o bloco.

// The order of the two commands is fixed by the aggregate, not by preference:
// PlaceOrder only loads, and Place refuses an order with no items, so AddItem
// is what creates the order and OrderPlaced can only come second.
func TestTheUseCaseRunsEndToEndOverPostgres(t *testing.T) {
	h := appkit.NewOrders(t, clock.New(e2eOccurred), &ids.Sequence{Prefix: "m-"})
	pool, service := h.Pool, h.Service
	ctx := context.Background()

	added, err := service.AddItem(withExecution(t, ctx), application.AddItem{Order: repoOrderID, SKU: "sku-1", Quantity: 2})
	if err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}
	if rejection, refused := added.Rejection(); refused {
		t.Fatalf("AddItem() was rejected with %v, want accepted", rejection)
	}

	t.Run("the item added row lands at version 1", func(t *testing.T) {
		if _, version := load(t, pool); version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
		want := pg.Enqueued{
			MessageID:        "m-000001",
			MessageType:      "com.company.orders.item-added.v1",
			SchemaVersion:    "type.googleapis.com/company.orders.event.v1.ItemAdded",
			AggregateVersion: 1,
			Destination:      "orders.events",
			Status:           "pending",
		}
		if outbox := h.Outbox(t); len(outbox) != 1 || outbox[0] != want {
			t.Fatalf("outbox = %+v, want [%+v]", outbox, want)
		}
	})

	placed, err := service.PlaceOrder(withExecution(t, ctx), application.PlaceOrder{Order: repoOrderID})
	if err != nil {
		t.Fatalf("PlaceOrder() = %v, want nil", err)
	}
	if rejection, refused := placed.Rejection(); refused {
		t.Fatalf("PlaceOrder() was rejected with %v, want accepted", rejection)
	}

	t.Run("the order placed row lands at version 2", func(t *testing.T) {
		if _, version := load(t, pool); version != 2 {
			t.Fatalf("version = %d, want 2", version)
		}
		want := pg.Enqueued{
			MessageID:        "m-000002",
			MessageType:      "com.company.orders.order-placed.v1",
			SchemaVersion:    "type.googleapis.com/company.orders.event.v1.OrderPlaced",
			AggregateVersion: 2,
			Destination:      "orders.events",
			Status:           "pending",
		}
		if outbox := h.Outbox(t); len(outbox) != 2 || outbox[1] != want {
			t.Fatalf("outbox = %+v, want %+v second", outbox, want)
		}
	})

	t.Run("a rejection commits nothing and is not an error", func(t *testing.T) {
		before := pg.Counts(t, pool, "orders", "outbox")

		outcome, err := service.AddItem(withExecution(t, ctx), application.AddItem{Order: repoOrderID, SKU: "sku-2", Quantity: 1})

		if err != nil {
			t.Fatalf("AddItem() on a placed order = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
		}
		rejection, refused := outcome.Rejection()
		if !refused {
			t.Fatal("AddItem() on a placed order was accepted, want rejected")
		}
		if rejection.Code() != domain.CodeOrderNotOpen {
			t.Errorf("rejection code = %q, want %q", rejection.Code(), domain.CodeOrderNotOpen)
		}

		if after := pg.Counts(t, pool, "orders", "outbox"); after["orders"] != before["orders"] || after["outbox"] != before["outbox"] {
			t.Fatalf("counts moved on a rejection: %v→%v (UOW-06)", before, after)
		}
	})
}
