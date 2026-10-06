package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Executor is fixed once per context; a nil Instrumentation is the inert one.
type Executor[Res, Op any] struct {
	UoW             ports.UnitOfWork[Res]
	Inbox           func(Res) ports.Inbox
	Consumer        string
	Clock           ports.Clock
	IDs             ports.IDGenerator
	MaxEvents       int
	Policy          IdempotencyPolicy
	Authorize       Authorize[Op]
	Instrumentation ports.Instrumentation
}

// Command is one command's hooks. The Fingerprint comes ready from the context,
// because its fields fix the PayloadHash the inbox compares (IDM-04).
type Command[Res, Op, R any] struct {
	Operation   string
	Object      string
	Input       Op
	Fingerprint *Fingerprint
	Codec       OutcomeCodec[R]
	Run         func(ctx context.Context, res Res, identity Identity) (Outcome[R], error)
}

// Execute runs a command through FND-04 §3.2; errors come back without a prefix.
func Execute[Res, Op, R any](ctx context.Context, x Executor[Res, Op], c Command[Res, Op, R]) (Outcome[R], error) {
	var zero Outcome[R]

	inst := x.Instrumentation
	if inst == nil {
		inst = ports.NoInstrumentation()
	}
	ctx, end := inst.BeginOperation(ctx, c.Operation)

	if err := x.Authorize(ctx, c.Input); err != nil {
		end(ports.AuthorizationResult(err))
		return zero, err
	}

	identity := ResolveIdentity(x.Clock, x.IDs, x.MaxEvents)

	outcome, replayed := zero, false
	err := x.UoW.Within(ctx, func(ctx context.Context, res Res) error {
		var err error
		outcome, replayed, err = RunIdempotent(ctx, IdempotentCommand[R]{
			Inbox:       x.Inbox(res),
			Consumer:    x.Consumer,
			Operation:   c.Operation,
			Fingerprint: c.Fingerprint,
			Now:         identity.OccurredAt,
			Policy:      x.Policy,
			Codec:       c.Codec,
			Run:         func() (Outcome[R], error) { return c.Run(ctx, res, identity) },
		})
		return err
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcome.Category()
	end(ports.Result{Outcome: category})
	if !replayed {
		inst.Audit(ctx, ports.AuditEvent{Object: c.Object, Action: c.Operation, Outcome: category, At: identity.OccurredAt})
	}
	return outcome, nil
}
