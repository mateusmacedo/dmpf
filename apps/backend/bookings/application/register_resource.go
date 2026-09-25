package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

func (s Service) RegisterResource(ctx context.Context, cmd Register) (application.Outcome[domain.RegisteredResponse], error) {
	var zero application.Outcome[domain.RegisteredResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationRegister)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := application.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		r, stored, err := loadOrCreateResource(ctx, res, cmd.Code)
		if err != nil {
			return err
		}

		accepted, rejection := r.Register(domain.RegisterResource{
			Code: cmd.Code,
			At:   domain.Instant(identity.OccurredAt),
		})
		if rejection != nil {
			outcome = application.Rejected[domain.RegisteredResponse](rejection)
			return nil
		}
		if err := res.Resources.Save(ctx, cmd.Code, r.Snapshot(), stored); err != nil {
			return fmt.Errorf("application: register %s: %w", cmd.Code, err)
		}
		if err := enqueueAll(ctx, res.Outbox, identity, AggregateTypeResource, string(cmd.Code), stored+1, accepted.Events()); err != nil {
			return fmt.Errorf("application: register %s: enqueue: %w", cmd.Code, err)
		}
		outcome = application.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := ports.OutcomeAccepted
	if _, refused := outcome.Rejection(); refused {
		category = ports.OutcomeRejected
	}
	end(ports.Result{Outcome: category})
	return outcome, nil
}

func loadOrCreateResource(ctx context.Context, res Resources, code domain.ResourceCode) (*domain.Resource, ports.Version, error) {
	snapshot, stored, err := res.Resources.Load(ctx, code)
	if errors.Is(err, ports.ErrNotFound) {
		return domain.NewResource(code), 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("application: register %s: %w", code, err)
	}
	return domain.FromResourceSnapshot(snapshot), stored, nil
}
