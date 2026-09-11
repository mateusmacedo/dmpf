package bookingsdomain

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

func (b *Booking) Cancel(cmd CancelBooking) (dmpfdomain.Accepted[CancelledResponse], *dmpfdomain.Rejection) {
	next := b.clone()
	if next.status != BookingReservedStatus {
		return dmpfdomain.Accepted[CancelledResponse]{}, dmpfdomain.Reject(CodeNotReserved, "booking is not reserved")
	}
	next.status = BookingCancelled
	*b = next
	return dmpfdomain.Accept(
		CancelledResponse{BookingID: b.id},
		BookingCancelledEvent{BookingID: b.id, At: cmd.At},
	), nil
}
