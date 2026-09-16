package domain

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

const (
	CodeQuantityOutOfRange domain.Code = "resource-scheduling/booking/quantity-out-of-range"
	CodeNotReserved        domain.Code = "resource-scheduling/booking/not-reserved"
	CodeCodeEmpty          domain.Code = "resource-scheduling/resource/code-empty"
)
