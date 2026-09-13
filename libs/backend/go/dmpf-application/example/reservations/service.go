// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, BLK-04, UOW-03/04), dentro do limite de 3 linhas.

package reservationsapp

import (
	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const (
	// AggregateType is the business origin of the fact (BLK-04).
	AggregateType = "reservations.Reservation"

	// Destination names the integration flow logically: never a topic, queue
	// or broker address, which are the provider's and the relay's choice (BLK-04).
	Destination = "reservations.events"
)

// maxEventsPerCommand is what this use case declares to ResolveIdentity: each
// of its commands produces at most one event, so no identifier is minted unused.
const maxEventsPerCommand = 1

// Resources is the resource set this consumer declares, bound to the open
// transaction by the composition root (UOW-03, UOW-04).
type Resources struct {
	Inbox        dmpfports.Inbox
	Reservations dmpfports.Repository[reservations.OrderID, reservations.Snapshot]
	Outbox       dmpfports.Outbox
}

// Command is the closed union of this service's commands, so one AuthorizeFunc
// covers every entry point. The marker is unexported: no other package widens it.
type Command interface{ isCommand() }

// ConsumeOrderPlaced asks the reservation to confirm Items for Order, in
// response to one delivered message the consumer adapter has already decoded.
type ConsumeOrderPlaced struct {
	MessageID   dmpfports.MessageID
	MessageType string
	PayloadHash string
	ReceivedAt  dmpfports.Instant
	Order       reservations.OrderID
	Items       int
}

func (ConsumeOrderPlaced) isCommand() {}

// Service is the reference consumer of the kernel: it walks the seven steps
// of FND-04 §6.3 over the reservations aggregate, branching at step 5 on the
// inbox reception instead of loading unconditionally.
type Service struct {
	UoW dmpfports.UnitOfWork[Resources]

	Clock     dmpfports.Clock
	IDs       dmpfports.IDGenerator
	Authorize dmpfapplication.AuthorizeFunc[Command]
	Consumer  string
}
