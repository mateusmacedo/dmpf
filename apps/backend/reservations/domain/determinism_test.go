package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func TestReserveIsDeterministic(t *testing.T) {
	a := domain.NewReservation(orderID)
	b := domain.NewReservation(orderID)
	cmd := domain.Reserve{Items: 3, At: at}

	accA, rejA := a.Reserve(cmd)
	accB, rejB := b.Reserve(cmd)

	if rejA != nil || rejB != nil {
		t.Fatalf("expected both Accepted: %v / %v", rejA, rejB)
	}
	if accA.Response() != accB.Response() {
		t.Fatalf("responses differ: %+v vs %+v", accA.Response(), accB.Response())
	}
	tb.RequireSameEvents(t, accA.Events(), accB.Events())
	if !a.Snapshot().Equal(b.Snapshot()) {
		t.Fatalf("final snapshots differ:\n%+v\n%+v", a.Snapshot(), b.Snapshot())
	}
}

func TestCancelIsDeterministic(t *testing.T) {
	a := domain.NewReservation(orderID)
	b := domain.NewReservation(orderID)
	cmd := domain.Cancel{At: at}

	accA, rejA := a.Cancel(cmd)
	accB, rejB := b.Cancel(cmd)

	if rejA != nil || rejB != nil {
		t.Fatalf("expected both Accepted: %v / %v", rejA, rejB)
	}
	if accA.Response() != accB.Response() {
		t.Fatalf("responses differ: %+v vs %+v", accA.Response(), accB.Response())
	}
	tb.RequireSameEvents(t, accA.Events(), accB.Events())
	if !a.Snapshot().Equal(b.Snapshot()) {
		t.Fatalf("final snapshots differ:\n%+v\n%+v", a.Snapshot(), b.Snapshot())
	}
}

func mustBeComparable[T comparable]() {}

// Responses and events are comparable value types (no slice, map or func); the
// absence of pointers is a review item, as domain documents (DEC-12).
var (
	_ = mustBeComparable[domain.ReservedResponse]
	_ = mustBeComparable[domain.ReservationConfirmed]
	_ = mustBeComparable[domain.Reserve]
	_ = mustBeComparable[domain.CancelledResponse]
	_ = mustBeComparable[domain.ReservationCancelled]
	_ = mustBeComparable[domain.Cancel]
)
