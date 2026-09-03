package ordersapp

import (
	"context"
	"fmt"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
)

// FindOrder reads through Reader, never through UoW: a query neither opens a
// transaction nor writes the outbox (UOW-11).
func (s Service) FindOrder(ctx context.Context, id orders.OrderID) (orders.Snapshot, error) {
	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		return orders.Snapshot{}, fmt.Errorf("ordersapp: find order %s: %w", id, err)
	}
	return snapshot, nil
}
