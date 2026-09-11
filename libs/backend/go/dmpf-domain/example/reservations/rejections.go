package reservations

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

// Rejection codes of the reservations example, in the "context/reason" form (FND-03 §5.3).
const (
	// CodeNothingToReserve: Reserve was asked to confirm zero or fewer items.
	CodeNothingToReserve dmpfdomain.Code = "reservations/nothing-to-reserve"
	// CodeAlreadyReserved: the reservation is no longer pending.
	CodeAlreadyReserved dmpfdomain.Code = "reservations/already-reserved"
)
