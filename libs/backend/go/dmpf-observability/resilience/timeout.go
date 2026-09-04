package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
)

// Timeout derives the effective deadline of the call and counts what it ran
// into. The reserve is the wait the next attempt would need, so the budget
// term does not hand the whole balance to the attempt in flight; whoever
// composes reads it from the sheet's backoff.
//
// A nil set of instruments is accepted: a test exercises the derivation without
// a meter, and the decorator still refuses and classifies.
func Timeout(c clock.Clock, instruments *metrics.Instruments, reserve time.Duration) Decorator {
	return func(next Call) Call {
		return func(ctx context.Context, op Operation, do func(context.Context) error) error {
			effective := EffectiveDeadline(ctx, c, op, reserve)
			if effective <= 0 {
				count(ctx, instrumentOf(instruments, deadlineExceeded), op)
				return fmt.Errorf("%w: %s.%s has no time left", ErrDeadlineExceeded, op.Dependency, op.Method)
			}

			bounded, cancel := c.WithTimeout(ctx, effective)
			defer cancel()

			err := next(bounded, op, do)
			return classify(ctx, instruments, op, bounded, err)
		}
	}
}

// EffectiveDeadline is the minimum of the three terms of RES-06: the deadline
// the method declares, what is left of the caller's deadline, and the retry
// budget minus the reserve.
//
// A context with no deadline contributes nothing, and the method's own deadline
// decides — which is why RES-05 makes that deadline mandatory. A budget with no
// balance also contributes nothing: an undimensioned budget restricts retries,
// never the attempt in flight.
func EffectiveDeadline(ctx context.Context, c clock.Clock, op Operation, reserve time.Duration) time.Duration {
	effective := op.Deadline

	if deadline, declared := ctx.Deadline(); declared {
		if remaining := deadline.Sub(c.Now()); remaining < effective {
			effective = remaining
		}
	}

	if budget, carried := retry.BudgetFrom(ctx); carried {
		if balance := budget.Remaining(); balance > 0 {
			if available := balance - reserve; available < effective {
				effective = available
			}
		}
	}

	return effective
}

type counter int

const (
	deadlineExceeded counter = iota
	cancellations
)

// classify tells a deadline apart from a cancellation. The distinction is the
// point: running late is the platform's problem and giving up is the caller's,
// and one metric that mixed them would hide both (CTX-28).
func classify(ctx context.Context, instruments *metrics.Instruments, op Operation, bounded context.Context, err error) error {
	if err == nil {
		return nil
	}

	// The caller's cancellation is read first: it also closes the bounded
	// context, and reading that one first would report every giving up as a
	// deadline of ours.
	if errors.Is(ctx.Err(), context.Canceled) {
		count(ctx, instrumentOf(instruments, cancellations), op)
		return fmt.Errorf("%w: %s.%s: %w", ErrCancelled, op.Dependency, op.Method, err)
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(bounded.Err(), context.DeadlineExceeded) {
		count(ctx, instrumentOf(instruments, deadlineExceeded), op)
		return fmt.Errorf("%w: %s.%s: %w", ErrDeadlineExceeded, op.Dependency, op.Method, err)
	}

	if errors.Is(err, context.Canceled) {
		count(ctx, instrumentOf(instruments, cancellations), op)
		return fmt.Errorf("%w: %s.%s: %w", ErrCancelled, op.Dependency, op.Method, err)
	}

	return err
}

func instrumentOf(instruments *metrics.Instruments, which counter) metric64Counter {
	if instruments == nil {
		return nil
	}
	switch which {
	case cancellations:
		return instruments.Cancellations
	default:
		return instruments.DeadlineExceeded
	}
}

func count(ctx context.Context, instrument metric64Counter, op Operation) {
	if instrument == nil {
		return
	}

	labels := metrics.Labels{}.Dependency(op.Dependency).Operation(op.Method)
	instrument.Add(ctx, 1, metricAttributes(labels))
}
