package reservations

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Rejection codes of the reservations example, in the "context/reason" form (FND-03 §5.3).
const (
	// CodeNothingToReserve: Reserve was asked to confirm zero or fewer items.
	CodeNothingToReserve domain.Code = "reservations/nothing-to-reserve"
	// CodeAlreadyReserved: the reservation is no longer pending.
	CodeAlreadyReserved domain.Code = "reservations/already-reserved"
	// CodeAlreadyCanceled: the reservation was canceled before this command.
	CodeAlreadyCanceled domain.Code = "reservations/already-canceled"
	// CodeReservationCanceled: a canceled reservation takes no reservation.
	CodeReservationCanceled domain.Code = "reservations/reservation-canceled"
)
