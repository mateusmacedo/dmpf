package domain

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (b *Booking) Cancel(cmd CancelBooking) (domain.Accepted[CancelledResponse], *domain.Rejection) {
	next := b.clone()
	if next.status != Reserved {
		return domain.Accepted[CancelledResponse]{}, domain.Reject(CodeBookingNotReserved, "booking is not reserved")
	}
	next.status = Cancelled
	*b = next
	return domain.Accept(
		CancelledResponse{BookingID: b.id},
		BookingCancelled{BookingID: b.id, At: cmd.At},
	), nil
}
