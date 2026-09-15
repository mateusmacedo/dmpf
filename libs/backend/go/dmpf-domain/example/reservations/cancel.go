package reservations

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

// Cancel is a UPR: the first decision on a pending reservation wins, so a
// confirmed or canceled reservation refuses it and stays untouched (DEC-10).
func (r *Reservation) Cancel(cmd Cancel) (dmpfdomain.Accepted[CancelledResponse], *dmpfdomain.Rejection) {
	next := r.clone()
	switch next.status {
	case Confirmed:
		return dmpfdomain.Accepted[CancelledResponse]{}, dmpfdomain.Reject(CodeAlreadyReserved, "reservation is already confirmed")
	case Canceled:
		return dmpfdomain.Accepted[CancelledResponse]{}, dmpfdomain.Reject(CodeAlreadyCanceled, "reservation is already canceled")
	}
	next.status = Canceled
	*r = next
	return dmpfdomain.Accept(
		CancelledResponse{Order: r.order},
		ReservationCancelled{Order: r.order, At: cmd.At},
	), nil
}
