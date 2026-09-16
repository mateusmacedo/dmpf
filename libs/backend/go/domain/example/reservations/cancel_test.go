package reservations_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
)

func TestCancelAccepts(t *testing.T) {
	r := reservations.NewReservation(orderID)

	acc, rej := r.Cancel(reservations.Cancel{At: at})

	requireAccepted[reservations.CancelledResponse](t, rej)
	if got, want := acc.Response(), (reservations.CancelledResponse{Order: orderID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := reservations.ReservationCancelled{Order: orderID, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := r.Snapshot().Status; got != reservations.Canceled {
		t.Fatalf("Snapshot().Status = %v, want Canceled", got)
	}
}

func TestCancelRejects(t *testing.T) {
	tests := []struct {
		name        string
		reservation func(t *testing.T) *reservations.Reservation
		code        domain.Code
	}{
		{
			name: "already reserved",
			reservation: func(t *testing.T) *reservations.Reservation {
				r := reservations.NewReservation(orderID)
				if _, rej := r.Reserve(reservations.Reserve{Items: 2, At: at}); rej != nil {
					t.Fatalf("setup: Reserve rejected: %v", rej)
				}
				return r
			},
			code: reservations.CodeAlreadyReserved,
		},
		{
			name: "already canceled",
			reservation: func(t *testing.T) *reservations.Reservation {
				r := reservations.NewReservation(orderID)
				if _, rej := r.Cancel(reservations.Cancel{At: at}); rej != nil {
					t.Fatalf("setup: Cancel rejected: %v", rej)
				}
				return r
			},
			code: reservations.CodeAlreadyCanceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.reservation(t)
			before := r.Snapshot()

			acc, rej := r.Cancel(reservations.Cancel{At: at})

			requireRejected(t, acc, rej, tt.code)
			requireUnchanged(t, before, r.Snapshot())
		})
	}
}
