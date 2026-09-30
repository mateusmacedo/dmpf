package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestReserveAccepts(t *testing.T) {
	r := domain.NewReservation(orderID)

	acc, rej := r.Reserve(domain.Reserve{Items: 3, At: at})

	requireAccepted[domain.ReservedResponse](t, rej)
	if got, want := acc.Response(), (domain.ReservedResponse{Order: orderID, Items: 3}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d elements, want 1", len(events))
	}
	want := domain.ReservationConfirmed{Order: orderID, Items: 3, At: at}
	if events[0] != want {
		t.Fatalf("Events()[0] = %+v, want %+v", events[0], want)
	}
	if got := r.Snapshot().Status; got != domain.Confirmed {
		t.Fatalf("Snapshot().Status = %v, want Confirmed", got)
	}
}

func TestReserveRejects(t *testing.T) {
	tests := []struct {
		name        string
		reservation func(t *testing.T) *domain.Reservation
		items       int
		code        kernel.Code
	}{
		{
			name:        "zero items",
			reservation: func(t *testing.T) *domain.Reservation { return domain.NewReservation(orderID) },
			items:       0,
			code:        domain.CodeReservationNothingToReserve,
		},
		{
			name:        "negative items",
			reservation: func(t *testing.T) *domain.Reservation { return domain.NewReservation(orderID) },
			items:       -1,
			code:        domain.CodeReservationNothingToReserve,
		},
		{
			name: "already reserved",
			reservation: func(t *testing.T) *domain.Reservation {
				r := domain.NewReservation(orderID)
				if _, rej := r.Reserve(domain.Reserve{Items: 2, At: at}); rej != nil {
					t.Fatalf("setup: Reserve rejected: %v", rej)
				}
				return r
			},
			items: 1,
			code:  domain.CodeReservationAlreadyReserved,
		},
		{
			name: "canceled",
			reservation: func(t *testing.T) *domain.Reservation {
				r := domain.NewReservation(orderID)
				if _, rej := r.Cancel(domain.Cancel{At: at}); rej != nil {
					t.Fatalf("setup: Cancel rejected: %v", rej)
				}
				return r
			},
			items: 1,
			code:  domain.CodeReservationCancelled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.reservation(t)
			before := r.Snapshot()

			acc, rej := r.Reserve(domain.Reserve{Items: tt.items, At: at})

			requireRejected(t, acc, rej, tt.code)
			requireUnchanged(t, before, r.Snapshot())
		})
	}
}
