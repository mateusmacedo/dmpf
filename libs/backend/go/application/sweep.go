package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// SweepPage reports one page of a cycle: Next resumes after the last attempt
// Seen, failed or not, so attempts that always fail never hold the page's head.
type SweepPage struct {
	Seen  int
	Swept int64
	Next  ports.AttemptCursor
}

// A failed close does not stop the page, because its attempt stays begun and
// the next cycle lists it again (RES-43).
func SweepStale(ctx context.Context, store ports.AttemptLedger, cutoff ports.Instant, after ports.AttemptCursor, batch int, closeAttempt func(context.Context, ports.Attempt) error) (SweepPage, error) {
	attempts, err := store.Stale(ctx, cutoff, after, batch)
	if err != nil {
		return SweepPage{}, fmt.Errorf("application: list stale attempts: %w", err)
	}
	page := SweepPage{Next: after}
	var failures []error
	for _, attempt := range attempts {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		page.Seen++
		page.Next = attempt.Cursor()
		if err := closeAttempt(ctx, attempt); err != nil {
			failures = append(failures, err)
			continue
		}
		page.Swept++
	}
	return page, errors.Join(failures...)
}
