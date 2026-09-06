package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
)

// BreakerState is what dmpf_dependency_breaker_state records, and its numbering
// is the one MET-28 declares.
type BreakerState int64

const (
	// BreakerClosed lets every call through.
	BreakerClosed BreakerState = 0
	// BreakerHalfOpen lets the declared number of probes through.
	BreakerHalfOpen BreakerState = 1
	// BreakerOpen refuses every call until the cooldown elapses.
	BreakerOpen BreakerState = 2
)

func (s BreakerState) String() string {
	switch s {
	case BreakerHalfOpen:
		return "half_open"
	case BreakerOpen:
		return "open"
	default:
		return "closed"
	}
}

// Breaker is the circuit breaker of one dependency (RES-10, RES-11). It holds
// state across calls, so it is built once and its decorator reused: a breaker
// per call would never accumulate a window.
//
// Safe for concurrent use: the calls it guards run in parallel.
type Breaker struct {
	dependency string
	policy     BreakerPolicy
	clock      clock.Clock
	gauge      metric64Gauge

	mu         sync.Mutex
	window     []observation
	state      BreakerState
	openedAt   time.Time
	probing    int
	generation uint64
}

// admission is what a call carries from admit to record: whether it was let
// through as a probe, and which cycle of the state machine it belongs to. A
// breaker that decided by the state at record time would hand the outcome of a
// call started two cycles ago to whichever cycle happened to be running when it
// returned — closing over a dependency that never recovered.
type admission struct {
	probe      bool
	generation uint64
}

type observation struct {
	at     time.Time
	failed bool
}

// NewBreaker builds the breaker of a dependency. A nil set of instruments is
// accepted: a test exercises the state machine without a meter.
func NewBreaker(dependency string, policy BreakerPolicy, c clock.Clock, instruments *metrics.Instruments) *Breaker {
	breaker := &Breaker{dependency: dependency, policy: policy, clock: c, state: BreakerClosed}
	if instruments != nil {
		breaker.gauge = instruments.BreakerState
	}
	return breaker
}

// State is the current state, for a test and for whoever reports it.
func (b *Breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Decorate is the decorator of the canonical order. An open breaker returns
// ErrBreakerOpen without invoking the call, so it consumes neither the timeout
// nor an attempt (RES-12).
func (b *Breaker) Decorate() Decorator {
	return func(next Call) Call {
		return func(ctx context.Context, op Operation, do func(context.Context) error) error {
			granted, err := b.admit(ctx)
			if err != nil {
				return err
			}

			err = next(ctx, op, do)
			b.record(ctx, granted, err)
			return err
		}
	}
}

// admit decides whether the call goes through, moving the breaker to
// half-openness when the cooldown has elapsed. It returns the ticket the call
// hands back to record.
func (b *Breaker) admit(ctx context.Context) (admission, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()

	if b.state == BreakerOpen {
		if now.Sub(b.openedAt) < b.policy.Cooldown {
			return admission{}, fmt.Errorf("%w: %s is open", ErrBreakerOpen, b.dependency)
		}
		b.transition(ctx, BreakerHalfOpen, now)
	}

	if b.state == BreakerHalfOpen {
		if b.probing >= b.probes() {
			return admission{}, fmt.Errorf("%w: %s is half-open and its probes are in flight", ErrBreakerOpen, b.dependency)
		}
		b.probing++
		return admission{probe: true, generation: b.generation}, nil
	}

	return admission{generation: b.generation}, nil
}

// record accounts for the outcome. A cancellation by the caller is not counted:
// the dependency was never given the chance to answer, and holding it against
// the dependency would open the breaker on a client that gave up.
func (b *Breaker) record(ctx context.Context, granted admission, err error) {
	if errors.Is(err, ErrCancelled) {
		b.releaseProbe(granted)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()
	failed := err != nil

	if granted.probe {
		// A probe from a cycle that already ended decides nothing, and its slot
		// was released when the cycle turned over.
		if granted.generation != b.generation || b.state != BreakerHalfOpen {
			return
		}
		b.probing--
		b.window = nil
		if failed {
			b.transition(ctx, BreakerOpen, now)
			return
		}
		b.transition(ctx, BreakerClosed, now)
		return
	}

	// Half-openness belongs to the probe. A call admitted before the cycle
	// began carries an answer about a dependency that may no longer be the one
	// being tested, so it is neither counted nor allowed to close the breaker.
	if b.state == BreakerHalfOpen {
		return
	}

	b.window = append(b.prune(now), observation{at: now, failed: failed})
	if b.shouldOpen() {
		b.transition(ctx, BreakerOpen, now)
	}
}

func (b *Breaker) releaseProbe(granted admission) {
	if !granted.probe {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if granted.generation == b.generation && b.state == BreakerHalfOpen && b.probing > 0 {
		b.probing--
	}
}

// shouldOpen applies the sample floor before the threshold: a burst of three
// calls with one failure is 33 %, which would cross a 50 % threshold read on
// too little evidence (RES-10).
func (b *Breaker) shouldOpen() bool {
	if len(b.window) < b.policy.MinSamples {
		return false
	}

	failures := 0
	for _, seen := range b.window {
		if seen.failed {
			failures++
		}
	}
	return float64(failures)/float64(len(b.window)) >= b.policy.Threshold
}

func (b *Breaker) prune(now time.Time) []observation {
	if b.policy.Window <= 0 {
		return b.window
	}

	edge := now.Add(-b.policy.Window)
	kept := b.window[:0]
	for _, seen := range b.window {
		if seen.at.After(edge) {
			kept = append(kept, seen)
		}
	}
	return kept
}

func (b *Breaker) probes() int {
	if b.policy.Probes <= 0 {
		return 1
	}
	return b.policy.Probes
}

// transition records the new state on the gauge. It runs under the lock, so the
// value recorded and the state held never disagree.
func (b *Breaker) transition(ctx context.Context, to BreakerState, now time.Time) {
	// Every transition starts a cycle. A call admitted in an earlier one still
	// carries the generation it was granted, which is how record tells a current
	// answer from a stale one.
	b.generation++
	b.state = to

	if to == BreakerOpen {
		b.openedAt = now
		b.probing = 0
	}
	if to == BreakerClosed {
		b.probing = 0
	}

	if b.gauge != nil {
		b.gauge.Record(ctx, int64(to), metricAttributes(metrics.Labels{}.Dependency(b.dependency)))
	}
}
