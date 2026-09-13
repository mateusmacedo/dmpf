package bookingsapplication

import (
	"context"
	"fmt"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func (s Service) CancelBooking(ctx context.Context, cmd Cancel) (dmpfapplication.Outcome[bookingsdomain.CancelledResponse], error) {
	var zero dmpfapplication.Outcome[bookingsdomain.CancelledResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationCancel)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		snapshot, stored, err := res.Bookings.Load(ctx, cmd.BookingID)
		if err != nil {
			return fmt.Errorf("bookingsapplication: cancel %s: %w", cmd.BookingID, err)
		}

		b := bookingsdomain.FromBookingSnapshot(snapshot)
		accepted, rejection := b.Cancel(bookingsdomain.CancelBooking{
			At: bookingsdomain.Instant(identity.OccurredAt),
		})
		if rejection != nil {
			outcome = dmpfapplication.Rejected[bookingsdomain.CancelledResponse](rejection)
			return nil
		}
		if err := res.Bookings.Save(ctx, cmd.BookingID, b.Snapshot(), stored); err != nil {
			return fmt.Errorf("bookingsapplication: cancel %s: %w", cmd.BookingID, err)
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
