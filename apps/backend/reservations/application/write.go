package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

type decision[R any] func(*domain.Reservation, domain.Instant) (kernel.Accepted[R], *kernel.Rejection)

// write is the nine steps of FND-04 §3.2 shared by Reserve and Cancel. Identity
// is resolved before the transaction, because a re-execution would mint new
// identity for the same fact (UOW-09).
func write[R any](ctx context.Context, s Service, operation string, cmd Operation, order domain.OrderID, decide decision[R]) (application.Outcome[R], error) {
	var zero application.Outcome[R]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, operation)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := application.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		reservation, stored, err := loadOrCreate(ctx, res, order)
		if err != nil {
			return err
		}

		accepted, rejection := decide(reservation, domain.Instant(identity.OccurredAt))
		if rejection != nil {
			// Committing a transaction with no effect keeps a refusal apart from a
			// technical failure, which DEC-04 forbids to conflate (FND-04 §3.2).
			outcome = application.Rejected[R](rejection)
			return nil
		}

		if err := res.Reservations.Save(ctx, order, reservation.Snapshot(), stored); err != nil {
			return err
		}
		if err := enqueueAll(ctx, res.Outbox, identity, order, stored+1, accepted.Events()); err != nil {
			return err
		}

		outcome = application.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcomeCategory(outcome)
	end(ports.Result{Outcome: category})
	instrumentation.Audit(ctx, ports.AuditEvent{
		Object:  string(order),
		Action:  operation,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
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
