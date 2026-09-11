package bookingsdomain_test

import (
	"testing"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

func sameSequence(t *testing.T, a, b []dmpfdomain.DomainEvent) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("len(a)=%d, len(b)=%d", len(a), len(b))
	}
	for i := range a {
		if a[i].EventName() != b[i].EventName() {
			t.Fatalf("event[%d] name: %q vs %q", i, a[i].EventName(), b[i].EventName())
		}
		if a[i] != b[i] {
			t.Fatalf("event[%d] value: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestReserveDeterminism(t *testing.T) {
	cmd := bookingsdomain.ReserveBooking{ResourceID: resourceID, Quantity: 3, At: at}

	b1 := bookingsdomain.NewBooking(bookingID)
	acc1, _ := b1.Reserve(cmd)

	b2 := bookingsdomain.NewBooking(bookingID)
	acc2, _ := b2.Reserve(cmd)

	if !b1.Snapshot().Equal(b2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	sameSequence(t, acc1.Events(), acc2.Events())
}

func TestCancelDeterminism(t *testing.T) {
	cmd := bookingsdomain.CancelBooking{At: at}

	b1 := newReservedBooking(t)
	acc1, _ := b1.Cancel(cmd)

	b2 := newReservedBooking(t)
	acc2, _ := b2.Cancel(cmd)

	if !b1.Snapshot().Equal(b2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	sameSequence(t, acc1.Events(), acc2.Events())
}
