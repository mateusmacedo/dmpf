package retry

import (
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
)

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
	Operation   resilience.Operation
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
}

// Evaluate is the conjunction of RES-27: every factor must hold for one more
// attempt to be allowed, and the first false one ends the evaluation and is
// named in Denied. It is a pure function of Input, so the decision is testable
// without a clock, a network or a transaction.
func Evaluate(in Input) Verdict {
	if in.Attempt >= in.MaxAttempts {
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

	return Verdict{Allowed: true, Wait: wait}
}

func repeatable(operation resilience.Operation, err error) bool {
	if operation.Idempotent {
		return true
	}
	return operation.EffectAbsent != nil && operation.EffectAbsent(err)
}
