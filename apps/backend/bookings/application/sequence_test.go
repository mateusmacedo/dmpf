package application_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

func TestReserveWalksTheNineStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.ReserveBooking(withExecution(t, context.Background()), application.Reserve{
		BookingID: testBookingID, ResourceID: testResourceID, Quantity: 5,
	}); err != nil {
		t.Fatalf("ReserveBooking() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"bookings.Save",
		"outbox.Enqueue",
		"commit",
	}
	if !slices.Equal(h.rec.observed, want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.observed, want)
	}
}

func TestCancelWalksTheNineStepsInOrder(t *testing.T) {
	h := newHarness(t)
	h.seedBooking(t, domain.BookingSnapshot{
		ID: testBookingID, ResourceID: testResourceID, Quantity: 5,
		Status: domain.BookingReservedStatus, ReservedAt: 1000,
	}, 1)

	if _, err := h.service.CancelBooking(withExecution(t, context.Background()), application.Cancel{
		BookingID: testBookingID,
	}); err != nil {
		t.Fatalf("CancelBooking() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"bookings.Load",
		"bookings.Save",
		"commit",
	}
	if !slices.Equal(h.rec.observed, want) {
		t.Fatalf("sequence mismatch\ngot:  %v\nwant: %v", h.rec.observed, want)
	}
}

func TestRegisterWalksTheNineStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterResource(withExecution(t, context.Background()), application.Register{
		Code: testResCode,
	}); err != nil {
		t.Fatalf("RegisterResource() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"resources.Load",
		"resources.Save",
		"commit",
	}
	if !slices.Equal(h.rec.observed, want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.observed, want)
	}
}

func TestCancelRejectsWhenBookingNotReserved(t *testing.T) {
	h := newHarness(t)
	h.seedBooking(t, domain.BookingSnapshot{
		ID: testBookingID, Status: domain.BookingCancelled,
	}, 2)

	outcome, err := h.service.CancelBooking(withExecution(t, context.Background()), application.Cancel{
		BookingID: testBookingID,
	})
	if err != nil {
		t.Fatalf("CancelBooking() error = %v, want nil", err)
	}
	rej, refused := outcome.Rejection()
	if !refused {
		t.Fatal("expected Rejected, got Accepted")
	}
	if rej.Code() != domain.CodeNotReserved {
		t.Fatalf("Code() = %q, want %q", rej.Code(), domain.CodeNotReserved)
	}
}
