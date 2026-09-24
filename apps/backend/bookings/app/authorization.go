package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
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
	case application.Reserve:
		return "bookings:write"
	case application.Cancel:
		return "bookings:write"
	case application.Register:
		return "bookings:write"
	case application.FindBooking:
		return "bookings:read"
	case application.FindBookingByResource:
		return "bookings:read"
	default:
		return ""
	}
}
