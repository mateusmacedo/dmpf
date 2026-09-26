package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type BookingID string

type ResourceID string

type ResourceCode string

type Instant int64

type BookingStatus int

const (
	New BookingStatus = iota
	Reserved
	Cancelled
)

type ResourceRegisteredAt = Instant

type ReserveBooking struct {
	ResourceID ResourceID
	Quantity   int
	At         Instant
}

type CancelBooking struct {
	At Instant
}

type RegisterResource struct {
	Code ResourceCode
	At   Instant
}

type ReservedResponse struct {
	BookingID BookingID
}

type CancelledResponse struct {
	BookingID BookingID
}

type RegisteredResponse struct {
	Code ResourceCode
}

type BookingReserved struct {
	BookingID  BookingID
	ResourceID ResourceID
	Quantity   int
	At         Instant
}

func (BookingReserved) EventName() string { return "bookings.booking-reserved" }

type BookingCancelled struct {
	BookingID BookingID
	At        Instant
}

func (BookingCancelled) EventName() string { return "bookings.booking-cancelled" }

type ResourceRegistered struct {
	Code ResourceCode
	At   Instant
}

func (ResourceRegistered) EventName() string { return "bookings.resource-registered" }

var (
	_ kernel.DomainEvent = BookingReserved{}
	_ kernel.DomainEvent = BookingCancelled{}
	_ kernel.DomainEvent = ResourceRegistered{}
)
