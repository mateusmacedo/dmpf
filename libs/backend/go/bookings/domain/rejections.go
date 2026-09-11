package bookingsdomain

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

const (
	CodeQuantityOutOfRange dmpfdomain.Code = "resource-scheduling/booking/quantity-out-of-range"
	CodeNotReserved        dmpfdomain.Code = "resource-scheduling/booking/not-reserved"
	CodeCodeEmpty          dmpfdomain.Code = "resource-scheduling/resource/code-empty"
)
