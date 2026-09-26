package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// The rejection codes are published: the Go names follow Code<Agg><Reason>, but
// the values on the wire stay the ones clients already read (D2).
func TestRejectionCodesKeepTheirPublishedValues(t *testing.T) {
	codes := map[kernel.Code]string{
		domain.CodeReservationNothingToReserve: "reservations/nothing-to-reserve",
		domain.CodeReservationAlreadyReserved:  "reservations/already-reserved",
		domain.CodeReservationAlreadyCancelled: "reservations/already-canceled",
		domain.CodeReservationCancelled:        "reservations/reservation-canceled",
	}
	for code, want := range codes {
		if string(code) != want {
			t.Errorf("code = %q, want the published value %q", code, want)
		}
	}
}
