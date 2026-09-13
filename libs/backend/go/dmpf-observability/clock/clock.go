package clock

import (
	"context"
	"time"
)

// Timer is a single-shot timer, as an interface because *time.Timer is a struct
// and a deterministic test cannot substitute it.
type Timer interface {
	// C is the channel the expiry is delivered on.
	C() <-chan time.Time
	// Stop prevents a pending expiry, reporting whether it stopped one.
	Stop() bool
	// Reset restarts the timer, reporting whether it stopped a pending expiry.
	Reset(d time.Duration) bool
}

// Clock is the provider's time. It exists next to dmpfports.Clock, and does not
// replace it: the port only reads the instant, which neither unblocks a select
// nor cancels a context, and the port may not import time at all.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
	NewTimer(d time.Duration) Timer
	WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc)
}

// Sleeper isolates the wait, so the backoff of a retry decorator is injected
// rather than reaching for time.Sleep, which no deterministic test can advance.
type Sleeper func(ctx context.Context, d time.Duration) error

// NewSleeper waits on the given clock, and gives up as soon as the context is
// done: a wait that outlives the deadline it was meant to respect is a leak.
func NewSleeper(c Clock) Sleeper {
	return func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if d <= 0 {
			return nil
		}

		timer := c.NewTimer(d)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C():
			return nil
		}
	}
}

// System is the real clock.
func System() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (systemClock) NewTimer(d time.Duration) Timer { return systemTimer{inner: time.NewTimer(d)} }

func (systemClock) WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

type systemTimer struct{ inner *time.Timer }

func (t systemTimer) C() <-chan time.Time { return t.inner.C }

func (t systemTimer) Stop() bool { return t.inner.Stop() }

func (t systemTimer) Reset(d time.Duration) bool { return t.inner.Reset(d) }
