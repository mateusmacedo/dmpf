package bookingsapplication

import (
	"context"
	"fmt"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func (s Service) ReserveBooking(ctx context.Context, cmd Reserve) (dmpfapplication.Outcome[bookingsdomain.ReservedResponse], error) {
	var zero dmpfapplication.Outcome[bookingsdomain.ReservedResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationReserve)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		b := bookingsdomain.NewBooking(cmd.BookingID)
		accepted, rejection := b.Reserve(bookingsdomain.ReserveBooking{
			ResourceID: cmd.ResourceID,
			Quantity:   cmd.Quantity,
			At:         bookingsdomain.Instant(identity.OccurredAt),
		})
		if rejection != nil {
			outcome = dmpfapplication.Rejected[bookingsdomain.ReservedResponse](rejection)
			return nil
		}
		if err := res.Bookings.Save(ctx, cmd.BookingID, b.Snapshot(), 0); err != nil {
			return fmt.Errorf("bookingsapplication: reserve %s: %w", cmd.BookingID, err)
		}
		written := dmpfports.Version(1)
		if err := enqueueAll(ctx, res.Outbox, identity, AggregateTypeBooking, string(cmd.BookingID), written, accepted.Events()); err != nil {
			return fmt.Errorf("bookingsapplication: reserve %s: enqueue: %w", cmd.BookingID, err)
		}
		outcome = dmpfapplication.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := dmpfports.OutcomeAccepted
	if _, refused := outcome.Rejection(); refused {
		category = dmpfports.OutcomeRejected
	}
	end(dmpfports.Result{Outcome: category})
	return outcome, nil
}
