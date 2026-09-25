package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Cancel is a UPR: the first decision on a pending reservation wins, so a
// confirmed or canceled reservation refuses it and stays untouched (DEC-10).
func (r *Reservation) Cancel(cmd Cancel) (kernel.Accepted[CancelledResponse], *kernel.Rejection) {
	next := r.clone()
	switch next.status {
	case Confirmed:
		return kernel.Accepted[CancelledResponse]{}, kernel.Reject(CodeReservationAlreadyReserved, "reservation is already confirmed")
	case Cancelled:
		return kernel.Accepted[CancelledResponse]{}, kernel.Reject(CodeReservationAlreadyCancelled, "reservation is already canceled")
	}
	next.status = Cancelled
	*r = next
	return kernel.Accept(
		CancelledResponse{Order: r.order},
		ReservationCancelled{Order: r.order, At: cmd.At},
	), nil
}
