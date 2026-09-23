package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// NewBookingReader serves the read side without the write side (UOW-11): it
// takes the pool because a query must not open a transaction.
func NewBookingReader(pool postgres.ReadPool) ports.Reader[domain.BookingID, domain.BookingSnapshot] {
	return bookingTable.Reader(pool)
}

// bookingsByResource is the one query the generic ports do not express: many
// rows filtered by a column. The statement is the kernel's, so the tenant
// predicate is not this package's to remember (IDN-14).
var bookingsByResource = bookingTable.Relation("resource_id")

type BookingsByResourceReader struct{ pool postgres.ReadPool }

func NewBookingsByResourceReader(pool postgres.ReadPool) *BookingsByResourceReader {
	return &BookingsByResourceReader{pool: pool}
}

func (r *BookingsByResourceReader) LoadByResource(ctx context.Context, resourceID domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return bookingsByResource.Query(ctx, r.pool, string(resourceID))
}
