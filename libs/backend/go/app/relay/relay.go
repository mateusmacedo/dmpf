package relay

import (
	"context"
	"errors"
	"log/slog"
	"math/bits"
	"sync"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// Relay is the drain loop of FND-04 §5.4. Every operational value below is the
// caller's declaration and FND-08's to catalogue; this block fixes none of them.
type Relay struct {
	Store     Store
	Publisher Publisher
	ClaimIDs  ClaimIDs
	Clock     ports.Clock

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

	Tracer         trace.Tracer
	MeterProvider  metric.MeterProvider
	System         string
	Address        func(destination string) string
	LoggerProvider log.LoggerProvider

	logs   *slog.Logger
	meters *sendInstruments
}

// Signals reports the four readings of OBX-12. Exposing them is this block's
// duty; naming the metrics and binding them to a backend is not.
func (r Relay) Signals(ctx context.Context) (postgres.OutboxHealth, error) {
	if r.Store == nil {
		return postgres.OutboxHealth{}, ErrIncompleteRelay
	}
	return r.Store.OutboxSignals(ctx)
}

// Run drains until ctx is done, one scan at a time: claim, publish the batch
// under the concurrency limit, then scan again. A cancelled context ends the
// loop without an error — stopping is not a failure.
func (r Relay) Run(ctx context.Context) (err error) {
	if err := r.validate(); err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrPanicked
		}
	}()
	r.logs = r.logger()
	r.meters = r.instruments()

	for ctx.Err() == nil {
		claimedAt := time.Now()
		claimID := r.ClaimIDs.NewClaimID()
		claimed, err := r.Store.Claim(ctx, claimID, r.BatchSize, r.Lease)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			r.logClaimFailed(ctx, err)
			return err
		}

		if len(claimed) == 0 {
			if !r.wait(ctx) {
				break
			}
			continue
		}

		if err := r.scan(ctx, claimedAt, claimID, claimed); err != nil {
			return err
		}
	}
	return nil
}

func (r Relay) scan(ctx context.Context, claimedAt time.Time, claimID string, claimed []postgres.Claimed) error {
	holding := newHeld()
	for _, record := range claimed {
		holding.take(record)
	}
	drainCtx, drain := ctx, trace.Span(nil)
	// Per scan, not per loop: a claim nobody finished goes back to the pool
	// now rather than waiting out its lease, and the set never carries
	// records from earlier scans into a process that runs for weeks.
	defer func() {
		if drain != nil {
			defer drain.End()
		}
		r.release(drainCtx, holding.remaining())
	}()
	drainCtx, drain = r.openDrain(ctx, claimedAt, claimID, claimed)

	var (
		group   sync.WaitGroup
		slots   = make(chan struct{}, r.Concurrency)
		failure fault
	)
	for _, record := range claimed {
		slots <- struct{}{}
		group.Add(1)
		go func() {
			defer func() { group.Done(); <-slots }()
			defer failure.catch()
			if _, err := r.deliver(drainCtx, record); err != nil {
				// The transition never landed, so the claim is still live
				// and the release at the end of this scan hands it back.
				return
			}
			holding.settled(record.ID)
		}()
	}
	group.Wait()
	return failure.err
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
