package reservations_test

import (
	"testing"

	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]dmpfdomain.Code{
		"CodeNothingToReserve": reservations.CodeNothingToReserve,
		"CodeAlreadyReserved":  reservations.CodeAlreadyReserved,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
