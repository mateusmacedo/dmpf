package reservationsapp

import (
	"context"
	"errors"
	"fmt"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

type decision[R any] func(*reservations.Reservation, reservations.Instant) (dmpfdomain.Accepted[R], *dmpfdomain.Rejection)

// write is the nine steps of FND-04 §3.2 shared by Reserve and Cancel. Identity
// is resolved before the transaction, because a re-execution would mint new
// identity for the same fact (UOW-09).
func write[R any](ctx context.Context, s Service, operation string, cmd Command, order reservations.OrderID, decide decision[R]) (dmpfapplication.Outcome[R], error) {
	var zero dmpfapplication.Outcome[R]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, operation)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		reservation, stored, err := loadOrCreate(ctx, res, order)
		if err != nil {
			return err
		}

		accepted, rejection := decide(reservation, reservations.Instant(identity.OccurredAt.Unix()))
		if rejection != nil {
			// Committing a transaction with no effect keeps a refusal apart from a
			// technical failure, which DEC-04 forbids to conflate (FND-04 §3.2).
			outcome = dmpfapplication.Rejected[R](rejection)
			return nil
		}

		if err := res.Reservations.Save(ctx, order, reservation.Snapshot(), stored); err != nil {
			return err
		}
		if err := enqueueAll(ctx, res.Outbox, identity, order, stored+1, accepted.Events()); err != nil {
			return err
		}

		outcome = dmpfapplication.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcomeCategory(outcome)
	end(dmpfports.Result{Outcome: category})
	instrumentation.Audit(ctx, dmpfports.AuditEvent{
		Object:  string(order),
		Action:  operation,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
}

func loadOrCreate(ctx context.Context, res Resources, id reservations.OrderID) (*reservations.Reservation, dmpfports.Version, error) {
	snapshot, stored, err := res.Reservations.Load(ctx, id)
	switch {
	case errors.Is(err, dmpfports.ErrNotFound):
		return reservations.NewReservation(id), 0, nil
	case err != nil:
		return nil, 0, fmt.Errorf("reservationsapp: load reservation %s: %w", id, err)
	default:
		return reservations.FromSnapshot(snapshot), stored, nil
	}
}
