package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Query is the skeleton of a read: it walks step 1 before load and never opens
// a unit of work (UOW-11). The load error is returned unchanged, so the caller
// owns its wrapping. A nil inst is the inert instrumentation.
func Query[Op, S any](
	ctx context.Context,
	inst ports.Instrumentation,
	authz Authorize[Op],
	operation string,
	input Op,
	load func(context.Context) (S, error),
) (S, error) {
	if inst == nil {
		inst = ports.NoInstrumentation()
	}
	ctx, end := inst.BeginOperation(ctx, operation)

	var zero S
	if err := authz(ctx, input); err != nil {
		end(ports.AuthorizationResult(err))
		return zero, err
	}

	snapshot, err := load(ctx)
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshot, nil
}
