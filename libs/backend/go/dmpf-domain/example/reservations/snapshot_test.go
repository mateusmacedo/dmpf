package reservations_test

import (
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
)

func TestFromSnapshotRoundTripsTheObservableState(t *testing.T) {
	r := reservations.NewReservation(orderID)
	if _, rej := r.Reserve(reservations.Reserve{Items: 2, At: at}); rej != nil {
		t.Fatalf("setup: Reserve rejected: %v", rej)
	}
	before := r.Snapshot()

	got := reservations.FromSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromSnapshotRestoresTheStatusAsBehaviour(t *testing.T) {
	r := reservations.NewReservation(orderID)
	if _, rej := r.Reserve(reservations.Reserve{Items: 2, At: at}); rej != nil {
		t.Fatalf("setup: Reserve rejected: %v", rej)
	}
	confirmed := r.Snapshot()

	reconstituted := reservations.FromSnapshot(confirmed)

	if got := reconstituted.Snapshot().Status; got != reservations.Confirmed {
		t.Fatalf("Status = %v, want Confirmed", got)
	}
	acc, rej := reconstituted.Reserve(reservations.Reserve{Items: 1, At: at})
	requireRejected(t, acc, rej, reservations.CodeAlreadyReserved)
}
