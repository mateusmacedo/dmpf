package retry

import "time"

// Operation is what the evaluator needs to know about the call: whether
// repeating it is safe and how long one attempt takes.
//
// It is declared here instead of read from the resilience sheet because the
// decorator that applies this verdict lives in resilience, and importing that
// package back would close a cycle the compiler refuses. resilience.Operation
// converts into this view.
type Operation struct {
	Dependency string
	Method     string

	// Idempotent says the method may be repeated safely.
	Idempotent bool

	// EffectAbsent decides, for a given error, that the attempt left no effect.
	// A nil function is false for every error, never an implicit yes.
	EffectAbsent func(error) bool

	// EstimatedDuration is how long one attempt is expected to take, and is
	// what the budget and deadline factors measure against.
	EstimatedDuration time.Duration
}

// Factor names the condition that denied the retry. The zero value means none
// denied, which pairs with Verdict.Allowed being true.
type Factor int

const (
	// FactorNone is the absence of a denial.
	FactorNone Factor = iota
	// FactorAttempts is the attempt ceiling of RES-33, checked before the four.
	FactorAttempts
	// FactorRetryable is factor 1: the taxonomy did not call the error transient.
	FactorRetryable
	// FactorIdempotent is factor 2: repeating the method is not safe for this error.
	FactorIdempotent
	// FactorBudget is factor 3: the execution budget does not cover another attempt.
	FactorBudget
	// FactorDeadline is factor 4: the remaining deadline does not cover another attempt.
	FactorDeadline
)

func (f Factor) String() string {
	switch f {
	case FactorAttempts:
		return "attempts"
	case FactorRetryable:
		return "retryable"
	case FactorIdempotent:
		return "idempotent"
	case FactorBudget:
		return "budget"
	case FactorDeadline:
		return "deadline"
	default:
		return "none"
	}
}

// Input is everything Evaluate reads. It is a struct and not a parameter list
// because the conjunction has five conditions and positional arguments would
// make a reordering silent.
type Input struct {
	Err         error
	Classifier  Classifier
	Operation   Operation
	Attempt     int
	MaxAttempts int
	Budget      *Budget
	Remaining   time.Duration
	Backoff     Backoff
	Rand        func() float64
}

// Verdict is the decision. Wait is the backoff before the next attempt and is
// only meaningful when Allowed.
type Verdict struct {
	Allowed bool
	Denied  Factor
	Wait    time.Duration

	// Need is what the attempt is expected to cost — the backoff wait plus the
	// estimated duration of the call. The caller reserves exactly this from the
	// budget, and settles it against what the attempt really took.
	Need time.Duration
}

// Evaluate is the conjunction of RES-27: every factor must hold for one more
// attempt to be allowed, and the first false one ends the evaluation and is
// named in Denied. It is a pure function of Input, so the decision is testable
// without a clock, a network or a transaction.
//
// Being pure is also its limit: the budget it reads is shared with every other
// dependency of the execution, and reading a balance is not claiming it. The
// caller reserves Verdict.Need before acting on an allowance, and treats a
// refused reservation as denial by the budget factor.
func Evaluate(in Input) Verdict {
	// The ceiling counts the original call, so the attempt that just failed
	// already spent one of the allowance (RES-33).
	if in.Attempt+1 >= in.MaxAttempts {
		return Verdict{Denied: FactorAttempts}
	}

	wait := in.Backoff.Next(in.Attempt, in.Rand)
	need := in.Operation.EstimatedDuration + wait

	if in.Classifier == nil || in.Classifier(in.Err) != Retryable {
		return Verdict{Denied: FactorRetryable}
	}
	if !repeatable(in.Operation, in.Err) {
		return Verdict{Denied: FactorIdempotent}
	}
	if in.Budget == nil || in.Budget.Remaining() < need {
		return Verdict{Denied: FactorBudget}
	}
	if in.Remaining < need {
		return Verdict{Denied: FactorDeadline}
	}

	return Verdict{Allowed: true, Wait: wait, Need: need}
}

func repeatable(operation Operation, err error) bool {
	if operation.Idempotent {
		return true
	}
	return operation.EffectAbsent != nil && operation.EffectAbsent(err)
}
