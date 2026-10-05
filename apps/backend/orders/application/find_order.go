package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

// FindOrder reads through Reader, never through UoW: a query neither opens a
// transaction nor writes the outbox (UOW-11). It still walks step 1 first,
// because authenticating at the route is not permission to read.
func (s Service) FindOrder(ctx context.Context, id domain.OrderID) (domain.Snapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindOrder, FindOrder{Order: id},
		func(ctx context.Context) (domain.Snapshot, error) {
			snapshot, _, err := s.Reader.Load(ctx, id)
			if err != nil {
				return domain.Snapshot{}, fmt.Errorf("application: find order %s: %w", id, err)
			}
			return snapshot, nil
		})
}
