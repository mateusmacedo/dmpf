package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Reserve is a UPR: it decides over a copy and commits only on Accepted, so a
// rejection leaves the reservation untouched (DEC-10) and carries no event (DEC-11).
func (r *Reservation) Reserve(cmd Reserve) (kernel.Accepted[ReservedResponse], *kernel.Rejection) {
	return kernel.DecideOver(r, (*Reservation).clone, func(next *Reservation) (kernel.Accepted[ReservedResponse], *kernel.Rejection) {
		if cmd.Items <= 0 {
			return kernel.Refuse[ReservedResponse](CodeReservationNothingToReserve, "nothing to reserve")
		}
		switch next.status {
		case Confirmed:
			return kernel.Refuse[ReservedResponse](CodeReservationAlreadyReserved, "reservation is already confirmed")
		case Cancelled:
			return kernel.Refuse[ReservedResponse](CodeReservationCancelled, "reservation is canceled")
		}
		next.status = Confirmed
		next.items = cmd.Items
		return kernel.Accept(
			ReservedResponse{Order: next.order, Items: next.items},
			ReservationConfirmed{Order: next.order, Items: next.items, At: cmd.At},
		), nil
	})
}
