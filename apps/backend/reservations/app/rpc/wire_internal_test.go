package rpc

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
)

// The Go name reads Cancelled, the published enum keeps CANCELED (D2).
func TestACancelledReservationTravelsAsTheCanceledEnum(t *testing.T) {
	if got := reservationStatus(domain.Cancelled); got != servicev1.ReservationStatus_RESERVATION_STATUS_CANCELED {
		t.Fatalf("reservationStatus(Cancelled) = %v, want RESERVATION_STATUS_CANCELED", got)
	}
}
