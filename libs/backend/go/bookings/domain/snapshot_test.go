package bookingsdomain_test

import (
	"testing"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func TestFromBookingSnapshotRoundTrips(t *testing.T) {
	before := newReservedBooking(t).Snapshot()

	got := bookingsdomain.FromBookingSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromBookingSnapshotRestoresStatusAsBehaviour(t *testing.T) {
	b := newReservedBooking(t)
	reserved := b.Snapshot()

	reconstituted := bookingsdomain.FromBookingSnapshot(reserved)

	if got := reconstituted.Snapshot().Status; got != bookingsdomain.BookingReservedStatus {
		t.Fatalf("Status = %v, want BookingReservedStatus", got)
	}
	acc, rej := reconstituted.Cancel(bookingsdomain.CancelBooking{At: at})
	requireAccepted[bookingsdomain.CancelledResponse](t, rej)
	if len(acc.Events()) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(acc.Events()))
	}
}

func TestFromResourceSnapshotRoundTrips(t *testing.T) {
	before := newRegisteredResource(t).Snapshot()

	got := bookingsdomain.FromResourceSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}
