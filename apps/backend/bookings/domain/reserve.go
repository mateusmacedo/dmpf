package domain

import (
	"strconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (b *Booking) Reserve(cmd ReserveBooking) (domain.Accepted[ReservedResponse], *domain.Rejection) {
	next := b.clone()
	if cmd.Quantity < 1 || cmd.Quantity > 100 {
		return domain.Accepted[ReservedResponse]{}, domain.Reject(CodeBookingQuantityOutOfRange, "quantity must be between 1 and 100",
			domain.Detail{Key: "quantity", Value: strconv.Itoa(cmd.Quantity)},
		)
	}
	next.resourceID = cmd.ResourceID
	next.quantity = cmd.Quantity
	next.status = Reserved
	next.reservedAt = cmd.At
	*b = next
	return domain.Accept(
		ReservedResponse{BookingID: b.id},
		BookingReserved{BookingID: b.id, ResourceID: cmd.ResourceID, Quantity: cmd.Quantity, At: cmd.At},
	), nil
}
