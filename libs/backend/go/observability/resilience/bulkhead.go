package resilience

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// Bulkhead is the pool of one dependency (RES-13, RES-14): a semaphore of the
// declared size, a queue of the same size, and an acquisition that gives up
// after the declared wait.
//
// The two channels are separate on purpose. Admission bounds how many calls are
// in the system at all and is taken without blocking, so a call arriving at a
// full pool and a full queue is refused at once instead of waiting for a wait
// it cannot win. Only a call that got an admission waits for a slot.
type Bulkhead struct {
	dependency string
	policy     BulkheadPolicy
	clock      clock.Clock
	rejections metric64Counter

	admissions chan struct{}
	slots      chan struct{}
}

// NewBulkhead builds the pool of a dependency. A queue of zero takes the size
// of the pool, which is what RES-13 declares. A nil set of instruments is
// accepted: a test exercises the saturation without a meter.
func NewBulkhead(dependency string, policy BulkheadPolicy, c clock.Clock, instruments *metrics.Instruments) *Bulkhead {
	pool := policy.Pool
	if pool <= 0 {
		pool = DefaultBulkheadPool
	}
	queue := policy.Queue
	if queue <= 0 {
		queue = pool
	}

	bulkhead := &Bulkhead{
		dependency: dependency,
		policy:     policy,
		clock:      c,
		admissions: make(chan struct{}, pool+queue),
		slots:      make(chan struct{}, pool),
	}
	if instruments != nil {
		bulkhead.rejections = instruments.BulkheadRejections
	}
	return bulkhead
}

// Decorate is the decorator of the canonical order.
func (b *Bulkhead) Decorate() Decorator {
	return func(next Call) Call {
		return func(ctx context.Context, op Operation, do func(context.Context) error) error {
			release, err := b.acquire(ctx)
			if err != nil {
				return err
			}
			defer release()

			return next(ctx, op, do)
		}
	}
}

// acquire takes an admission and then a slot, returning the release of both.
func (b *Bulkhead) acquire(ctx context.Context) (func(), error) {
	select {
	case b.admissions <- struct{}{}:
	default:
		return nil, b.refuse(ctx, "the pool and its queue are full")
	}

	release := func() {
		<-b.slots
		<-b.admissions
	}

	// A free slot is taken without arming anything: a timer for a wait that
	// does not happen leaks until it fires, which the platform forbids in a hot
	// path, and would also make the pool indistinguishable from the queue to an
	// observer counting pending alarms.
	select {
	case b.slots <- struct{}{}:
		return release, nil
	default:
	}

	acquisition := b.policy.Acquisition
	if acquisition <= 0 {
		acquisition = DefaultBulkheadAcquire
	}
	waiting := b.clock.NewTimer(acquisition)
	defer waiting.Stop()

	select {
	case b.slots <- struct{}{}:
		return release, nil

	case <-waiting.C():
		<-b.admissions
		return nil, b.refuse(ctx, "no slot became free within the acquisition window")

	case <-ctx.Done():
		<-b.admissions
		return nil, ctx.Err()
	}
}

// refuse counts the rejection and records it on the span, so a saturated pool
// is visible in both signals (RES-23).
func (b *Bulkhead) refuse(ctx context.Context, why string) error {
	if b.rejections != nil {
		b.rejections.Add(ctx, 1, metricAttributes(metrics.Labels{}.Dependency(b.dependency)))
	}
	tracing.BulkheadSaturated(trace.SpanFromContext(ctx), b.dependency)

	return fmt.Errorf("%w: %s: %s", ErrBulkheadSaturated, b.dependency, why)
}
