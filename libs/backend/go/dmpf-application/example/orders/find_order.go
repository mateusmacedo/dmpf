package ordersapp

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// FindOrder reads through Reader, never through UoW: a query neither opens a
// transaction nor writes the outbox (UOW-11).
func (s Service) FindOrder(ctx context.Context, id orders.OrderID) (orders.Snapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindOrder)

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("ordersapp: find order %s: %w", id, err)
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: failed})
		return orders.Snapshot{}, failed
	}

	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
	return snapshot, nil
}
