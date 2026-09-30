package rpc

import (
	"testing"

	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/service/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

// The Go name reads Cancelled, the published enum keeps CANCELED (D2).
func TestACancelledReservationTravelsAsTheCanceledEnum(t *testing.T) {
	if got := reservationStatus(domain.Cancelled); got != servicev1.ReservationStatus_RESERVATION_STATUS_CANCELED {
		t.Fatalf("reservationStatus(Cancelled) = %v, want RESERVATION_STATUS_CANCELED", got)
	}
}
