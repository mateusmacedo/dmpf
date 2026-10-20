package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var ErrIncompleteResolution = errors.New("application: external resolution incomplete")

type ExternalResolution[R any] struct {
	UoW             ports.UnitOfWork[R]
	Ledger          func(R) ports.AttemptLedger
	Observation     ports.Observation
	Profile         ports.ExternalProfile
	QueryAvailable  bool
	Clock           ports.Clock
	Instrumentation ports.Instrumentation
	Apply           func(ctx context.Context, res R, step ExternalStep) error
}

// Resolve records the observation and applies the step only when it was
// recorded. A conflict is the returned result, not an error, so the commit that
// kept it stands and a caller's retry path does not hide it as a duplicate.
func Resolve[R any](ctx context.Context, r ExternalResolution[R]) (ports.Observed, error) {
	if err := r.validate(); err != nil {
		return ports.Observed{}, err
	}
	ctx, end := r.Instrumentation.BeginOperation(ctx, operationResolve)
	observed, err := r.run(ctx)
	end(operationResult(err))
	return observed, err
}

func (r ExternalResolution[R]) run(ctx context.Context) (ports.Observed, error) {
	var observed ports.Observed
	err := r.UoW.Within(ctx, func(ctx context.Context, res R) error {
		var err error
		observed, err = r.Ledger(res).Observe(ctx, r.Observation, r.Clock.Now())
		if err != nil || observed.Result != ports.ObserveRecorded {
			return err
		}
		step := NextStep(StepInput{
			Class: observed.Class, Finding: r.Observation.Finding, Profile: r.Profile,
			QueryAvailable: r.QueryAvailable, QueriesDone: observed.QueriesDone,
		})
		return r.Apply(ctx, res, ExternalStep{
			Step: step, Attempt: ports.Attempt{Key: r.Observation.Key, N: observed.N}, Class: observed.Class, Observation: r.Observation,
		})
	})
	if err != nil {
		return ports.Observed{}, fmt.Errorf("application: resolve: %w", err)
	}
	if observed.Class != 0 {
		ports.MarkExternalClass(ctx, observed.Class)
	}
	if observed.Result == ports.ObserveConflict {
		ports.MarkExternalConflict(ctx)
	}
	return observed, nil
}

func (r ExternalResolution[R]) validate() error {
	if r.UoW == nil || r.Ledger == nil || r.Clock == nil || r.Apply == nil || r.Instrumentation == nil {
		return fmt.Errorf("%w: a transaction, a ledger accessor, a clock, an instrumentation and an apply are required", ErrIncompleteResolution)
	}
	return ValidateExternalProfile(r.Profile)
}
