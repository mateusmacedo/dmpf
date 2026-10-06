package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Origin is where the events of a decision come from and go to; AggregateID is
// also the partition key, so the facts of one aggregate keep their order.
type Origin struct {
	Destination   string
	AggregateType string
	AggregateID   string
}

// Enqueue authors the seven fields the application service owns (FND-04 §2.3)
// for each event, with written as the version Save persisted. More events than
// resolved identifiers is a programming defect and panics.
func Enqueue(
	ctx context.Context,
	outbox ports.Outbox,
	identity Identity,
	origin Origin,
	written ports.Version,
	events []domain.DomainEvent,
) error {
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"application: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}

	for i, event := range events {
		entry := ports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           ports.PublishIntent{Destination: origin.Destination, PartitionKey: origin.AggregateID},
			AggregateType:    origin.AggregateType,
			AggregateID:      origin.AggregateID,
			AggregateVersion: written,
			Event:            event,
			Context:          MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
