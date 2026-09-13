package bookingsdomain

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

type BookingID string

type ResourceID string

type ResourceCode string

type Instant int64

type BookingStatus int

const (
	BookingNew BookingStatus = iota
	BookingReservedStatus
	BookingCancelled
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

type BookingCancelledEvent struct {
	BookingID BookingID
	At        Instant
}

func (BookingCancelledEvent) EventName() string { return "bookings.booking-cancelled" }

type ResourceRegistered struct {
	Code ResourceCode
	At   Instant
}

func (ResourceRegistered) EventName() string { return "bookings.resource-registered" }

var (
	_ dmpfdomain.DomainEvent = BookingReserved{}
	_ dmpfdomain.DomainEvent = BookingCancelledEvent{}
	_ dmpfdomain.DomainEvent = ResourceRegistered{}
)
