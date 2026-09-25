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

	if _, err := h.service.ReserveBooking(withExecution(t, context.Background()), application.ReserveBooking{
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

	if _, err := h.service.CancelBooking(withExecution(t, context.Background()), application.CancelBooking{
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
		"outbox.Enqueue",
		"commit",
	}
	if !slices.Equal(h.rec.observed, want) {
		t.Fatalf("sequence mismatch\ngot:  %v\nwant: %v", h.rec.observed, want)
	}
}

func TestRegisterWalksTheNineStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterResource(withExecution(t, context.Background()), application.RegisterResource{
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
		"outbox.Enqueue",
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

	outcome, err := h.service.CancelBooking(withExecution(t, context.Background()), application.CancelBooking{
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

func TestCancelEnqueuesTheCancellationInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	h.seedBooking(t, domain.BookingSnapshot{
		ID: testBookingID, ResourceID: testResourceID, Quantity: 5,
		Status: domain.BookingReservedStatus, ReservedAt: 1000,
	}, 1)

	if _, err := h.service.CancelBooking(withExecution(t, context.Background()), application.CancelBooking{BookingID: testBookingID}); err != nil {
		t.Fatalf("CancelBooking() error = %v, want nil", err)
	}

	entries := h.store.outbox
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if _, ok := entry.Event.(domain.BookingCancelledEvent); !ok || entry.AggregateType != application.AggregateTypeBooking ||
		entry.AggregateID != string(testBookingID) || entry.AggregateVersion != 2 {
		t.Fatalf("outbox entry = %+v, want the cancellation of the booking at version 2", entry)
	}
}

func TestRegisterEnqueuesTheRegistrationInTheSameTransaction(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterResource(withExecution(t, context.Background()), application.RegisterResource{Code: testResCode}); err != nil {
		t.Fatalf("RegisterResource() error = %v, want nil", err)
	}

	entries := h.store.outbox
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if _, ok := entry.Event.(domain.ResourceRegistered); !ok || entry.AggregateType != application.AggregateTypeResource ||
		entry.AggregateID != string(testResCode) || entry.AggregateVersion != 1 {
		t.Fatalf("outbox entry = %+v, want the registration of the resource at version 1", entry)
	}
}
