package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Authorization is step 1 of this context: each operation declares the
// permission it requires here, next to its definition (IDN-16).
func Authorization() usecase.Authorize[application.Operation] {
	return usecase.Permitted(permissionOf)
}

// An operation absent from the switch declares none, which Permitted denies
// (IDN-17): adding an operation without a line here closes it, never opens it.
func permissionOf(op application.Operation) ports.Permission {
	switch op.(type) {
	case application.ConsumeOrderPlaced:
		return "reservations:consume"
	case application.Reserve:
		return "reservations:write"
	case application.Cancel:
		return "reservations:write"
	case application.FindReservation:
		return "reservations:read"
	default:
		return ""
	}
}
