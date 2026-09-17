package domain

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (b *Booking) Cancel(cmd CancelBooking) (domain.Accepted[CancelledResponse], *domain.Rejection) {
	next := b.clone()
	if next.status != BookingReservedStatus {
		return domain.Accepted[CancelledResponse]{}, domain.Reject(CodeNotReserved, "booking is not reserved")
	}
	next.status = BookingCancelled
	*b = next
	return domain.Accept(
		CancelledResponse{BookingID: b.id},
		BookingCancelledEvent{BookingID: b.id, At: cmd.At},
	), nil
}
