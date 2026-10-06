package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (s Service) RegisterResource(ctx context.Context, cmd RegisterResource) (usecase.Outcome[domain.RegisteredResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RegisteredResponse]{
		Operation:   OperationRegisterResource,
		Object:      string(cmd.Code),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationRegisterResource).String(string(cmd.Code)),
		Codec:       registeredCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisteredResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Resources, res.Outbox, origin(AggregateTypeResource, string(cmd.Code)), cmd.Code, identity,
				loadResource,
				func(r *domain.Resource) (kernel.Accepted[domain.RegisteredResponse], *kernel.Rejection) {
					return r.Register(domain.RegisterResource{Code: cmd.Code, At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: register %s: %w", cmd.Code, err)
			}
			return outcome, nil
		},
	})
}
