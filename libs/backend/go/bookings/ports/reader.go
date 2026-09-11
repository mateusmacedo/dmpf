package bookingsports

import (
	"context"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

// BookingsByResourceReader reads bookings by the resource they hold. The
// generic Reader of the kernel loads one aggregate by its own identity, so a
// query that traverses a relation has no expression there and belongs to the
// port block of the context (FND-04 §3.3).
type BookingsByResourceReader interface {
	LoadByResource(ctx context.Context, resourceID bookingsdomain.ResourceID) ([]bookingsdomain.BookingSnapshot, error)
}
