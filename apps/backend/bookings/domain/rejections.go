package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

const (
	CodeBookingQuantityOutOfRange kernel.Code = "resource-scheduling/booking/quantity-out-of-range"
	CodeBookingNotReserved        kernel.Code = "resource-scheduling/booking/not-reserved"
	CodeResourceCodeEmpty         kernel.Code = "resource-scheduling/resource/code-empty"
	CodeResourceNameEmpty         kernel.Code = "resource-scheduling/resource/name-empty"
	CodeResourceDuplicateName     kernel.Code = "resource-scheduling/resource/duplicate-name"
)

var RejectResourceDuplicateName = kernel.Reject(CodeResourceDuplicateName, "a resource with this name already exists in the tenant")
