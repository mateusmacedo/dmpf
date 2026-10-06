package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (b *Booking) Cancel(cmd CancelBooking) (kernel.Accepted[CancelledResponse], *kernel.Rejection) {
	return kernel.DecideOver(b, (*Booking).clone, func(next *Booking) (kernel.Accepted[CancelledResponse], *kernel.Rejection) {
		if next.status != Reserved {
			return kernel.Refuse[CancelledResponse](CodeBookingNotReserved, "booking is not reserved")
		}
		next.status = Cancelled
		return kernel.Accept(
			CancelledResponse{BookingID: next.id},
			BookingCancelled{BookingID: next.id, At: cmd.At},
		), nil
	})
}
