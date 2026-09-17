package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// FindOrder reads through Reader, never through UoW: a query neither opens a
// transaction nor writes the outbox (UOW-11).
func (s Service) FindOrder(ctx context.Context, id domain.OrderID) (domain.Snapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindOrder)

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("application: find order %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return domain.Snapshot{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshot, nil
}
