package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// NewOrderReader serves the read side without the write side (UOW-11): it takes the
// pool because a query must not open a transaction.
func NewOrderReader(pool postgres.ReadPool) ports.Reader[domain.OrderID, domain.Snapshot] {
	return orderTable.Reader(pool)
}
