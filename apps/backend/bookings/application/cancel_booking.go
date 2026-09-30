package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func (s Service) CancelBooking(ctx context.Context, cmd CancelBooking) (usecase.Outcome[domain.CancelledResponse], error) {
	var zero usecase.Outcome[domain.CancelledResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationCancelBooking)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	fingerprint := usecase.NewFingerprint(OperationCancelBooking).String(string(cmd.BookingID))

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		var err error
		outcome, _, err = idempotent(ctx, s, res, fingerprint, OperationCancelBooking, identity.OccurredAt, cancelledCodec,
			func() (usecase.Outcome[domain.CancelledResponse], error) { return cancel(ctx, res, cmd, identity) })
		return err
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	end(ports.Result{Outcome: outcomeCategory(outcome)})
	return outcome, nil
}

func cancel(ctx context.Context, res Resources, cmd CancelBooking, identity usecase.Identity) (usecase.Outcome[domain.CancelledResponse], error) {
	var zero usecase.Outcome[domain.CancelledResponse]

	snapshot, stored, err := res.Bookings.Load(ctx, cmd.BookingID)
	if err != nil {
		return zero, fmt.Errorf("application: cancel %s: %w", cmd.BookingID, err)
	}

	b := domain.FromBookingSnapshot(snapshot)
	accepted, rejection := b.Cancel(domain.CancelBooking{
		At: domain.Instant(identity.OccurredAt),
	})
	if rejection != nil {
		return usecase.Rejected[domain.CancelledResponse](rejection), nil
	}
	if err := res.Bookings.Save(ctx, cmd.BookingID, b.Snapshot(), stored); err != nil {
		return zero, fmt.Errorf("application: cancel %s: %w", cmd.BookingID, err)
	}
	if err := enqueueAll(ctx, res.Outbox, identity, AggregateTypeBooking, string(cmd.BookingID), stored+1, accepted.Events()); err != nil {
		return zero, fmt.Errorf("application: cancel %s: enqueue: %w", cmd.BookingID, err)
	}
	return usecase.Accepted(accepted.Response()), nil
}
