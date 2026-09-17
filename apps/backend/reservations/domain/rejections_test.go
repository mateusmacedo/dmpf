package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]kernel.Code{
		"CodeNothingToReserve":    domain.CodeNothingToReserve,
		"CodeAlreadyReserved":     domain.CodeAlreadyReserved,
		"CodeAlreadyCanceled":     domain.CodeAlreadyCanceled,
		"CodeReservationCanceled": domain.CodeReservationCanceled,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
