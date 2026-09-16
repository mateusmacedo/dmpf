package reservations

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Cancel is a UPR: the first decision on a pending reservation wins, so a
// confirmed or canceled reservation refuses it and stays untouched (DEC-10).
func (r *Reservation) Cancel(cmd Cancel) (domain.Accepted[CancelledResponse], *domain.Rejection) {
	next := r.clone()
	switch next.status {
	case Confirmed:
		return domain.Accepted[CancelledResponse]{}, domain.Reject(CodeAlreadyReserved, "reservation is already confirmed")
	case Canceled:
		return domain.Accepted[CancelledResponse]{}, domain.Reject(CodeAlreadyCanceled, "reservation is already canceled")
	}
	next.status = Canceled
	*r = next
	return domain.Accept(
		CancelledResponse{Order: r.order},
		ReservationCancelled{Order: r.order, At: cmd.At},
	), nil
}
