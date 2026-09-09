package ordersapp

import (
	"context"
	"errors"
	"fmt"

	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
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
	Orders dmpfports.Repository[orders.OrderID, orders.Snapshot]
	Outbox dmpfports.Outbox
}

// Command is the closed union of this service's commands, so one AuthorizeFunc
// covers every entry point. The marker is unexported: no other package widens it.
type Command interface{ isCommand() }

// AddItem asks the order to take one more item.
type AddItem struct {
	Order    orders.OrderID
	SKU      orders.SKU
	Quantity int
}

func (AddItem) isCommand() {}

// PlaceOrder asks the order to be placed.
type PlaceOrder struct {
	Order orders.OrderID
}

func (PlaceOrder) isCommand() {}

// Service is the reference application service of the kernel: it walks the nine
// steps of FND-04 §3.2 over the orders aggregate. README.md maps each step to
// the code that performs it.
type Service struct {
	UoW    dmpfports.UnitOfWork[Resources]
	Reader dmpfports.Reader[orders.OrderID, orders.Snapshot]

	Clock     dmpfports.Clock
	IDs       dmpfports.IDGenerator
	Authorize dmpfapplication.AuthorizeFunc[Command]
	ItemLimit int

	Instrumentation dmpfports.Instrumentation
}

// instrumentation resolves the nil hook to the inert realization, so every
// operation calls BeginOperation unconditionally.
func (s Service) instrumentation() dmpfports.Instrumentation {
	if s.Instrumentation == nil {
		return dmpfports.NoInstrumentation()
	}
	return s.Instrumentation
}

// authorizationResult categorises a step 1 error. Only a declared denial is
// Denied; anything else is technical failure, because inferring a refusal from
// an unrelated error would report a false negative of access.
func authorizationResult(err error) dmpfports.Result {
	if errors.Is(err, dmpfports.ErrDenied) {
		return dmpfports.Result{Outcome: dmpfports.OutcomeDenied}
	}
	return dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: err}
}

// outcomeCategory reads the terminal category off the outcome, which is the
// only place that knows which branch of the UPR was taken.
func outcomeCategory[R any](outcome dmpfapplication.Outcome[R]) dmpfports.OutcomeCategory {
	if _, refused := outcome.Rejection(); refused {
		return dmpfports.OutcomeRejected
	}
	return dmpfports.OutcomeAccepted
}

// enqueueAll authors the seven fields the application service owns (FND-04
// §2.3, BLK-04, BLK-05) for each event the decision produced. written is the
// version Save persisted, so a consumer can order facts of the same aggregate.
func enqueueAll(
	ctx context.Context,
	outbox dmpfports.Outbox,
	identity dmpfapplication.Identity,
	order orders.OrderID,
	written dmpfports.Version,
	events []dmpfdomain.DomainEvent,
) error {
	// A identidade é resolvida antes da transação, com a contagem que o caso de
	// uso declara. Produzir mais eventos do que isso é defeito de programação, e
	// sem esta guarda ele apareceria como "index out of range" dentro da
	// transação, sem dizer a causa.
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"ordersapp: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}

	for i, event := range events {
		entry := dmpfports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           dmpfports.PublishIntent{Destination: Destination, PartitionKey: string(order)},
			AggregateType:    AggregateType,
			AggregateID:      string(order),
			AggregateVersion: written,
			Event:            event,
			Context:          dmpfapplication.MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
