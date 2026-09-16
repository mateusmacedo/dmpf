package reservationsapp

import (
	"errors"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

func (s Service) instrumentation() dmpfports.Instrumentation {
	if s.Instrumentation == nil {
		return dmpfports.NoInstrumentation()
	}
	return s.Instrumentation
}

// authorizationResult reports Denied only for a declared denial: inferring a
// refusal from an unrelated error would be a false negative of access.
func authorizationResult(err error) dmpfports.Result {
	if errors.Is(err, dmpfports.ErrDenied) {
		return dmpfports.Result{Outcome: dmpfports.OutcomeDenied}
	}
	return dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: err}
}

func outcomeCategory[R any](outcome dmpfapplication.Outcome[R]) dmpfports.OutcomeCategory {
	if _, refused := outcome.Rejection(); refused {
		return dmpfports.OutcomeRejected
	}
	return dmpfports.OutcomeAccepted
}
