package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]kernel.Code{
		"CodeReservationNothingToReserve": domain.CodeReservationNothingToReserve,
		"CodeReservationAlreadyReserved":  domain.CodeReservationAlreadyReserved,
		"CodeReservationAlreadyCancelled": domain.CodeReservationAlreadyCancelled,
		"CodeReservationCancelled":        domain.CodeReservationCancelled,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
