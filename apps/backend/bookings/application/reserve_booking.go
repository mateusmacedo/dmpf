package application

import (
	"context"
	"errors"
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
	fingerprint := usecase.NewFingerprint(OperationReserveBooking).
		String(string(cmd.BookingID)).String(string(cmd.ResourceID)).Int(int64(cmd.Quantity))

	outcome, replayed := zero, false
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		var err error
		outcome, replayed, err = idempotent(ctx, s, res, fingerprint, OperationReserveBooking, identity.OccurredAt, reservedCodec,
			func() (usecase.Outcome[domain.ReservedResponse], error) { return reserve(ctx, res, cmd, identity) })
		return err
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcomeCategory(outcome)
	end(ports.Result{Outcome: category})
	if !replayed {
		instrumentation.Audit(ctx, ports.AuditEvent{
			Object:  string(cmd.BookingID),
			Action:  OperationReserveBooking,
			Outcome: category,
			At:      identity.OccurredAt,
		})
	}
	return outcome, nil
}

func reserve(ctx context.Context, res Resources, cmd ReserveBooking, identity usecase.Identity) (usecase.Outcome[domain.ReservedResponse], error) {
	var zero usecase.Outcome[domain.ReservedResponse]

	switch _, _, err := res.Bookings.Load(ctx, cmd.BookingID); {
	case err == nil:
		return zero, fmt.Errorf("application: reserve %s: %w", cmd.BookingID, ports.ErrAlreadyExists)
	case !errors.Is(err, ports.ErrNotFound):
		return zero, fmt.Errorf("application: reserve %s: %w", cmd.BookingID, err)
	}

	b := domain.NewBooking(cmd.BookingID)
	accepted, rejection := b.Reserve(domain.ReserveBooking{
		ResourceID: cmd.ResourceID,
		Quantity:   cmd.Quantity,
		At:         domain.Instant(identity.OccurredAt),
	})
	if rejection != nil {
		return usecase.Rejected[domain.ReservedResponse](rejection), nil
	}
	if err := res.Bookings.Save(ctx, cmd.BookingID, b.Snapshot(), 0); err != nil {
		return zero, fmt.Errorf("application: reserve %s: %w", cmd.BookingID, err)
	}
	written := ports.Version(1)
	if err := enqueueAll(ctx, res.Outbox, identity, AggregateTypeBooking, string(cmd.BookingID), written, accepted.Events()); err != nil {
		return zero, fmt.Errorf("application: reserve %s: enqueue: %w", cmd.BookingID, err)
	}
	return usecase.Accepted(accepted.Response()), nil
}
