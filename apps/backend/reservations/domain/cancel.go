package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Cancel is a UPR: the first decision on a pending reservation wins, so a
// confirmed or canceled reservation refuses it and stays untouched (DEC-10).
func (r *Reservation) Cancel(cmd Cancel) (kernel.Accepted[CancelledResponse], *kernel.Rejection) {
	return kernel.DecideOver(r, (*Reservation).clone, func(next *Reservation) (kernel.Accepted[CancelledResponse], *kernel.Rejection) {
		switch next.status {
		case Confirmed:
			return kernel.Refuse[CancelledResponse](CodeReservationAlreadyReserved, "reservation is already confirmed")
		case Cancelled:
			return kernel.Refuse[CancelledResponse](CodeReservationAlreadyCancelled, "reservation is already canceled")
		}
		next.status = Cancelled
		return kernel.Accept(
			CancelledResponse{Order: next.order},
			ReservationCancelled{Order: next.order, At: cmd.At},
		), nil
	})
}
