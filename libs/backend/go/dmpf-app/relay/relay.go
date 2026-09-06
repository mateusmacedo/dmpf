package relay

import (
	"context"
	"errors"
	"math/bits"
	"sync"
	"time"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

// Relay is the drain loop of FND-04 §5.4. Every operational value below is the
// caller's declaration and FND-08's to catalogue; this block fixes none of them.
type Relay struct {
	Store     Store
	Publisher Publisher
	ClaimIDs  ClaimIDs
	Clock     dmpfports.Clock

	// Source is the producer URI the envelope carries, derived from the
	// bounded context and never from the host, the pod or the environment.
	Source string

	// Interval is how long an empty scan waits, BatchSize how many records one
	// claim acquires, Lease how long that claim stays valid.
	Interval  time.Duration
	BatchSize int
	Lease     time.Duration

	// Concurrency caps publishers in flight; MaxAttempts is the ceiling of
	// OBX-06, and Backoff how far a transient failure pushes available_at.
	Concurrency int
	MaxAttempts int
	Backoff     func(attempt int) time.Duration

	// ShutdownGrace bounds every write that runs detached from the caller's
	// cancellation: the transitions of step 3 and the release of unfinished
	// claims. Zero falls back to a deadline of the block's own.
	ShutdownGrace time.Duration
}

// Signals reports the four readings of OBX-12. Exposing them is this block's
// duty; naming the metrics and binding them to a backend is not.
func (r Relay) Signals(ctx context.Context) (dmpfpostgres.OutboxHealth, error) {
	if r.Store == nil {
		return dmpfpostgres.OutboxHealth{}, ErrIncompleteRelay
	}
	return r.Store.OutboxSignals(ctx)
}

// Run drains until ctx is done, one scan at a time: claim, publish the batch
// under the concurrency limit, then scan again. A cancelled context ends the
// loop without an error — stopping is not a failure.
func (r Relay) Run(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}

	for ctx.Err() == nil {
		claimed, err := r.Store.Claim(ctx, r.ClaimIDs.NewClaimID(), r.BatchSize, r.Lease)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return err
		}

		if len(claimed) == 0 {
			if !r.wait(ctx) {
				break
			}
			continue
		}

		var (
			group   sync.WaitGroup
			slots   = make(chan struct{}, r.Concurrency)
			holding = newHeld()
		)
		for _, record := range claimed {
			slots <- struct{}{}
			holding.take(record)
			group.Add(1)
			go func() {
				defer func() { group.Done(); <-slots }()
				if _, err := r.deliver(ctx, record); err != nil {
					// The transition never landed, so the claim is still live
					// and the release at the end of this scan hands it back.
					return
				}
				holding.settled(record.ID)
			}()
		}
		group.Wait()

		// Per scan, not per loop: a claim nobody finished goes back to the pool
		// now rather than waiting out its lease, and the set never carries
		// records from earlier scans into a process that runs for weeks.
		r.release(ctx, holding.remaining())
	}
	return nil
}

// wait sleeps out the scan interval and reports whether the loop should carry
// on; a cancelled context ends it right there instead of after the interval.
func (r Relay) wait(ctx context.Context) bool {
	timer := time.NewTimer(r.Interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r Relay) validate() error {
	switch {
	case r.Store == nil, r.Publisher == nil, r.ClaimIDs == nil, r.Clock == nil:
		return ErrIncompleteRelay
	case r.Source == "":
		return ErrIncompleteRelay
	case r.Interval <= 0, r.BatchSize <= 0, r.Lease <= 0, r.Concurrency <= 0:
		return ErrIncompleteRelay
	}
	return nil
}

// ExponentialBackoff doubles the window per attempt up to ceiling, then hands
// back half of it plus a jittered share of the other half. The fixed half keeps
// every retry making progress; the jittered half keeps relays that failed
// together from coming back together. jitter returns a fraction in [0, 1].
func ExponentialBackoff(base, ceiling time.Duration, jitter func() float64) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		window := windowFor(base, ceiling, attempt)
		half := window / 2
		if jitter == nil {
			return half
		}
		return half + time.Duration(jitter()*float64(window-half))
	}
}

// windowFor caps the shift before it happens: a large attempt count would
// overflow the duration and wrap into a negative window.
func windowFor(base, ceiling time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	if attempt < 1 {
		attempt = 1
	}

	shift := attempt - 1
	if shift >= bits.UintSize || base > ceiling>>shift {
		return ceiling
	}
	return base << shift
}

// ErrIncompleteRelay is what Run reports when a collaborator or an operational
// value is missing. There are no defaults to fall back on: every value here is
// the caller's declaration.
var ErrIncompleteRelay = errors.New("relay: requires a store, a publisher, claim ids, a clock, a source, and positive interval, batch size, lease and concurrency")
