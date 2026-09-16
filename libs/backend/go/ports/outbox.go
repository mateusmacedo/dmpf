// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §2.3, BLK-01/04/05, ADR-021), dentro do limite de 3 linhas.

package ports

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// PublishIntent is the routing the application service authors (BLK-04).
type PublishIntent struct {
	// Destination names the integration flow logically: never a topic, queue,
	// ARN or broker address, because choosing the physical target belongs to
	// the provider and to the relay (BLK-04).
	Destination string

	// PartitionKey preserves ordering among events of the same aggregate, and
	// is empty when there is no order to preserve.
	PartitionKey string
}

// OutboxEntry is what the application service authors and hands to the outbox:
// the three groups of FND-04 §2.3 plus the event. Wire fields and drain state
// are absent by design — they belong to the provider and to the relay (BLK-05).
type OutboxEntry struct {
	MessageID        MessageID
	OccurredAt       Instant
	Intent           PublishIntent
	AggregateType    string
	AggregateID      string
	AggregateVersion Version
	Event            domain.DomainEvent
	Context          MessageContext
}

// Outbox is the single sink of publish intent inside the transaction (UOW-08):
// this block declares no publishing port, because publishing happens after the
// commit and outside the unit of work.
type Outbox interface {
	// Enqueue takes a context only for cancellation and deadline (CTX-20,
	// CTX-21); it carries no execution context of its own.
	Enqueue(ctx context.Context, entry OutboxEntry) error
}
