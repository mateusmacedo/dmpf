package ordersapp_test

import (
	"context"
	"errors"
	"testing"

	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

func TestPlaceOrderAcceptedPlacesTheOrderAndEnqueuesTheFact(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(1), 0)

	out, err := h.service.PlaceOrder(context.Background(), ordersapp.PlaceOrder{Order: orderID})

	if err != nil {
		t.Fatalf("PlaceOrder() error = %v, want nil", err)
	}
	if got, want := out.Response(), (orders.PlacedResponse{Order: orderID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}

	snapshot, version, _ := h.store.Reader().Load(context.Background(), orderID)
	if snapshot.Status != orders.Placed {
		t.Fatalf("Status = %v, want Placed", snapshot.Status)
	}
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d entries, want 1", len(entries))
	}
	if got := entries[0].Event.EventName(); got != "orders.order-placed" {
		t.Fatalf("EventName() = %q, want %q", got, "orders.order-placed")
	}
	if got := entries[0].AggregateVersion; got != 2 {
		t.Fatalf("AggregateVersion = %d, want 2", got)
	}
}

func TestPlaceOrderRejectedCommitsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(0), 0)

	out, err := h.service.PlaceOrder(context.Background(), ordersapp.PlaceOrder{Order: orderID})

	if err != nil {
		t.Fatalf("PlaceOrder() error = %v, want nil", err)
	}
	rej, refused := out.Rejection()
	if !refused {
		t.Fatal("Rejection() reported no refusal, want orders/empty-order")
	}
	if rej.Code() != orders.CodeEmptyOrder {
		t.Fatalf("Code() = %q, want %q", rej.Code(), orders.CodeEmptyOrder)
	}
	if got := h.serviceWithinCalls(); got != 1 {
		t.Fatalf("transactions opened = %d, want 1", got)
	}
	if got := h.serviceCommits(); got != 1 {
		t.Fatalf("commits = %d, want 1 — the commit occurs under a refusal", got)
	}
	if h.saves != 0 || h.enqueues != 0 {
		t.Fatalf("a refusal wrote: saves = %d, enqueues = %d, want 0 and 0", h.saves, h.enqueues)
	}
	snapshot, version, _ := h.store.Reader().Load(context.Background(), orderID)
	if snapshot.Status != orders.Open || version != 1 {
		t.Fatalf("the store changed under a refusal: status = %v, version = %d", snapshot.Status, version)
	}
}

func TestPlaceOrderKeepsNothingWhenTheCommitFailsWhileLoading(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(1), 0)
	h.store.FailNextCommit(errCommitFailed)

	out, err := h.service.PlaceOrder(context.Background(), ordersapp.PlaceOrder{Order: orderID})

	if !errors.Is(err, errCommitFailed) {
		t.Fatalf("PlaceOrder() error = %v, want errCommitFailed", err)
	}
	if got := out.Response(); got != (orders.PlacedResponse{}) {
		t.Fatalf("Response() = %+v, want the zero outcome", got)
	}
	snapshot, version, err := h.store.Reader().Load(context.Background(), orderID)
	if err != nil {
		t.Fatalf("Load() = %v, want the previous snapshot", err)
	}
	if snapshot.Status != orders.Open || version != 1 {
		t.Fatalf("a failed commit changed the store: status = %v, version = %d, want Open and 1", snapshot.Status, version)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
}

func TestPlaceOrderReportsErrNotFoundForAnAbsentAggregate(t *testing.T) {
	h := newHarness(t)

	out, err := h.service.PlaceOrder(context.Background(), ordersapp.PlaceOrder{Order: "P-200"})

	if !errors.Is(err, dmpfports.ErrNotFound) {
		t.Fatalf("PlaceOrder() error = %v, want ErrNotFound through the wrapping", err)
	}
	if got := out.Response(); got != (orders.PlacedResponse{}) {
		t.Fatalf("Response() = %+v, want the zero outcome", got)
	}
	if rej, refused := out.Rejection(); refused {
		t.Fatalf("Rejection() = %v, want none — an absent aggregate is not a domain refusal", rej)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
}
