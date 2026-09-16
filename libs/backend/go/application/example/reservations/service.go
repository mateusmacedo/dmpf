// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, BLK-04, UOW-03/04), dentro do limite de 3 linhas.

package reservationsapp

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	// AggregateType is the business origin of the fact (BLK-04).
	AggregateType = "reservations.Reservation"

	// Destination names the integration flow logically: never a topic, queue
	// or broker address, which are the provider's and the relay's choice (BLK-04).
	Destination = "reservations.events"

	// OperationReserve, OperationCancel and OperationFindReservation name the
	// synchronous operations for BeginOperation and for the audit action.
	OperationReserve         = "reservations.Reserve"
	OperationCancel          = "reservations.Cancel"
	OperationFindReservation = "reservations.FindReservation"
)

// maxEventsPerCommand is what this use case declares to ResolveIdentity: each
// of its commands produces at most one event, so no identifier is minted unused.
const maxEventsPerCommand = 1

// Resources is the resource set this consumer declares, bound to the open
// transaction by the composition root (UOW-03, UOW-04).
type Resources struct {
	Inbox        ports.Inbox
	Reservations ports.Repository[reservations.OrderID, reservations.Snapshot]
	Outbox       ports.Outbox
}

// Command is the closed union of this service's commands, so one AuthorizeFunc
// covers every entry point. The marker is unexported: no other package widens it.
type Command interface{ isCommand() }

// ConsumeOrderPlaced asks the reservation to confirm Items for Order, in
// response to one delivered message the consumer adapter has already decoded.
type ConsumeOrderPlaced struct {
	MessageID   ports.MessageID
	MessageType string
	PayloadHash string
	ReceivedAt  ports.Instant
	Order       reservations.OrderID
	Items       int
}

func (ConsumeOrderPlaced) isCommand() {}

// Reserve asks the reservation of Order to confirm Items outside any delivery.
type Reserve struct {
	Order reservations.OrderID
	Items int
}

func (Reserve) isCommand() {}

// Cancel asks the pending reservation of Order to be canceled.
type Cancel struct {
	Order reservations.OrderID
}

func (Cancel) isCommand() {}

// Service realizes the reservations use cases: Consume walks the seven steps of
// FND-04 §6.3 over a delivered message, and Reserve and Cancel walk the nine
// steps of §3.2 over a synchronous command; the first decision persisted wins.
type Service struct {
	UoW    ports.UnitOfWork[Resources]
	Reader ports.Reader[reservations.OrderID, reservations.Snapshot]

	Clock     ports.Clock
	IDs       ports.IDGenerator
	Authorize application.AuthorizeFunc[Command]
	Consumer  string

	Instrumentation ports.Instrumentation
}
