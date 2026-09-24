package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

func TestFromSnapshotRoundTripsTheObservableState(t *testing.T) {
	r := domain.NewReservation(orderID)
	if _, rej := r.Reserve(domain.Reserve{Items: 2, At: at}); rej != nil {
		t.Fatalf("setup: Reserve rejected: %v", rej)
	}
	before := r.Snapshot()

	got := domain.FromSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromSnapshotRestoresTheStatusAsBehaviour(t *testing.T) {
	r := domain.NewReservation(orderID)
	if _, rej := r.Reserve(domain.Reserve{Items: 2, At: at}); rej != nil {
		t.Fatalf("setup: Reserve rejected: %v", rej)
	}
	confirmed := r.Snapshot()

	reconstituted := domain.FromSnapshot(confirmed)

	if got := reconstituted.Snapshot().Status; got != domain.Confirmed {
		t.Fatalf("Status = %v, want Confirmed", got)
	}
	acc, rej := reconstituted.Reserve(domain.Reserve{Items: 1, At: at})
	requireRejected(t, acc, rej, domain.CodeAlreadyReserved)
}

func TestFromSnapshotRestoresCanceledAsBehaviour(t *testing.T) {
	r := domain.NewReservation(orderID)
	if _, rej := r.Cancel(domain.Cancel{At: at}); rej != nil {
		t.Fatalf("setup: Cancel rejected: %v", rej)
	}
	canceled := r.Snapshot()

	reconstituted := domain.FromSnapshot(canceled)

	if got := reconstituted.Snapshot().Status; got != domain.Canceled {
		t.Fatalf("Status = %v, want Canceled", got)
	}
	acc, rej := reconstituted.Reserve(domain.Reserve{Items: 1, At: at})
	requireRejected(t, acc, rej, domain.CodeReservationCanceled)
}
