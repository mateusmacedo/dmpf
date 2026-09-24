package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	// AggregateType is the business origin of the fact (BLK-04).
	AggregateType = "orders.Order"

	// Destination names the integration flow logically: never a topic, queue or
	// broker address, which are the provider's and the relay's choice (BLK-04).
	Destination = "orders.events"

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
	Orders ports.Repository[domain.OrderID, domain.Snapshot]
	Outbox ports.Outbox
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

	Clock     ports.Clock
	IDs       ports.IDGenerator
	Authorize application.Authorize[Operation]
	ItemLimit int

	Instrumentation ports.Instrumentation
}

// instrumentation resolves the nil hook to the inert realization, so every
// operation calls BeginOperation unconditionally.
func (s Service) instrumentation() ports.Instrumentation {
	if s.Instrumentation == nil {
		return ports.NoInstrumentation()
	}
	return s.Instrumentation
}

// authorizationResult categorises a step 1 error. Only a declared denial is
// Denied; anything else is technical failure, because inferring a refusal from
// an unrelated error would report a false negative of access.
func authorizationResult(err error) ports.Result {
	if errors.Is(err, ports.ErrDenied) {
		return ports.Result{Outcome: ports.OutcomeDenied}
	}
	return ports.Result{Outcome: ports.OutcomeFailed, Err: err}
}

// outcomeCategory reads the terminal category off the outcome, which is the
// only place that knows which branch of the UPR was taken.
func outcomeCategory[R any](outcome application.Outcome[R]) ports.OutcomeCategory {
	if _, refused := outcome.Rejection(); refused {
		return ports.OutcomeRejected
	}
	return ports.OutcomeAccepted
}

// enqueueAll authors the seven fields the application service owns (FND-04
// §2.3, BLK-04, BLK-05) for each event the decision produced. written is the
// version Save persisted, so a consumer can order facts of the same aggregate.
func enqueueAll(
	ctx context.Context,
	outbox ports.Outbox,
	identity application.Identity,
	order domain.OrderID,
	written ports.Version,
	events []kernel.DomainEvent,
) error {
	// A identidade é resolvida antes da transação, com a contagem que o caso de
	// uso declara. Produzir mais eventos do que isso é defeito de programação, e
	// sem esta guarda ele apareceria como "index out of range" dentro da
	// transação, sem dizer a causa.
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"application: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}

	for i, event := range events {
		entry := ports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           ports.PublishIntent{Destination: Destination, PartitionKey: string(order)},
			AggregateType:    AggregateType,
			AggregateID:      string(order),
			AggregateVersion: written,
			Event:            event,
			Context:          application.MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
