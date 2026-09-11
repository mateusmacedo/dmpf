package bookingsdomain_test

import (
	"slices"
	"testing"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

func TestReserveAccepts(t *testing.T) {
	b := bookingsdomain.NewBooking(bookingID)

	acc, rej := b.Reserve(bookingsdomain.ReserveBooking{ResourceID: resourceID, Quantity: 5, At: at})

	requireAccepted[bookingsdomain.ReservedResponse](t, rej)
	if got, want := acc.Response(), (bookingsdomain.ReservedResponse{BookingID: bookingID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	ev := events[0].(bookingsdomain.BookingReserved)
	if ev.BookingID != bookingID || ev.ResourceID != resourceID || ev.Quantity != 5 || ev.At != at {
		t.Fatalf("BookingReserved = %+v", ev)
	}
	snap := b.Snapshot()
	if snap.Status != bookingsdomain.BookingReservedStatus || snap.Quantity != 5 || snap.ResourceID != resourceID {
		t.Fatalf("Snapshot = %+v", snap)
	}
}

func TestReserveRejectsQuantityOutOfRange(t *testing.T) {
	tests := []struct {
		name     string
		quantity int
		details  []dmpfdomain.Detail
	}{
		{name: "zero", quantity: 0, details: []dmpfdomain.Detail{{Key: "quantity", Value: "0"}}},
		{name: "negative", quantity: -1, details: []dmpfdomain.Detail{{Key: "quantity", Value: "-1"}}},
		{name: "above 100", quantity: 101, details: []dmpfdomain.Detail{{Key: "quantity", Value: "101"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bookingsdomain.NewBooking(bookingID)
			before := b.Snapshot()

			acc, rej := b.Reserve(bookingsdomain.ReserveBooking{ResourceID: resourceID, Quantity: tt.quantity, At: at})

			requireRejected(t, acc, rej, bookingsdomain.CodeQuantityOutOfRange)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireBookingUnchanged(t, before, b.Snapshot())
		})
	}
}
