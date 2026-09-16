package reservations_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
)

func TestReserveAccepts(t *testing.T) {
	r := reservations.NewReservation(orderID)

	acc, rej := r.Reserve(reservations.Reserve{Items: 3, At: at})

	requireAccepted[reservations.ReservedResponse](t, rej)
	if got, want := acc.Response(), (reservations.ReservedResponse{Order: orderID, Items: 3}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := reservations.ReservationConfirmed{Order: orderID, Items: 3, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := r.Snapshot().Status; got != reservations.Confirmed {
		t.Fatalf("Snapshot().Status = %v, want Confirmed", got)
	}
}

func TestReserveRejects(t *testing.T) {
	tests := []struct {
		name        string
		reservation func(t *testing.T) *reservations.Reservation
		items       int
		code        domain.Code
	}{
		{
			name:        "zero items",
			reservation: func(t *testing.T) *reservations.Reservation { return reservations.NewReservation(orderID) },
			items:       0,
			code:        reservations.CodeNothingToReserve,
		},
		{
			name:        "negative items",
			reservation: func(t *testing.T) *reservations.Reservation { return reservations.NewReservation(orderID) },
			items:       -1,
			code:        reservations.CodeNothingToReserve,
		},
		{
			name: "already reserved",
			reservation: func(t *testing.T) *reservations.Reservation {
				r := reservations.NewReservation(orderID)
				if _, rej := r.Reserve(reservations.Reserve{Items: 2, At: at}); rej != nil {
					t.Fatalf("setup: Reserve rejected: %v", rej)
				}
				return r
			},
			items: 1,
			code:  reservations.CodeAlreadyReserved,
		},
		{
			name: "canceled",
			reservation: func(t *testing.T) *reservations.Reservation {
				r := reservations.NewReservation(orderID)
				if _, rej := r.Cancel(reservations.Cancel{At: at}); rej != nil {
					t.Fatalf("setup: Cancel rejected: %v", rej)
				}
				return r
			},
			items: 1,
			code:  reservations.CodeReservationCanceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.reservation(t)
			before := r.Snapshot()

			acc, rej := r.Reserve(reservations.Reserve{Items: tt.items, At: at})

			requireRejected(t, acc, rej, tt.code)
			requireUnchanged(t, before, r.Snapshot())
		})
	}
}
