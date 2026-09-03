package ordersapp

import (
	"context"

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
	for i, event := range events {
		entry := dmpfports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           dmpfports.PublishIntent{Destination: Destination, PartitionKey: string(order)},
			AggregateType:    AggregateType,
			AggregateID:      string(order),
			AggregateVersion: written,
			Event:            event,
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
