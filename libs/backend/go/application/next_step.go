package application

import "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

// Step is what the external command cycle does next; the zero value is no step.
type Step uint8

const (
	StepConfirm Step = iota + 1
	StepAwait
	StepRetry
	StepFail
	StepQuery
	StepReconcile
	StepSuspend
)

func (s Step) String() string {
	switch s {
	case StepConfirm:
		return "confirm"
	case StepAwait:
		return "await"
	case StepRetry:
		return "retry"
	case StepFail:
		return "fail"
	case StepQuery:
		return "query"
	case StepReconcile:
		return "reconcile"
	case StepSuspend:
		return "suspend"
	default:
		return "invalid"
	}
}

// StepInput is everything NextStep reads, a struct so that no reordering of
// its five conditions goes silent.
type StepInput struct {
	Class          ports.ExternalClass
	Finding        ports.Finding
	Profile        ports.ExternalProfile
	QueryAvailable bool
	QueriesDone    int
}

// NextStep is pure over a validated profile; waits are the caller's, the resend budget
// RES-27's, and an unknown class is uncertain (ERR-11). Await with no
// query port is a callback wait, bounded by time and RES-43's owner, not MaxQueries.
func NextStep(in StepInput) Step {
	if in.Finding != 0 {
		return stepForFinding(in)
	}
	switch in.Class {
	case ports.ClassAccepted:
		if in.Profile.Efficacy == ports.EfficacyOnSubmit {
			return StepConfirm
		}
		return awaitOrReconcile(in)
	case ports.ClassRetryableTransport, ports.ClassRetryableUpstream:
		return StepRetry
	case ports.ClassFinalValidation, ports.ClassFinalBusiness, ports.ClassFinalAuthorization:
		return StepFail
	case ports.ClassSchemaMismatch:
		return StepSuspend
	default:
		return queryOrReconcile(in)
	}
}

func stepForFinding(in StepInput) Step {
	switch in.Finding {
	case ports.FindingApplied:
		return StepConfirm
	case ports.FindingRejected:
		return StepFail
	case ports.FindingAbsent:
		if in.Profile.KeySemantics == ports.KeyNative || in.Profile.KeySemantics == ports.KeyHeader || in.Profile.AuthoritativeAbsence {
			return StepRetry
		}
		return StepReconcile
	case ports.FindingPending:
		return awaitOrReconcile(in)
	default:
		return queryOrReconcile(in)
	}
}

func awaitOrReconcile(in StepInput) Step {
	if in.QueriesDone < in.Profile.MaxQueries {
		return StepAwait
	}
	return StepReconcile
}

func queryOrReconcile(in StepInput) Step {
	if in.QueryAvailable && in.QueriesDone < in.Profile.MaxQueries {
		return StepQuery
	}
	return StepReconcile
}
