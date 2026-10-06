package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type namedEvent string

func (e namedEvent) EventName() string { return string(e) }

type recordingOutbox struct {
	entries []ports.OutboxEntry
	failAt  int
	err     error
}

func (o *recordingOutbox) Enqueue(_ context.Context, entry ports.OutboxEntry) error {
	if o.err != nil && len(o.entries) == o.failAt {
		return o.err
	}
	o.entries = append(o.entries, entry)
	return nil
}

var origin = application.Origin{
	Destination:   "orders.events",
	AggregateType: "order",
	AggregateID:   "order-7",
}

func TestEnqueueAuthorsTheSevenFieldsOfEachEntry(t *testing.T) {
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{CorrelationID: "corr-1"})
	identity := application.Identity{OccurredAt: 42, MessageIDs: []ports.MessageID{"m-1", "m-2"}}
	events := []domain.DomainEvent{namedEvent("OrderPlaced"), namedEvent("ItemAdded")}
	outbox := &recordingOutbox{}

	if err := application.Enqueue(ctx, outbox, identity, origin, 3, events); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if len(outbox.entries) != len(events) {
		t.Fatalf("entries = %d, want one per event (%d)", len(outbox.entries), len(events))
	}
	for i, entry := range outbox.entries {
		want := ports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           ports.PublishIntent{Destination: origin.Destination, PartitionKey: origin.AggregateID},
			AggregateType:    origin.AggregateType,
			AggregateID:      origin.AggregateID,
			AggregateVersion: 3,
			Event:            events[i],
			Context:          ports.MessageContext{CorrelationID: "corr-1", CausationID: string(identity.MessageIDs[i])},
		}
		if entry != want {
			t.Fatalf("entry %d = %+v, want %+v", i, entry, want)
		}
	}
}

func TestEnqueueStopsAtTheFirstOutboxError(t *testing.T) {
	cause := errors.New("outbox unavailable")
	identity := application.Identity{OccurredAt: 1, MessageIDs: []ports.MessageID{"m-1", "m-2", "m-3"}}
	events := []domain.DomainEvent{namedEvent("A"), namedEvent("B"), namedEvent("C")}
	outbox := &recordingOutbox{failAt: 1, err: cause}

	err := application.Enqueue(context.Background(), outbox, identity, origin, 1, events)

	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want %v", err, cause)
	}
	if len(outbox.entries) != 1 {
		t.Fatalf("entries = %d, want 1: the events after the failure must not be enqueued", len(outbox.entries))
	}
}

func TestEnqueuePanicsWhenTheDecisionOutgrowsTheResolvedIdentifiers(t *testing.T) {
	identity := application.Identity{OccurredAt: 1, MessageIDs: []ports.MessageID{"m-1"}}
	events := []domain.DomainEvent{namedEvent("A"), namedEvent("B")}
	const want = "application: the decision produced 2 events but only 1 identifiers were resolved; raise maxEventsPerCommand"

	defer func() {
		if got := recover(); got != want {
			t.Fatalf("recovered %v, want %q", got, want)
		}
	}()
	_ = application.Enqueue(context.Background(), &recordingOutbox{}, identity, origin, 1, events)
}
