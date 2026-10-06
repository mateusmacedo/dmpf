package ports_test

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestTheIdempotencyFailuresAreDistinct(t *testing.T) {
	failures := []error{
		ports.ErrIdempotencyMismatch,
		ports.ErrIdempotencyInFlight,
		ports.ErrIdempotencyKeyAbsent,
		ports.ErrIdempotencyKeyInvalid,
		ports.ErrRegisterTimeout,
	}
	for i, a := range failures {
		for j, b := range failures {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v reads as %v", a, b)
			}
		}
	}
}
