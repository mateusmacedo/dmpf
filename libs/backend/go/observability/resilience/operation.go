package resilience

import (
	"context"
	"fmt"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
)

// Kind separates what may be retried from what may not: a unit of work is never
// wrapped by a retry decorator, because repeating a transaction is the caller's
// decision (RES-25, RES-34).
type Kind string

const (
	// Remote is a call to a dependency outside the process.
	Remote Kind = "remote"
	// UnitOfWork is a transactional boundary.
	UnitOfWork Kind = "unit_of_work"
)

// Operation is what the resilience sheet declares for one method. It is the
// input the retry evaluator reads, so it carries the idempotence of the method
// and the estimate the budget and the deadline factors need.
type Operation struct {
	Dependency string
	Method     string
	Kind       Kind

	// Idempotent says the method may be repeated safely.
	Idempotent bool

	// EffectAbsent decides, for a given error, that the attempt left no effect,
	// which makes a non-idempotent method repeatable for that error. A nil
	// function is false for every error, never an implicit yes.
	EffectAbsent func(error) bool

	// Deadline is the per-call budget the sheet declares for this method.
	Deadline time.Duration

	// EstimatedDuration is how long one attempt is expected to take. Zero is
	// invalid: the budget and deadline factors cannot be decided without it.
	EstimatedDuration time.Duration
}

// Call is a decorated call. Every decorator has this shape, so the composition
// is a chain of the same type and the order is the only thing that varies.
type Call func(ctx context.Context, op Operation, do func(context.Context) error) error

// Decorator wraps a call. It is applied from the outside in, so the first
// decorator of the canonical order is the outermost one.
type Decorator func(next Call) Call

// Direct is the innermost call: it invokes the work and nothing else.
func Direct(ctx context.Context, _ Operation, do func(context.Context) error) error {
	return do(ctx)
}

// ForRetry is the view the retry evaluator reads. The conversion lives here,
// and not in retry, because that package must not import this one: the retry
// decorator is in this package and the cycle would not compile.
func (o Operation) ForRetry() retry.Operation {
	return retry.Operation{
		Dependency:        o.Dependency,
		Method:            o.Method,
		Idempotent:        o.Idempotent,
		EffectAbsent:      o.EffectAbsent,
		EstimatedDuration: o.EstimatedDuration,
	}
}

// Validate refuses an operation the decorators cannot decide about. A call
// without a deadline does not exist (RES-05), and without an estimate the
// budget and deadline factors of the retry conjunction have nothing to compare.
func (o Operation) Validate() error {
	switch {
	case o.Dependency == "":
		return fmt.Errorf("resilience: operation declares no dependency")
	case o.Method == "":
		return fmt.Errorf("resilience: %s: operation declares no method", o.Dependency)
	case o.Kind == "":
		return fmt.Errorf("resilience: %s.%s: operation declares no kind", o.Dependency, o.Method)
	case o.Deadline <= 0:
		return fmt.Errorf("resilience: %s.%s: operation declares no deadline (RES-05)", o.Dependency, o.Method)
	case o.EstimatedDuration <= 0:
		return fmt.Errorf("resilience: %s.%s: operation declares no estimated duration", o.Dependency, o.Method)
	default:
		return nil
	}
}
