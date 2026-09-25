package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

func TestCancelAccepts(t *testing.T) {
	r := domain.NewReservation(orderID)

	acc, rej := r.Cancel(domain.Cancel{At: at})

	requireAccepted[domain.CancelledResponse](t, rej)
	if got, want := acc.Response(), (domain.CancelledResponse{Order: orderID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := domain.ReservationCancelled{Order: orderID, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := r.Snapshot().Status; got != domain.Cancelled {
		t.Fatalf("Snapshot().Status = %v, want Cancelled", got)
	}
}

func TestCancelRejects(t *testing.T) {
	tests := []struct {
		name        string
		reservation func(t *testing.T) *domain.Reservation
		code        kernel.Code
	}{
		{
			name: "already reserved",
			reservation: func(t *testing.T) *domain.Reservation {
				r := domain.NewReservation(orderID)
				if _, rej := r.Reserve(domain.Reserve{Items: 2, At: at}); rej != nil {
					t.Fatalf("setup: Reserve rejected: %v", rej)
				}
				return r
			},
			code: domain.CodeReservationAlreadyReserved,
		},
		{
			name: "already canceled",
			reservation: func(t *testing.T) *domain.Reservation {
				r := domain.NewReservation(orderID)
				if _, rej := r.Cancel(domain.Cancel{At: at}); rej != nil {
					t.Fatalf("setup: Cancel rejected: %v", rej)
				}
				return r
			},
			code: domain.CodeReservationAlreadyCancelled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.reservation(t)
			before := r.Snapshot()

			acc, rej := r.Cancel(domain.Cancel{At: at})

			requireRejected(t, acc, rej, tt.code)
			requireUnchanged(t, before, r.Snapshot())
		})
	}
}
