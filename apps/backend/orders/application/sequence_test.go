package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
)

var errDenied = errors.New("application_test: denied")

func TestAddItemWalksTheNineStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.AddItem(withExecution(t, context.Background()), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"orders.Load",
		"orders.Save",
		"outbox.Enqueue",
		"commit",
	}
	if !slices.Equal(h.rec.observed, want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.observed, want)
	}
}

func TestPlaceOrderWalksTheNineStepsInOrder(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(1), 0)

	if _, err := h.service.PlaceOrder(withExecution(t, context.Background()), application.PlaceOrder{Order: orderID}); err != nil {
		t.Fatalf("PlaceOrder() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"orders.Load",
		"orders.Save",
		"outbox.Enqueue",
		"commit",
	}
	if !slices.Equal(h.rec.observed, want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.observed, want)
	}
}

func TestADeniedAuthorizationStopsBeforeIdentityAndTransaction(t *testing.T) {
	deny := func(context.Context, application.Operation) error { return errDenied }
	h := newHarness(t, withAuthorize(deny))

	_, err := h.service.AddItem(withExecution(t, context.Background()), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1})

	if !errors.Is(err, errDenied) {
		t.Fatalf("AddItem() error = %v, want errDenied", err)
	}
	if want := []string{"authorize"}; !slices.Equal(h.rec.observed, want) {
		t.Fatalf("observed %v, want %v — nothing runs after a denied authorization", h.rec.observed, want)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}
