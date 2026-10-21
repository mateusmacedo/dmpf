package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var ErrIncompleteAbandon = errors.New("application: external abandon incomplete")

// ErrAbandonOutsideTenant is Abandon's refusal of a context whose tenant is not
// the attempt's: Finish would scope the key to the wrong tenant (IDN-15).
var ErrAbandonOutsideTenant = errors.New("application: abandon outside the attempt's tenant")

type ExternalAbandon[R any] struct {
	UoW             ports.UnitOfWork[R]
	Ledger          func(R) ports.AttemptLedger
	Attempt         ports.Attempt
	Profile         ports.ExternalProfile
	QueryAvailable  bool
	Clock           ports.Clock
	Instrumentation ports.Instrumentation
	Apply           func(ctx context.Context, res R, step ExternalStep) error
}

// Abandon never resends (RES-43). ctx must carry the attempt's tenant, which the
// sweep in app mounts: this block builds no execution context.
func Abandon[R any](ctx context.Context, a ExternalAbandon[R]) (ports.Finished, error) {
	if err := a.validate(ctx); err != nil {
		return ports.Finished{}, err
	}
	ctx, end := a.Instrumentation.BeginOperation(ctx, operationAbandon)
	finished, err := a.run(ctx)
	end(operationResult(err))
	return finished, err
}

func (a ExternalAbandon[R]) run(ctx context.Context) (ports.Finished, error) {
	var finished ports.Finished
	err := a.UoW.Within(ctx, func(ctx context.Context, res R) error {
		var err error
		finished, err = a.Ledger(res).Finish(ctx, ports.AttemptOutcome{
			Key: a.Attempt.Key, N: a.Attempt.N, Class: ports.ClassUnknownSent, FinishedAt: a.Clock.Now(), Presumed: true,
		})
		if err != nil || finished.Result != ports.FinishRecorded {
			return err
		}
		step := NextStep(StepInput{Class: ports.ClassUnknownSent, Profile: a.Profile, QueryAvailable: a.QueryAvailable})
		return a.Apply(ctx, res, ExternalStep{Step: step, Attempt: a.Attempt, Class: ports.ClassUnknownSent})
	})
	if err != nil {
		return ports.Finished{}, fmt.Errorf("application: abandon attempt %d: %w", a.Attempt.N, err)
	}
	if finished.Recorded != 0 {
		ports.MarkExternalClass(ctx, finished.Recorded)
	}
	if finished.Result == ports.FinishConflict {
		ports.MarkExternalConflict(ctx)
	}
	return finished, nil
}

func (a ExternalAbandon[R]) validate(ctx context.Context) error {
	switch {
	case a.UoW == nil || a.Ledger == nil || a.Clock == nil || a.Apply == nil || a.Instrumentation == nil:
		return fmt.Errorf("%w: a transaction, a ledger accessor, a clock, an instrumentation and an apply are required", ErrIncompleteAbandon)
	case a.Attempt.Key == "" || a.Attempt.N < 1:
		return fmt.Errorf("%w: attempt %d of no key", ErrIncompleteAbandon, a.Attempt.N)
	}
	if err := ValidateExternalProfile(a.Profile); err != nil {
		return err
	}
	execution, err := ports.RequireExecutionContext(ctx)
	if err != nil {
		return err
	}
	if tenant, ok := execution.Tenant(); !ok || tenant != a.Attempt.Tenant {
		return fmt.Errorf("%w: attempt %d", ErrAbandonOutsideTenant, a.Attempt.N)
	}
	return nil
}
