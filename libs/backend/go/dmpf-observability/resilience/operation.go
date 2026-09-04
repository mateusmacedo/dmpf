package resilience

import "time"

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
