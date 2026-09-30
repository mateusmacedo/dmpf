package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Rejection codes of the reservations example, in the "context/reason" form (FND-03 §5.3).
const (
	// CodeReservationNothingToReserve: Reserve was asked to confirm zero or fewer items.
	CodeReservationNothingToReserve kernel.Code = "reservations/nothing-to-reserve"
	// CodeReservationAlreadyReserved: the reservation is no longer pending.
	CodeReservationAlreadyReserved kernel.Code = "reservations/already-reserved"
	// CodeReservationAlreadyCancelled: the reservation was canceled before this command.
	CodeReservationAlreadyCancelled kernel.Code = "reservations/already-canceled"
	// CodeReservationCancelled: a canceled reservation takes no reservation.
	CodeReservationCancelled kernel.Code = "reservations/reservation-canceled"
)
