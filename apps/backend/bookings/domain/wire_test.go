package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// The rejection codes are published: the Go names follow Code<Agg><Reason>, but
// the values on the wire stay the ones clients already read (D2).
func TestRejectionCodesKeepTheirPublishedValues(t *testing.T) {
	codes := map[kernel.Code]string{
		domain.CodeBookingQuantityOutOfRange: "resource-scheduling/booking/quantity-out-of-range",
		domain.CodeBookingNotReserved:        "resource-scheduling/booking/not-reserved",
		domain.CodeResourceCodeEmpty:         "resource-scheduling/resource/code-empty",
	}
	for code, want := range codes {
		if string(code) != want {
			t.Errorf("code = %q, want the published value %q", code, want)
		}
	}
}
