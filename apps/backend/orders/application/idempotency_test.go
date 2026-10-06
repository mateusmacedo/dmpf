package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestARepeatedAddItemReplaysWithoutASecondEffectOrAudit(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	add := application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1}

	first, err := h.service.AddItem(withKey(t, context.Background(), "k-add"), add)
	if err != nil {
		t.Fatalf("first AddItem() = %v, want nil", err)
	}
	ctx := withKey(t, context.Background(), "k-add")
	again, err := h.service.AddItem(ctx, add)
	if err != nil {
		t.Fatalf("repeated AddItem() = %v, want the stored outcome", err)
	}

	if again.Response() != first.Response() || again.Response().Items != 1 {
		t.Fatalf("replay = %+v, want %+v with one item", again.Response(), first.Response())
	}
	if h.saves != 1 || h.enqueues() != 1 {
		t.Fatalf("saves %d, enqueues %d, want 1 and 1: the replay reapplied the effect", h.saves, h.enqueues())
	}
	if len(instr.audits) != 1 {
		t.Fatalf("Audit called %d times, want 1: a replay is not a new fact", len(instr.audits))
	}
	if outcome, _ := ports.IdempotencyOutcomeFrom(ctx); outcome != ports.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
}

func TestARepeatedRefusalReplaysTheRejectionWithoutLoading(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(1), 0)
	if _, err := h.service.PlaceOrder(withExecution(t, context.Background()), application.PlaceOrder{Order: orderID}); err != nil {
		t.Fatalf("PlaceOrder() = %v, want nil", err)
	}

	add := application.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}
	if _, err := h.service.AddItem(withKey(t, context.Background(), "k-refused"), add); err != nil {
		t.Fatalf("first AddItem() = %v, want a rejection, not an error", err)
	}
	h.rec.Reset()
	again, err := h.service.AddItem(withKey(t, context.Background(), "k-refused"), add)
	if err != nil {
		t.Fatalf("repeated AddItem() = %v, want the stored rejection", err)
	}

	if _, rejected := again.Rejection(); !rejected {
		t.Fatalf("replay = %+v, want the rejection of an order already placed", again)
	}
	if slices.Contains(h.rec.Observed(), "orders.Load") {
		t.Fatalf("sequence = %v: the replay reached the aggregate", h.rec.Observed())
	}
}

func TestTheSameKeyWithAnotherPayloadIsAMismatch(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.AddItem(withKey(t, context.Background(), "k-reused"), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1}); err != nil {
		t.Fatalf("first AddItem() = %v, want nil", err)
	}

	_, err := h.service.AddItem(withKey(t, context.Background(), "k-reused"), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 2})

	if !errors.Is(err, ports.ErrIdempotencyMismatch) {
		t.Fatalf("AddItem(other quantity) = %v, want ErrIdempotencyMismatch", err)
	}
	if h.enqueues() != 1 {
		t.Fatalf("enqueues = %d, want 1", h.enqueues())
	}
}

func TestACommandWaitingPastTheCeilingIsInFlight(t *testing.T) {
	h := newHarness(t, withRegisterError(ports.ErrRegisterTimeout))

	_, err := h.service.PlaceOrder(withExecution(t, context.Background()), application.PlaceOrder{Order: orderID})

	if !errors.Is(err, ports.ErrIdempotencyInFlight) {
		t.Fatalf("PlaceOrder() = %v, want ErrIdempotencyInFlight", err)
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeAnyEffect(t *testing.T) {
	h := newHarness(t)
	ctx := ports.WithExecutionContext(context.Background(), testExecution(t))

	_, err := h.service.AddItem(ctx, application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1})

	if !errors.Is(err, ports.ErrIdempotencyKeyAbsent) {
		t.Fatalf("AddItem() = %v, want ErrIdempotencyKeyAbsent", err)
	}
	if h.saves != 0 || h.enqueues() != 0 {
		t.Fatalf("saves %d, enqueues %d, want none", h.saves, h.enqueues())
	}
}

func TestAKeyIsScopedToItsOperation(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(1), 0)
	if _, err := h.service.AddItem(withKey(t, context.Background(), "k-op"), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}

	_, err := h.service.PlaceOrder(withKey(t, context.Background(), "k-op"), application.PlaceOrder{Order: orderID})

	if !errors.Is(err, ports.ErrIdempotencyMismatch) {
		t.Fatalf("PlaceOrder(key of AddItem) = %v, want ErrIdempotencyMismatch", err)
	}
	if snapshot, _, _ := h.service.Reader.Load(withExecution(t, context.Background()), orderID); snapshot.Status == domain.Placed {
		t.Fatal("the order was placed under a key another operation had used")
	}
}
