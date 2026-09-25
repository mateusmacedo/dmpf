package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// OrderID is the permanent natural key of the reservation (FND-04 §7.2, GAR-10).
type OrderID string

// Status is the lifecycle state of the reservation.
type Status int

const (
	// Pending accepts a Reserve or a Cancel command; the first one decided wins.
	Pending Status = iota
	// Confirmed is terminal for this example: no further reservation can be made.
	Confirmed
	// Cancelled is terminal: a canceled reservation takes no other decision.
	Cancelled
)

// Instant is a point in time in Unix seconds, resolved by the application
// service and carried as a value (RFC §9.3): the domain never consults a clock.
type Instant int64

// Reserve is the command that asks the reservation to confirm Items for the order.
type Reserve struct {
	Items int
	At    Instant
}

// Cancel is the command that asks a pending reservation to be canceled.
type Cancel struct {
	At Instant
}

// ReservationConfirmed is the domain event of a reservation confirmed with Items lines.
type ReservationConfirmed struct {
	Order OrderID
	Items int
	At    Instant
}

// EventName is stable and carries no version or transport (MSG-N02, MSG-N03).
func (ReservationConfirmed) EventName() string { return "reservations.reservation-confirmed" }

// ReservationCancelled is the domain event of a pending reservation canceled.
type ReservationCancelled struct {
	Order OrderID
	At    Instant
}

// EventName is stable and carries no version or transport (MSG-N02, MSG-N03).
func (ReservationCancelled) EventName() string { return "reservations.reservation-cancelled" }

// ReservedResponse is the response of Reserve: the order and its confirmed item count.
type ReservedResponse struct {
	Order OrderID
	Items int
}

// CancelledResponse is the response of Cancel: the order whose reservation was canceled.
type CancelledResponse struct {
	Order OrderID
}

var _ kernel.DomainEvent = ReservationConfirmed{}
var _ kernel.DomainEvent = ReservationCancelled{}
