package resilience

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

// DegradedResult marks an answer delivered in degraded mode. It is an error and
// not a plain value because degrading must not be silent (RES-38): a caller that
// received it knows the dependency failed, and one that ignores errors gets the
// same signal it would have got from the failure itself.
type DegradedResult struct {
	Dependency string
	Cause      error
}

func (d *DegradedResult) Error() string {
	return fmt.Sprintf("resilience: %s answered in degraded mode: %v", d.Dependency, d.Cause)
}

// Unwrap exposes the cause, so a caller that wants the original failure finds
// it with errors.Is and errors.As.
func (d *DegradedResult) Unwrap() error { return d.Cause }

// ErrorCategory is what redaction reads, so the degradation reaches a log or a
// span as a category and never as the message of the cause.
func (d *DegradedResult) ErrorCategory() string { return CategoryDegraded }

// ErrorCode is the stable identifier of a degraded answer.
func (d *DegradedResult) ErrorCode() string { return "RES-37" }

// NewDegradation builds the decorator of the declared mode. It refuses Defer,
// whose mechanism is the outbox of another story, at construction rather than
// per call: a mode that cannot work should fail when the composition is built.
func NewDegradation(mode Degradation, dependency string, instruments *metrics.Instruments) (Decorator, error) {
	var degraded, omitted metric64Counter
	if instruments != nil {
		degraded, omitted = instruments.Degraded, instruments.Omitted
	}

	switch mode {
	case Fail, "":
		return passThrough, nil

	case Degrade:
		return func(next Call) Call {
			return func(ctx context.Context, op Operation, do func(context.Context) error) error {
				err := next(ctx, op, do)
				if err == nil || givenUp(ctx, err) {
					return err
				}

				countDependency(ctx, degraded, dependency)
				return &DegradedResult{Dependency: dependency, Cause: err}
			}
		}, nil

	case Ignore:
		return func(next Call) Call {
			return func(ctx context.Context, op Operation, do func(context.Context) error) error {
				err := next(ctx, op, do)
				if err == nil || givenUp(ctx, err) {
					return err
				}

				countDependency(ctx, omitted, dependency)
				return nil
			}
		}, nil

	case Defer:
		return nil, fmt.Errorf("%w: %s declares defer", ErrDeferIsOutbox, dependency)

	default:
		return nil, fmt.Errorf("%w: %s declares the unknown degradation mode %q", ErrBlankField, dependency, mode)
	}
}

func passThrough(next Call) Call { return next }

// givenUp reports a caller that stopped waiting. Degrading it would answer a
// request nobody is listening to, and counting it would blame the dependency
// for a decision of the caller.
func givenUp(ctx context.Context, err error) bool {
	return errors.Is(err, ErrCancelled) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(ctx.Err(), context.Canceled)
}

func countDependency(ctx context.Context, instrument metric64Counter, dependency string) {
	if instrument == nil {
		return
	}
	instrument.Add(ctx, 1, metricAttributes(metrics.Labels{}.Dependency(dependency)))
}
