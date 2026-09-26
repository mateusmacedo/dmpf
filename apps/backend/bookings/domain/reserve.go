package domain

import (
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (b *Booking) Reserve(cmd ReserveBooking) (kernel.Accepted[ReservedResponse], *kernel.Rejection) {
	next := b.clone()
	if cmd.Quantity < 1 || cmd.Quantity > 100 {
		return kernel.Accepted[ReservedResponse]{}, kernel.Reject(CodeBookingQuantityOutOfRange, "quantity must be between 1 and 100",
			kernel.Detail{Key: "quantity", Value: strconv.Itoa(cmd.Quantity)},
		)
	}
	next.resourceID = cmd.ResourceID
	next.quantity = cmd.Quantity
	next.status = Reserved
	next.reservedAt = cmd.At
	*b = next
	return kernel.Accept(
		ReservedResponse{BookingID: b.id},
		BookingReserved{BookingID: b.id, ResourceID: cmd.ResourceID, Quantity: cmd.Quantity, At: cmd.At},
	), nil
}
