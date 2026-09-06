package resilience

import (
	"fmt"
	"time"
)

// ValidateRoute refuses a route whose hops cannot fit in the time that is left
// (RES-07). Each hop reserves its declared deadline plus HopSlack, the margin
// for the hop's own overhead, because a route that fits only if nothing costs
// anything does not fit.
//
// It is checked before the call, and not inside Compose: a route is a property
// of the path the request will take, which the composition of one dependency
// does not know.
func ValidateRoute(remaining time.Duration, hops []Operation) error {
	if len(hops) == 0 {
		return nil
	}

	var needed time.Duration
	for _, hop := range hops {
		if err := hop.Validate(); err != nil {
			return err
		}
		needed += hop.Deadline + DefaultHopSlack
	}

	if needed > remaining {
		return fmt.Errorf("%w: %d hops need %v and %v remain", ErrDeadlineComposition, len(hops), needed, remaining)
	}
	return nil
}
