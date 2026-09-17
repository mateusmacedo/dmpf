package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Rejection codes of the reservations example, in the "context/reason" form (FND-03 §5.3).
const (
	// CodeNothingToReserve: Reserve was asked to confirm zero or fewer items.
	CodeNothingToReserve kernel.Code = "reservations/nothing-to-reserve"
	// CodeAlreadyReserved: the reservation is no longer pending.
	CodeAlreadyReserved kernel.Code = "reservations/already-reserved"
	// CodeAlreadyCanceled: the reservation was canceled before this command.
	CodeAlreadyCanceled kernel.Code = "reservations/already-canceled"
	// CodeReservationCanceled: a canceled reservation takes no reservation.
	CodeReservationCanceled kernel.Code = "reservations/reservation-canceled"
)
