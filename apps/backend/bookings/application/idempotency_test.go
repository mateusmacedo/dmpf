package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func reserve(quantity int) application.ReserveBooking {
	return application.ReserveBooking{BookingID: testBookingID, ResourceID: testResourceID, Quantity: quantity}
}

func (h *harness) outboxLen() int {
	return len(h.store.Entries())
}

func TestARepeatedReserveReplaysWithoutASecondEffect(t *testing.T) {
	h := newHarness(t)

	first, err := h.service.ReserveBooking(withKey(t, context.Background(), "k-reserve"), reserve(5))
	if err != nil {
		t.Fatalf("first ReserveBooking() = %v, want nil", err)
	}
	ctx := withKey(t, context.Background(), "k-reserve")
	again, err := h.service.ReserveBooking(ctx, reserve(5))
	if err != nil {
		t.Fatalf("repeated ReserveBooking() = %v, want the stored outcome", err)
	}

	if again.Response() != first.Response() {
		t.Fatalf("replay = %+v, want %+v", again.Response(), first.Response())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1: the replay produced a second event", got)
	}
	if outcome, _ := ports.IdempotencyOutcomeFrom(ctx); outcome != ports.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
}

func TestARepeatedRefusalReplaysTheRejectionWithoutLoading(t *testing.T) {
	h := newHarness(t)
	h.seedBooking(t, domain.BookingSnapshot{ID: testBookingID, ResourceID: testResourceID, Quantity: 5, Status: domain.Cancelled}, 2)
	cancel := application.CancelBooking{BookingID: testBookingID}

	if _, err := h.service.CancelBooking(withKey(t, context.Background(), "k-cancel"), cancel); err != nil {
		t.Fatalf("first CancelBooking() = %v, want a rejection, not an error", err)
	}
	h.rec.observed = nil
	again, err := h.service.CancelBooking(withKey(t, context.Background(), "k-cancel"), cancel)
	if err != nil {
		t.Fatalf("repeated CancelBooking() = %v, want the stored rejection", err)
	}

	rejection, rejected := again.Rejection()
	if !rejected || rejection.Code() != domain.CodeBookingNotReserved {
		t.Fatalf("replay = %+v, want the rejection %s", again, domain.CodeBookingNotReserved)
	}
	if slices.Contains(h.rec.observed, "bookings.Load") {
		t.Fatalf("sequence = %v: the replay reached the aggregate", h.rec.observed)
	}
}

func TestTheSameKeyWithAnotherPayloadIsAMismatch(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.ReserveBooking(withKey(t, context.Background(), "k-reused"), reserve(5)); err != nil {
		t.Fatalf("first ReserveBooking() = %v, want nil", err)
	}

	_, err := h.service.ReserveBooking(withKey(t, context.Background(), "k-reused"), reserve(6))

	if !errors.Is(err, ports.ErrIdempotencyMismatch) {
		t.Fatalf("ReserveBooking(other quantity) = %v, want ErrIdempotencyMismatch", err)
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1", got)
	}
}

func TestACommandWaitingPastTheCeilingIsInFlight(t *testing.T) {
	h := newHarness(t, withRegisterError(ports.ErrRegisterTimeout))

	_, err := h.service.RegisterResource(withExecution(t, context.Background()), application.RegisterResource{Code: testResCode})

	if !errors.Is(err, ports.ErrIdempotencyInFlight) {
		t.Fatalf("RegisterResource() = %v, want ErrIdempotencyInFlight", err)
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeAnyEffect(t *testing.T) {
	h := newHarness(t)
	ctx := ports.WithExecutionContext(context.Background(), testExecution(t))

	_, err := h.service.ReserveBooking(ctx, reserve(5))

	if !errors.Is(err, ports.ErrIdempotencyKeyAbsent) {
		t.Fatalf("ReserveBooking() = %v, want ErrIdempotencyKeyAbsent", err)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none", got)
	}
}

func TestReservingAnExistingBookingUnderAnotherKeyAlreadyExists(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.ReserveBooking(withKey(t, context.Background(), "k-first"), reserve(5)); err != nil {
		t.Fatalf("first ReserveBooking() = %v, want nil", err)
	}

	_, err := h.service.ReserveBooking(withKey(t, context.Background(), "k-second"), reserve(5))

	if !errors.Is(err, ports.ErrAlreadyExists) {
		t.Fatalf("ReserveBooking(another key) = %v, want ErrAlreadyExists", err)
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1", got)
	}
}

func TestARepeatedRegisterReplaysWithoutASecondEvent(t *testing.T) {
	h := newHarness(t)
	register := application.RegisterResource{Code: testResCode}

	for range 2 {
		if _, err := h.service.RegisterResource(withKey(t, context.Background(), "k-register"), register); err != nil {
			t.Fatalf("RegisterResource() = %v, want nil", err)
		}
	}

	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1", got)
	}
}
