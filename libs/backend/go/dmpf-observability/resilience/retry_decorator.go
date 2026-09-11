package resilience

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
)

// AttrBudgetExhausted marks the span of an execution that spent its retry
// budget (RES-36).
const AttrBudgetExhausted = "dmpf.retry.budget_exhausted"

// RetryConfig is what the retry decorator needs beyond the sheet: the error
// taxonomy and the source of randomness, both injected by the composition root
// because the kernel defines neither.
type RetryConfig struct {
	Dependency  string
	Classifier  retry.Classifier
	MaxAttempts int
	Backoff     BackoffPolicy
	Rand        func() float64
}

// NewRetry builds the retry decorator. The sleeper is injected so the wait is
// advanced by a test instead of slept through.
func NewRetry(config RetryConfig, c clock.Clock, sleeper clock.Sleeper, instruments *metrics.Instruments) (Decorator, error) {
	if config.MaxAttempts <= 0 {
		return nil, fmt.Errorf("%w: %s declares no attempt ceiling (RES-33)", ErrBlankField, config.Dependency)
	}
	if c == nil || sleeper == nil {
		return nil, fmt.Errorf("%w: %s: the retry decorator needs a clock and a sleeper", ErrBlankField, config.Dependency)
	}

	var retries, exhausted metric64Counter
	if instruments != nil {
		retries, exhausted = instruments.Retries, instruments.BudgetExhausted
	}

	return func(next Call) Call {
		return func(ctx context.Context, op Operation, do func(context.Context) error) error {
			if err := guardUnitOfWork(op); err != nil {
				return err
			}

			span := trace.SpanFromContext(ctx)
			budget, carried := retry.BudgetFrom(ctx)

			// What the previous round claimed from the budget for the attempt
			// now running, and the wait it already paid for: together they are
			// settled against the real cost when that attempt returns.
			var claimed, waited time.Duration

			for attempt := 0; ; attempt++ {
				startedAt := c.Now()
				err := next(ctx, op, do)

				// Only a repeated attempt is settled: the budget pays for what
				// the retry added, not for the call the caller asked for. The
				// real cost is the wait that was served plus how long the
				// attempt took (RES-31).
				if attempt > 0 && carried {
					budget.Settle(claimed, waited+c.Now().Sub(startedAt))
					claimed, waited = 0, 0
				}
				if err == nil {
					return nil
				}

				if carried {
					armBudget(ctx, c, budget)
				}

				verdict := retry.Evaluate(retry.Input{
					Err:         err,
					Classifier:  config.Classifier,
					Operation:   op.ForRetry(),
					Attempt:     attempt,
					MaxAttempts: config.MaxAttempts,
					Budget:      budget,
					Remaining:   remainingOf(ctx, c),
					Backoff:     config.Backoff.ForRetry(),
					Rand:        config.Rand,
				})
				if !verdict.Allowed {
					// Denial by the budget factor IS the exhaustion RES-36 wants
					// reported: the balance need not reach zero, it only needs
					// to stop covering another attempt. The budget latches the
					// report, so it is counted once per execution.
					if verdict.Denied == retry.FactorBudget && carried && budget.ReportExhaustion() {
						reportExhaustion(ctx, span, exhausted, config.Dependency)
					}
					return err
				}

				// The claim is taken before sleeping, so a budget that cannot
				// afford the attempt is exhausted now and not after the process
				// already spent the time (RES-32). Reserving rather than reading
				// is what keeps two dependencies of the same execution from
				// both spending the same balance (RES-30).
				if carried {
					if !budget.Reserve(verdict.Need) {
						if budget.ReportExhaustion() {
							reportExhaustion(ctx, span, exhausted, config.Dependency)
						}
						return err
					}
					claimed, waited = verdict.Need, verdict.Wait
				}

				tracing.AttemptEvent(span, attempt+1, categoryOf(err))
				if retries != nil {
					retries.Add(ctx, 1, metricAttributes(
						metrics.Labels{}.Dependency(config.Dependency).ErrorCategory(categoryOf(err))))
				}

				if slept := sleeper(ctx, verdict.Wait); slept != nil {
					return errors.Join(err, slept)
				}
			}
		}
	}, nil
}

// guardUnitOfWork refuses to retry a transactional boundary. The check is per
// call, and not at construction, because the Operation only arrives with the
// call: the composition of a dependency does not know which method it will
// decorate (RES-25, RES-34).
func guardUnitOfWork(op Operation) error {
	if op.Kind == UnitOfWork {
		return fmt.Errorf("%w: %s.%s is a unit of work", ErrWrapsUnitOfWork, op.Dependency, op.Method)
	}
	return nil
}

// armBudget sizes an undeclared budget on the first failure, with half of the
// remaining deadline (RES-31). Arm is idempotent, so later failures do not
// reset what earlier ones spent.
func armBudget(ctx context.Context, c clock.Clock, budget *retry.Budget) {
	if deadline, declared := ctx.Deadline(); declared {
		budget.Arm(deadline.Sub(c.Now()))
	}
}

func remainingOf(ctx context.Context, c clock.Clock) time.Duration {
	deadline, declared := ctx.Deadline()
	if !declared {
		// No deadline is not "no time": the deadline factor must not deny a
		// retry the caller placed no limit on.
		return math.MaxInt64
	}
	return deadline.Sub(c.Now())
}

func reportExhaustion(ctx context.Context, span trace.Span, exhausted metric64Counter, dependency string) {
	if span != nil {
		span.SetAttributes(attribute.Bool(AttrBudgetExhausted, true))
	}
	if exhausted != nil {
		exhausted.Add(ctx, 1, metricAttributes(metrics.Labels{}.Dependency(dependency)))
	}
}

// categoryOf reads the category the taxonomy declared, never the message.
func categoryOf(err error) string {
	var categorized redact.Categorized
	if errors.As(err, &categorized) {
		if category := categorized.ErrorCategory(); category != "" {
			return category
		}
	}
	return redact.CategoryUnclassified
}
