package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// AddItem walks the nine steps of FND-04 §3.2.
func (s Service) AddItem(ctx context.Context, cmd AddItem) (usecase.Outcome[domain.ItemAccepted], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ItemAccepted]{
		Operation: OperationAddItem,
		Object:    string(cmd.Order),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationAddItem).
			String(string(cmd.Order)).String(string(cmd.SKU)).Int(int64(cmd.Quantity)),
		Codec: itemAcceptedCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ItemAccepted], error) {
			outcome, err := usecase.Decide(ctx, res.Orders, res.Outbox, origin(cmd.Order), cmd.Order, identity,
				usecase.OrNew(func(id domain.OrderID) *domain.Order { return domain.NewOrder(id, s.ItemLimit) }, domain.FromSnapshot),
				func(o *domain.Order) (kernel.Accepted[domain.ItemAccepted], *kernel.Rejection) {
					return o.AddItem(domain.AddItem{SKU: cmd.SKU, Quantity: cmd.Quantity, At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: add item to %s: %w", cmd.Order, err)
			}
			return outcome, nil
		},
	})
}
