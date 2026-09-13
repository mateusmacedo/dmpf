package bookingsdomain

import (
	"strconv"

	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

func (b *Booking) Reserve(cmd ReserveBooking) (dmpfdomain.Accepted[ReservedResponse], *dmpfdomain.Rejection) {
	next := b.clone()
	if cmd.Quantity < 1 || cmd.Quantity > 100 {
		return dmpfdomain.Accepted[ReservedResponse]{}, dmpfdomain.Reject(CodeQuantityOutOfRange, "quantity must be between 1 and 100",
			dmpfdomain.Detail{Key: "quantity", Value: strconv.Itoa(cmd.Quantity)},
		)
	}
	next.resourceID = cmd.ResourceID
	next.quantity = cmd.Quantity
	next.status = BookingReservedStatus
	next.reservedAt = cmd.At
	*b = next
	return dmpfdomain.Accept(
		ReservedResponse{BookingID: b.id},
		BookingReserved{BookingID: b.id, ResourceID: cmd.ResourceID, Quantity: cmd.Quantity, At: cmd.At},
	), nil
}
