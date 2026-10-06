package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// PlaceOrder walks the same nine steps as AddItem, but only loads: an absent
// aggregate comes back as a wrapped technical error, not a rejection, because
// no UPR produced one and the edge category belongs to FND-07.
func (s Service) PlaceOrder(ctx context.Context, cmd PlaceOrder) (usecase.Outcome[domain.PlacedResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.PlacedResponse]{
		Operation:   OperationPlaceOrder,
		Object:      string(cmd.Order),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationPlaceOrder).String(string(cmd.Order)),
		Codec:       placedCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.PlacedResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Orders, res.Outbox, origin(cmd.Order), cmd.Order, identity,
				usecase.Existing[domain.OrderID](domain.FromSnapshot),
				func(o *domain.Order) (kernel.Accepted[domain.PlacedResponse], *kernel.Rejection) {
					return o.Place(domain.PlaceOrder{At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: place order %s: %w", cmd.Order, err)
			}
			return outcome, nil
		},
	})
}
