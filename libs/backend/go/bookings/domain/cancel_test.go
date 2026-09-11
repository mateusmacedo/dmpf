package bookingsdomain_test

import (
	"testing"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func TestCancelAccepts(t *testing.T) {
	b := newReservedBooking(t)

	acc, rej := b.Cancel(bookingsdomain.CancelBooking{At: at})

	requireAccepted[bookingsdomain.CancelledResponse](t, rej)
	if got, want := acc.Response(), (bookingsdomain.CancelledResponse{BookingID: bookingID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	ev := events[0].(bookingsdomain.BookingCancelledEvent)
	if ev.BookingID != bookingID || ev.At != at {
		t.Fatalf("BookingCancelledEvent = %+v", ev)
	}
	if got := b.Snapshot().Status; got != bookingsdomain.BookingCancelled {
		t.Fatalf("Status = %v, want BookingCancelled", got)
	}
}

func TestCancelRejectsWhenNotReserved(t *testing.T) {
	b := bookingsdomain.NewBooking(bookingID)
	before := b.Snapshot()

	acc, rej := b.Cancel(bookingsdomain.CancelBooking{At: at})

	requireRejected(t, acc, rej, bookingsdomain.CodeNotReserved)
	requireBookingUnchanged(t, before, b.Snapshot())
}

func TestCancelRejectsWhenAlreadyCancelled(t *testing.T) {
	b := newReservedBooking(t)
	if _, rej := b.Cancel(bookingsdomain.CancelBooking{At: at}); rej != nil {
		t.Fatalf("setup: Cancel rejected: %v", rej)
	}
	before := b.Snapshot()

	acc, rej := b.Cancel(bookingsdomain.CancelBooking{At: at})

	requireRejected(t, acc, rej, bookingsdomain.CodeNotReserved)
	requireBookingUnchanged(t, before, b.Snapshot())
}
