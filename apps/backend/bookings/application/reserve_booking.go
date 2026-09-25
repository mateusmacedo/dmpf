package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func (s Service) ReserveBooking(ctx context.Context, cmd ReserveBooking) (usecase.Outcome[domain.ReservedResponse], error) {
	var zero usecase.Outcome[domain.ReservedResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationReserveBooking)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		b := domain.NewBooking(cmd.BookingID)
		accepted, rejection := b.Reserve(domain.ReserveBooking{
			ResourceID: cmd.ResourceID,
			Quantity:   cmd.Quantity,
			At:         domain.Instant(identity.OccurredAt),
		})
		if rejection != nil {
			outcome = usecase.Rejected[domain.ReservedResponse](rejection)
			return nil
		}
		if err := res.Bookings.Save(ctx, cmd.BookingID, b.Snapshot(), 0); err != nil {
			return fmt.Errorf("application: reserve %s: %w", cmd.BookingID, err)
		}
		written := ports.Version(1)
		if err := enqueueAll(ctx, res.Outbox, identity, AggregateTypeBooking, string(cmd.BookingID), written, accepted.Events()); err != nil {
			return fmt.Errorf("application: reserve %s: enqueue: %w", cmd.BookingID, err)
		}
		outcome = usecase.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	end(ports.Result{Outcome: outcomeCategory(outcome)})
	return outcome, nil
}
