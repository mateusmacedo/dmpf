package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

func TestCancelAccepts(t *testing.T) {
	b := newReservedBooking(t)

	acc, rej := b.Cancel(domain.CancelBooking{At: at})

	requireAccepted[domain.CancelledResponse](t, rej)
	if got, want := acc.Response(), (domain.CancelledResponse{BookingID: bookingID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	ev := events[0].(domain.BookingCancelledEvent)
	if ev.BookingID != bookingID || ev.At != at {
		t.Fatalf("BookingCancelledEvent = %+v", ev)
	}
	if got := b.Snapshot().Status; got != domain.BookingCancelled {
		t.Fatalf("Status = %v, want BookingCancelled", got)
	}
}

func TestCancelRejectsWhenNotReserved(t *testing.T) {
	b := domain.NewBooking(bookingID)
	before := b.Snapshot()

	acc, rej := b.Cancel(domain.CancelBooking{At: at})

	requireRejected(t, acc, rej, domain.CodeNotReserved)
	requireBookingUnchanged(t, before, b.Snapshot())
}

func TestCancelRejectsWhenAlreadyCancelled(t *testing.T) {
	b := newReservedBooking(t)
	if _, rej := b.Cancel(domain.CancelBooking{At: at}); rej != nil {
		t.Fatalf("setup: Cancel rejected: %v", rej)
	}
	before := b.Snapshot()

	acc, rej := b.Cancel(domain.CancelBooking{At: at})

	requireRejected(t, acc, rej, domain.CodeNotReserved)
	requireBookingUnchanged(t, before, b.Snapshot())
}
