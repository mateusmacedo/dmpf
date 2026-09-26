package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// NewReservationReader serves the read side without the write side (UOW-11): it takes the
// pool because a query must not open a transaction.
func NewReservationReader(pool postgres.ReadPool) ports.Reader[domain.OrderID, domain.Snapshot] {
	return reservationTable.Reader(pool)
}
