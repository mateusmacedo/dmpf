package bookingsapplication

import (
	"context"
	"errors"
	"fmt"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func (s Service) RegisterResource(ctx context.Context, cmd Register) (dmpfapplication.Outcome[bookingsdomain.RegisteredResponse], error) {
	var zero dmpfapplication.Outcome[bookingsdomain.RegisteredResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationRegister)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		r, stored, err := loadOrCreateResource(ctx, res, cmd.Code)
		if err != nil {
			return err
		}

		accepted, rejection := r.Register(bookingsdomain.RegisterResource{
			Code: cmd.Code,
			At:   bookingsdomain.Instant(identity.OccurredAt),
		})
		if rejection != nil {
			outcome = dmpfapplication.Rejected[bookingsdomain.RegisteredResponse](rejection)
			return nil
		}
		if err := res.Resources.Save(ctx, cmd.Code, r.Snapshot(), stored); err != nil {
			return fmt.Errorf("bookingsapplication: register %s: %w", cmd.Code, err)
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

func loadOrCreateResource(ctx context.Context, res Resources, code bookingsdomain.ResourceCode) (*bookingsdomain.Resource, dmpfports.Version, error) {
	snapshot, stored, err := res.Resources.Load(ctx, code)
	if errors.Is(err, dmpfports.ErrNotFound) {
		return bookingsdomain.NewResource(code), 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("bookingsapplication: register %s: %w", code, err)
	}
	return bookingsdomain.FromResourceSnapshot(snapshot), stored, nil
}
