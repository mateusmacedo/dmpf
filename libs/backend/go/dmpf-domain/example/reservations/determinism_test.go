package reservations_test

import (
	"testing"

	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
)

func sameSequence(t *testing.T, a, b []dmpfdomain.DomainEvent) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("event sequences differ in length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("event %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestReserveIsDeterministic(t *testing.T) {
	a := reservations.NewReservation(orderID)
	b := reservations.NewReservation(orderID)
	cmd := reservations.Reserve{Items: 3, At: at}

	accA, rejA := a.Reserve(cmd)
	accB, rejB := b.Reserve(cmd)

	if rejA != nil || rejB != nil {
		t.Fatalf("expected both Accepted: %v / %v", rejA, rejB)
	}
	if accA.Response() != accB.Response() {
		t.Fatalf("responses differ: %+v vs %+v", accA.Response(), accB.Response())
	}
	sameSequence(t, accA.Events(), accB.Events())
	if !a.Snapshot().Equal(b.Snapshot()) {
		t.Fatalf("final snapshots differ:\n%+v\n%+v", a.Snapshot(), b.Snapshot())
	}
}

func mustBeComparable[T comparable]() {}

// Responses and events are comparable value types (no slice, map or func); the
// absence of pointers is a review item, as dmpfdomain documents (DEC-12).
var (
	_ = mustBeComparable[reservations.ReservedResponse]
	_ = mustBeComparable[reservations.ReservationConfirmed]
	_ = mustBeComparable[reservations.Reserve]
)
