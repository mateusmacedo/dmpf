package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

func TestFromBookingSnapshotRoundTrips(t *testing.T) {
	before := newReservedBooking(t).Snapshot()

	got := domain.FromBookingSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromBookingSnapshotRestoresStatusAsBehaviour(t *testing.T) {
	b := newReservedBooking(t)
	reserved := b.Snapshot()

	reconstituted := domain.FromBookingSnapshot(reserved)

	if got := reconstituted.Snapshot().Status; got != domain.Reserved {
		t.Fatalf("Status = %v, want Reserved", got)
	}
	acc, rej := reconstituted.Cancel(domain.CancelBooking{At: at})
	requireAccepted[domain.CancelledResponse](t, rej)
	if len(acc.Events()) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(acc.Events()))
	}
}

func TestFromResourceSnapshotRoundTrips(t *testing.T) {
	before := newRegisteredResource(t).Snapshot()

	got := domain.FromResourceSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}
