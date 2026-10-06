package domain

import (
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (b *Booking) Reserve(cmd ReserveBooking) (kernel.Accepted[ReservedResponse], *kernel.Rejection) {
	return kernel.DecideOver(b, (*Booking).clone, func(next *Booking) (kernel.Accepted[ReservedResponse], *kernel.Rejection) {
		if cmd.Quantity < 1 || cmd.Quantity > 100 {
			return kernel.Refuse[ReservedResponse](CodeBookingQuantityOutOfRange, "quantity must be between 1 and 100",
				kernel.Detail{Key: "quantity", Value: strconv.Itoa(cmd.Quantity)},
			)
		}
		next.resourceID = cmd.ResourceID
		next.quantity = cmd.Quantity
		next.status = Reserved
		next.reservedAt = cmd.At
		return kernel.Accept(
			ReservedResponse{BookingID: next.id},
			BookingReserved{BookingID: next.id, ResourceID: cmd.ResourceID, Quantity: cmd.Quantity, At: cmd.At},
		), nil
	})
}
