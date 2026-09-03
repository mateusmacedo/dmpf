// comment-discipline-ok-file: godoc de portas; a autoria dos campos e o que NÃO entra no tipo são normativos (FND-04 §2.3, BLK-01/03/04/05, ADR-021).

package dmpfports

import (
	"context"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
)

// PublishIntent is the routing the application service authors (BLK-04).
type PublishIntent struct {
	// Destination names the integration flow logically. It is never a topic,
	// queue, ARN or broker address: choosing the physical target is the
	// provider's, and the relay's, business (BLK-04).
	Destination string

	// PartitionKey preserves ordering among events of the same aggregate. It is
	// empty when there is no order to preserve.
	PartitionKey string
}

// OutboxEntry is what the application service hands to the outbox: the three
// authorship groups of FND-04 §2.3 — identity and time, routing, business
// origin — plus the event itself.
//
// Wire fields (message_type, schema_version, payload) belong to the provider,
// and drain state (status, available_at, attempt_count, locked_by,
// locked_until, published_at, last_error) belongs to the schema and to the
// relay. Neither group exists here, and adding one would move the boundary
// (BLK-04, BLK-05, ADR-021).
type OutboxEntry struct {
	MessageID        MessageID
	OccurredAt       Instant
	Intent           PublishIntent
	AggregateType    string
	AggregateID      string
	AggregateVersion Version
	Event            dmpfdomain.DomainEvent
}

// Outbox is the single sink of publish intent inside the transaction (UOW-08):
// this block declares no publishing port at all, because publishing happens
// after the commit and outside the unit of work.
type Outbox interface {
	// Enqueue takes a context only for cancellation and deadline (CTX-20,
	// CTX-21); it carries no execution context of its own.
	Enqueue(ctx context.Context, entry OutboxEntry) error
}
