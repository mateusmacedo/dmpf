package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	// AggregateType is the business origin of the fact (BLK-04).
	AggregateType = "orders.Order"

	// Destination names the integration flow logically: never a topic, queue or
	// broker address, which are the provider's and the relay's choice (BLK-04).
	Destination = "orders.events"

	CommandConsumer = "orders.commands"

	// OperationAddItem, OperationPlaceOrder and OperationFindOrder name the
	// operations for BeginOperation and for the audit action. They are exported
	// so the composition root can declare which of them is a read (TRC-16).
	OperationAddItem    = "orders.AddItem"
	OperationPlaceOrder = "orders.PlaceOrder"
	OperationFindOrder  = "orders.FindOrder"
)

// maxEventsPerCommand is what this use case declares to ResolveIdentity: each
// of its commands produces at most one event, so no identifier is minted unused.
const maxEventsPerCommand = 1

// Resources is the resource set the use case declares, bound to the open
// transaction by the composition root (UOW-03, UOW-04).
type Resources struct {
	Orders   ports.Repository[domain.OrderID, domain.Snapshot]
	Outbox   ports.Outbox
	Commands ports.Inbox
}

// Operation is the closed union of this service's entry points, writes and
// queries alike, so one authorizer covers every one of them. The marker is
// unexported: no other package widens the union.
type Operation interface{ isOperation() }

// AddItem asks the order to take one more item.
type AddItem struct {
	Order    domain.OrderID
	SKU      domain.SKU
	Quantity int
}

func (AddItem) isOperation() {}

// PlaceOrder asks the order to be placed.
type PlaceOrder struct {
	Order domain.OrderID
}

func (PlaceOrder) isOperation() {}

// FindOrder asks for the current state of one order. A query is an entry point
// like any other: authenticating at the route does not stand for permission to
// read what the operation returns.
type FindOrder struct {
	Order domain.OrderID
}

func (FindOrder) isOperation() {}

// Service is the reference application service of the kernel: it walks the nine
// steps of FND-04 §3.2 over the orders aggregate. README.md maps each step to
// the code that performs it.
type Service struct {
	UoW    ports.UnitOfWork[Resources]
	Reader ports.Reader[domain.OrderID, domain.Snapshot]

	Clock       ports.Clock
	IDs         ports.IDGenerator
	Authorize   usecase.Authorize[Operation]
	ItemLimit   int
	Idempotency usecase.IdempotencyPolicy

	Instrumentation ports.Instrumentation
}

func idempotent[R any](
	ctx context.Context,
	s Service,
	res Resources,
	fingerprint *usecase.Fingerprint,
	operation string,
	now ports.Instant,
	codec usecase.OutcomeCodec[R],
	run func() (usecase.Outcome[R], error),
) (usecase.Outcome[R], bool, error) {
	return usecase.RunIdempotent(ctx, usecase.IdempotentCommand[R]{
		Inbox:       res.Commands,
		Consumer:    CommandConsumer,
		Operation:   operation,
		Fingerprint: fingerprint,
		Now:         now,
		Policy:      s.Idempotency,
		Codec:       codec,
		Run:         run,
	})
}

// instrumentation resolves the nil hook to the inert realization, so every
// operation calls BeginOperation unconditionally.
func (s Service) instrumentation() ports.Instrumentation {
	if s.Instrumentation == nil {
		return ports.NoInstrumentation()
	}
	return s.Instrumentation
}

func origin(order domain.OrderID) usecase.Origin {
	return usecase.Origin{Destination: Destination, AggregateType: AggregateType, AggregateID: string(order)}
}
