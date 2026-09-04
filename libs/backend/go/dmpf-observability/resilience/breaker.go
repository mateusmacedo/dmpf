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

	mu       sync.Mutex
	window   []observation
	state    BreakerState
	openedAt time.Time
	probing  int
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
			if err := b.admit(ctx); err != nil {
				return err
			}

			err := next(ctx, op, do)
			b.record(ctx, err)
			return err
		}
	}
}

// admit decides whether the call goes through, moving the breaker to
// half-openness when the cooldown has elapsed.
func (b *Breaker) admit(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()

	if b.state == BreakerOpen {
		if now.Sub(b.openedAt) < b.policy.Cooldown {
			return fmt.Errorf("%w: %s is open", ErrBreakerOpen, b.dependency)
		}
		b.transition(ctx, BreakerHalfOpen, now)
	}

	if b.state == BreakerHalfOpen {
		if b.probing >= b.probes() {
			return fmt.Errorf("%w: %s is half-open and its probes are in flight", ErrBreakerOpen, b.dependency)
		}
		b.probing++
	}

	return nil
}

// record accounts for the outcome. A cancellation by the caller is not counted:
// the dependency was never given the chance to answer, and holding it against
// the dependency would open the breaker on a client that gave up.
func (b *Breaker) record(ctx context.Context, err error) {
	if errors.Is(err, ErrCancelled) {
		b.releaseProbe()
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()
	failed := err != nil

	if b.state == BreakerHalfOpen {
		b.probing--
		if failed {
			b.window = nil
			b.transition(ctx, BreakerOpen, now)
			return
		}
		b.window = nil
		b.transition(ctx, BreakerClosed, now)
		return
	}

	b.window = append(b.prune(now), observation{at: now, failed: failed})
	if b.shouldOpen() {
		b.transition(ctx, BreakerOpen, now)
	}
}

func (b *Breaker) releaseProbe() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == BreakerHalfOpen && b.probing > 0 {
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
