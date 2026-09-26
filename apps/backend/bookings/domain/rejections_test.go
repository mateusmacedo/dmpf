package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]kernel.Code{
		"CodeBookingQuantityOutOfRange": domain.CodeBookingQuantityOutOfRange,
		"CodeBookingNotReserved":        domain.CodeBookingNotReserved,
		"CodeResourceCodeEmpty":         domain.CodeResourceCodeEmpty,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
