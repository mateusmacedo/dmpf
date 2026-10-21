package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	operationDispatch = "external.dispatch"
	operationResolve  = "external.resolve"
	operationAbandon  = "external.abandon"
)

var ErrIncompleteDispatch = errors.New("application: external dispatch incomplete")

type ExternalStep struct {
	Step        Step
	Attempt     ports.Attempt
	Class       ports.ExternalClass
	Observation ports.Observation
}

// StaleAfter is the RES-21 deadline plus the margin, in ns. SettleContext is
// injected because the outcome is recorded beyond the caller's cancellation and
// this block imports no time: the composition root sets that deadline.
type ExternalDispatch[R any] struct {
	UoW             ports.UnitOfWork[R]
	Store           ports.AttemptLedger
	Ledger          func(R) ports.AttemptLedger
	Inbox           func(R) ports.Inbox
	Receipt         ports.Receipt
	Command         ports.ExternalCommand
	Key             ports.OutboundKey
	Payload         []byte
	Profile         ports.ExternalProfile
	QueryAvailable  bool
	StaleAfter      int64
	Clock           ports.Clock
	SettleContext   func(context.Context) (context.Context, context.CancelFunc)
	Instrumentation ports.Instrumentation
	Apply           func(ctx context.Context, res R, step ExternalStep) error
}

type settlement struct {
	attempt     ports.Attempt
	class       ports.ExternalClass
	observation ports.Observation
	owned       bool
	presumed    bool
}

func (s settlement) outcome(key ports.OutboundKey, at ports.Instant) ports.AttemptOutcome {
	return ports.AttemptOutcome{
		Key: key, N: s.attempt.N, Class: s.class, Observation: s.observation, FinishedAt: at, Presumed: s.presumed,
	}
}

// The partner is called outside any transaction (OBX-07, OBX-16, INB-07); the
// step is applied only when this call recorded the outcome (RES-42, RES-43).
func Dispatch[R any](ctx context.Context, d ExternalDispatch[R]) (Disposition, error) {
	if err := d.validate(); err != nil {
		return Classify(err), err
	}
	ctx, end := d.Instrumentation.BeginOperation(ctx, operationDispatch)
	disposition, err := d.run(ctx)
	end(operationResult(err))
	return disposition, err
}

func (d ExternalDispatch[R]) run(ctx context.Context) (Disposition, error) {
	start, err := d.Store.Begin(ctx, d.Key, d.Receipt.MessageID, d.Clock.Now())
	if err != nil {
		err = fmt.Errorf("application: dispatch: begin: %w", err)
		return Classify(err), err
	}

	var s settlement
	err = start.Match(
		func(a ports.Attempt) error {
			s, err = d.submit(ctx, a)
			return err
		},
		func(a ports.Attempt) error {
			s, err = d.resume(a)
			return err
		},
		func(a ports.Attempt, class ports.ExternalClass) error {
			s = settlement{attempt: a, class: class}
			return nil
		},
	)
	if err != nil {
		return Classify(err), err
	}
	ports.MarkExternalClass(ctx, s.class)
	return d.settle(ctx, s)
}

func (d ExternalDispatch[R]) validate() error {
	switch {
	case d.UoW == nil || d.Store == nil || d.Ledger == nil || d.Inbox == nil:
		return fmt.Errorf("%w: a transaction, a store and both accessors are required", ErrIncompleteDispatch)
	case d.Command == nil || d.Clock == nil || d.Apply == nil:
		return fmt.Errorf("%w: a command, a clock and an apply are required", ErrIncompleteDispatch)
	case d.SettleContext == nil || d.Instrumentation == nil:
		return fmt.Errorf("%w: a settle context and an instrumentation are required", ErrIncompleteDispatch)
	case d.Key == "":
		return fmt.Errorf("%w: no outbound key", ErrIncompleteDispatch)
	case d.StaleAfter <= 0:
		return fmt.Errorf("%w: stale cutoff %d", ErrIncompleteDispatch, d.StaleAfter)
	}
	return ValidateExternalProfile(d.Profile)
}

func (d ExternalDispatch[R]) settleContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return d.SettleContext(context.WithoutCancel(ctx))
}

// resume is a redelivery that found the attempt begun: within the cutoff its
// owner may still have the call in flight, so nothing is closed nor resent.
func (d ExternalDispatch[R]) resume(a ports.Attempt) (settlement, error) {
	if int64(d.Clock.Now()-a.StartedAt) < d.StaleAfter {
		cause := fmt.Errorf("application: dispatch: attempt %d: %w", a.N, ports.ErrAttemptInFlight)
		return settlement{}, NewFailure(TransientDependency, true, cause)
	}
	return settlement{attempt: a, class: ports.ClassUnknownSent, owned: true, presumed: true}, nil
}

func (d ExternalDispatch[R]) submit(ctx context.Context, a ports.Attempt) (settlement, error) {
	class, observation, cause := d.Command.Submit(ctx, d.Key, d.Payload)
	if class == 0 {
		// WHY: a zero class is a provider defect after the call left (ERR-11, ERR-31):
		// it is recorded as the uncertain class no resend follows, never dropped.
		class = ports.ClassUnknownAmbiguous
	}
	s := settlement{attempt: a, class: class, observation: observation, owned: true}
	if !class.Retryable() {
		return s, nil
	}

	ports.MarkExternalClass(ctx, class)
	settleCtx, cancel := d.settleContext(ctx)
	defer cancel()
	finished, err := d.Store.Finish(settleCtx, s.outcome(d.Key, d.Clock.Now()))
	if err != nil {
		return settlement{}, fmt.Errorf("application: dispatch: finish attempt %d: %w", a.N, err)
	}
	switch finished.Result {
	case ports.FinishConflict:
		ports.MarkExternalConflict(ctx)
	case ports.FinishSaturated:
		ports.MarkExternalSaturation(ctx)
	}
	if finished.Result != ports.FinishRecorded && !finished.Recorded.Retryable() {
		s.class = finished.Recorded
		return s, nil
	}
	return settlement{}, NewFailure(TransientDependency, true, cause)
}

func (d ExternalDispatch[R]) settle(ctx context.Context, s settlement) (Disposition, error) {
	ctx, cancel := d.settleContext(ctx)
	defer cancel()

	var (
		disposition Disposition
		finished    ports.Finished
	)
	err := d.UoW.Within(ctx, func(ctx context.Context, res R) error {
		finished = ports.Finished{}
		reception, err := d.Inbox(res).Register(ctx, d.Receipt)
		if err != nil {
			return err
		}
		closing := func(as Disposition) func() error {
			return func() error {
				disposition = as
				finished, err = d.close(ctx, res, s)
				return err
			}
		}
		return reception.Match(
			func(pending ports.Pending) error {
				disposition = R1D1
				finished, err = d.finish(ctx, res, s, pending)
				return err
			},
			closing(R2), closing(R3), closing(R4),
		)
	})
	if err != nil {
		err = fmt.Errorf("application: dispatch: %w", err)
		return Classify(err), err
	}
	markFinished(ctx, finished)
	return disposition, nil
}

func markFinished(ctx context.Context, finished ports.Finished) {
	if finished.Recorded != 0 {
		ports.MarkExternalClass(ctx, finished.Recorded)
	}
	switch finished.Result {
	case ports.FinishConflict:
		ports.MarkExternalConflict(ctx)
	case ports.FinishSaturated:
		ports.MarkExternalSaturation(ctx)
	}
}

// close is a redelivery the inbox already settled (R2, R3) or refused (R4): an
// attempt this call began is closed with its class, unapplied, so the sweep finds
// no begun attempt to apply a step for.
func (d ExternalDispatch[R]) close(ctx context.Context, res R, s settlement) (ports.Finished, error) {
	if !s.owned {
		return ports.Finished{}, nil
	}
	return d.Ledger(res).Finish(ctx, s.outcome(d.Key, d.Clock.Now()))
}

func (d ExternalDispatch[R]) finish(ctx context.Context, res R, s settlement, pending ports.Pending) (ports.Finished, error) {
	now := d.Clock.Now()
	finished, err := d.Ledger(res).Finish(ctx, s.outcome(d.Key, now))
	if err != nil {
		return ports.Finished{}, err
	}
	if finished.Result == ports.FinishRecorded {
		step := NextStep(StepInput{Class: s.class, Finding: s.observation.Finding, Profile: d.Profile, QueryAvailable: d.QueryAvailable})
		if err := d.Apply(ctx, res, ExternalStep{Step: step, Attempt: s.attempt, Class: s.class, Observation: s.observation}); err != nil {
			return ports.Finished{}, err
		}
	}
	return finished, pending.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: now})
}

func operationResult(err error) ports.Result {
	if err != nil {
		return ports.Result{Outcome: ports.OutcomeFailed, Err: err}
	}
	return ports.Result{Outcome: ports.OutcomeAccepted}
}
