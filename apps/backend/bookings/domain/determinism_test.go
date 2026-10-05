package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func TestReserveDeterminism(t *testing.T) {
	cmd := domain.ReserveBooking{ResourceID: resourceID, Quantity: 3, At: at}

	b1 := domain.NewBooking(bookingID)
	acc1, _ := b1.Reserve(cmd)

	b2 := domain.NewBooking(bookingID)
	acc2, _ := b2.Reserve(cmd)

	if !b1.Snapshot().Equal(b2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestCancelDeterminism(t *testing.T) {
	cmd := domain.CancelBooking{At: at}

	b1 := newReservedBooking(t)
	acc1, _ := b1.Cancel(cmd)

	b2 := newReservedBooking(t)
	acc2, _ := b2.Cancel(cmd)

	if !b1.Snapshot().Equal(b2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}
