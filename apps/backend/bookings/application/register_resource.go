package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func (s Service) RegisterResource(ctx context.Context, cmd RegisterResource) (usecase.Outcome[domain.RegisteredResponse], error) {
	var zero usecase.Outcome[domain.RegisteredResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationRegisterResource)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	fingerprint := usecase.NewFingerprint(OperationRegisterResource).String(string(cmd.Code))

	outcome, replayed := zero, false
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		var err error
		outcome, replayed, err = idempotent(ctx, s, res, fingerprint, OperationRegisterResource, identity.OccurredAt, registeredCodec,
			func() (usecase.Outcome[domain.RegisteredResponse], error) { return register(ctx, res, cmd, identity) })
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
			Object:  string(cmd.Code),
			Action:  OperationRegisterResource,
			Outcome: category,
			At:      identity.OccurredAt,
		})
	}
	return outcome, nil
}

func register(ctx context.Context, res Resources, cmd RegisterResource, identity usecase.Identity) (usecase.Outcome[domain.RegisteredResponse], error) {
	var zero usecase.Outcome[domain.RegisteredResponse]

	r, stored, err := loadOrCreateResource(ctx, res, cmd.Code)
	if err != nil {
		return zero, err
	}

	accepted, rejection := r.Register(domain.RegisterResource{
		Code: cmd.Code,
		At:   domain.Instant(identity.OccurredAt),
	})
	if rejection != nil {
		return usecase.Rejected[domain.RegisteredResponse](rejection), nil
	}
	if err := res.Resources.Save(ctx, cmd.Code, r.Snapshot(), stored); err != nil {
		return zero, fmt.Errorf("application: register %s: %w", cmd.Code, err)
	}
	if err := enqueueAll(ctx, res.Outbox, identity, AggregateTypeResource, string(cmd.Code), stored+1, accepted.Events()); err != nil {
		return zero, fmt.Errorf("application: register %s: enqueue: %w", cmd.Code, err)
	}
	return usecase.Accepted(accepted.Response()), nil
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
