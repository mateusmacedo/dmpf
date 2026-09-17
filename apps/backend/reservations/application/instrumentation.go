package application

import (
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func (s Service) instrumentation() ports.Instrumentation {
	if s.Instrumentation == nil {
		return ports.NoInstrumentation()
	}
	return s.Instrumentation
}

// authorizationResult reports Denied only for a declared denial: inferring a
// refusal from an unrelated error would be a false negative of access.
func authorizationResult(err error) ports.Result {
	if errors.Is(err, ports.ErrDenied) {
		return ports.Result{Outcome: ports.OutcomeDenied}
	}
	return ports.Result{Outcome: ports.OutcomeFailed, Err: err}
}

func outcomeCategory[R any](outcome application.Outcome[R]) ports.OutcomeCategory {
	if _, refused := outcome.Rejection(); refused {
		return ports.OutcomeRejected
	}
	return ports.OutcomeAccepted
}
