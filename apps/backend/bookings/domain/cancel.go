package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (b *Booking) Cancel(cmd CancelBooking) (kernel.Accepted[CancelledResponse], *kernel.Rejection) {
	next := b.clone()
	if next.status != Reserved {
		return kernel.Accepted[CancelledResponse]{}, kernel.Reject(CodeBookingNotReserved, "booking is not reserved")
	}
	next.status = Cancelled
	*b = next
	return kernel.Accept(
		CancelledResponse{BookingID: b.id},
		BookingCancelled{BookingID: b.id, At: cmd.At},
	), nil
}
