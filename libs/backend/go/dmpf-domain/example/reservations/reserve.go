package reservations

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

// Reserve is a UPR: it decides over a copy and commits only on Accepted, so a
// rejection leaves the reservation untouched (DEC-10) and carries no event (DEC-11).
func (r *Reservation) Reserve(cmd Reserve) (dmpfdomain.Accepted[ReservedResponse], *dmpfdomain.Rejection) {
	next := r.clone()
	if cmd.Items <= 0 {
		return dmpfdomain.Accepted[ReservedResponse]{}, dmpfdomain.Reject(CodeNothingToReserve, "nothing to reserve")
	}
	switch next.status {
	case Confirmed:
		return dmpfdomain.Accepted[ReservedResponse]{}, dmpfdomain.Reject(CodeAlreadyReserved, "reservation is already confirmed")
	case Canceled:
		return dmpfdomain.Accepted[ReservedResponse]{}, dmpfdomain.Reject(CodeReservationCanceled, "reservation is canceled")
	}
	next.status = Confirmed
	next.items = cmd.Items
	*r = next
	return dmpfdomain.Accept(
		ReservedResponse{Order: r.order, Items: r.items},
		ReservationConfirmed{Order: r.order, Items: r.items, At: cmd.At},
	), nil
}
