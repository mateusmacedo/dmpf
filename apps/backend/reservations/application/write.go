package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type decision[R any] func(*domain.Reservation, domain.Instant) (kernel.Accepted[R], *kernel.Rejection)

// write is the nine steps of FND-04 §3.2 shared by Reserve and Cancel. Identity
// is resolved before the transaction, because a re-execution would mint new
// identity for the same fact (UOW-09).
func write[R any](
	ctx context.Context,
	s Service,
	operation string,
	cmd Operation,
	order domain.OrderID,
	fingerprint *usecase.Fingerprint,
	codec usecase.OutcomeCodec[R],
	decide decision[R],
) (usecase.Outcome[R], error) {
	var zero usecase.Outcome[R]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, operation)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(ports.AuthorizationResult(err))
		return zero, err
	}

	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome, replayed := zero, false
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		var err error
		outcome, replayed, err = usecase.RunIdempotent(ctx, usecase.IdempotentCommand[R]{
			Inbox:       res.Commands,
			Consumer:    CommandConsumer,
			Operation:   operation,
			Fingerprint: fingerprint,
			Now:         identity.OccurredAt,
			Policy:      s.Idempotency,
			Codec:       codec,
			Run:         func() (usecase.Outcome[R], error) { return decideAndWrite(ctx, res, order, identity, decide) },
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
		instrumentation.Audit(ctx, ports.AuditEvent{
			Object:  string(order),
			Action:  operation,
			Outcome: category,
			At:      identity.OccurredAt,
		})
	}
	return outcome, nil
}

func decideAndWrite[R any](ctx context.Context, res Resources, order domain.OrderID, identity usecase.Identity, decide decision[R]) (usecase.Outcome[R], error) {
	var zero usecase.Outcome[R]

	reservation, stored, err := loadOrCreate(ctx, res, order)
	if err != nil {
		return zero, err
	}

	accepted, rejection := decide(reservation, domain.Instant(identity.OccurredAt))
	if rejection != nil {
		// A refusal returns no error: the transaction commits it with no
		// business effect, apart from a technical failure (DEC-04, FND-04 §3.2).
		return usecase.Rejected[R](rejection), nil
	}

	if err := res.Reservations.Save(ctx, order, reservation.Snapshot(), stored); err != nil {
		return zero, err
	}
	if err := usecase.Enqueue(ctx, res.Outbox, identity, origin(order), stored+1, accepted.Events()); err != nil {
		return zero, err
	}
	return usecase.Accepted(accepted.Response()), nil
}

func loadOrCreate(ctx context.Context, res Resources, id domain.OrderID) (*domain.Reservation, ports.Version, error) {
	snapshot, stored, err := res.Reservations.Load(ctx, id)
	switch {
	case errors.Is(err, ports.ErrNotFound):
		return domain.NewReservation(id), 0, nil
	case err != nil:
		return nil, 0, fmt.Errorf("application: load reservation %s: %w", id, err)
	default:
		return domain.FromSnapshot(snapshot), stored, nil
	}
}
