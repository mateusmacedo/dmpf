package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Reserve is a UPR: it decides over a copy and commits only on Accepted, so a
// rejection leaves the reservation untouched (DEC-10) and carries no event (DEC-11).
func (r *Reservation) Reserve(cmd Reserve) (kernel.Accepted[ReservedResponse], *kernel.Rejection) {
	next := r.clone()
	if cmd.Items <= 0 {
		return kernel.Accepted[ReservedResponse]{}, kernel.Reject(CodeNothingToReserve, "nothing to reserve")
	}
	switch next.status {
	case Confirmed:
		return kernel.Accepted[ReservedResponse]{}, kernel.Reject(CodeAlreadyReserved, "reservation is already confirmed")
	case Canceled:
		return kernel.Accepted[ReservedResponse]{}, kernel.Reject(CodeReservationCanceled, "reservation is canceled")
	}
	next.status = Confirmed
	next.items = cmd.Items
	*r = next
	return kernel.Accept(
		ReservedResponse{Order: r.order, Items: r.items},
		ReservationConfirmed{Order: r.order, Items: r.items, At: cmd.At},
	), nil
}
