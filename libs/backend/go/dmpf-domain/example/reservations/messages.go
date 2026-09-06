package reservations

import dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"

// OrderID is the permanent natural key of the reservation (FND-04 §7.2, GAR-10).
type OrderID string

// Status is the lifecycle state of the reservation.
type Status int

const (
	// Pending accepts a Reserve command.
	Pending Status = iota
	// Confirmed is terminal for this example: no further reservation can be made.
	Confirmed
)

// Instant is a point in time in Unix seconds, resolved by the application
// service and carried as a value (RFC §9.3): the domain never consults a clock.
type Instant int64

// Reserve is the command that asks the reservation to confirm Items for the order.
type Reserve struct {
	Items int
	At    Instant
}

// ReservationConfirmed is the domain event of a reservation confirmed with Items lines.
type ReservationConfirmed struct {
	Order OrderID
	Items int
	At    Instant
}

// EventName is stable and carries no version or transport (MSG-N02, MSG-N03).
func (ReservationConfirmed) EventName() string { return "reservations.reservation-confirmed" }

// ReservedResponse is the response of Reserve: the order and its confirmed item count.
type ReservedResponse struct {
	Order OrderID
	Items int
}

var _ dmpfdomain.DomainEvent = ReservationConfirmed{}
