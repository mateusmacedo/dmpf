package reservations_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]domain.Code{
		"CodeNothingToReserve":    reservations.CodeNothingToReserve,
		"CodeAlreadyReserved":     reservations.CodeAlreadyReserved,
		"CodeAlreadyCanceled":     reservations.CodeAlreadyCanceled,
		"CodeReservationCanceled": reservations.CodeReservationCanceled,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
