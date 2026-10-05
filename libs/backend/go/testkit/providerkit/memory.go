package providerkit

import (
	"context"
	"strconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type OutboxResources struct{ Outbox ports.Outbox }

type inboxResources struct{ Inbox ports.Inbox }

func MemoryUnitOfWork() UnitOfWorkSubject[OutboxResources] {
	store := memory.New()
	n := 0
	return UnitOfWorkSubject[OutboxResources]{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) OutboxResources { return OutboxResources{Outbox: tx.Outbox()} }),
		Write: func(ctx context.Context, res OutboxResources) error {
			n++
			return res.Outbox.Enqueue(ctx, memoryEntry(ports.MessageID("m-"+strconv.Itoa(n))))
		},
		Kept:             func() int { return len(store.Entries()) },
		ArmCommitFailure: store.FailNextCommit,
		Commits:          store.Commits,
	}
}

// MemoryInbox is the inbox subject over the in-memory realization. Store.txMu
// serializes every transaction, so the two-insert race has nothing to observe
// and is left to the Postgres realization.
func MemoryInbox() InboxSubject {
	store := memory.New()
	return InboxSubject{
		Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox ports.Inbox) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} })
			return uow.Within(ctx, func(ctx context.Context, res inboxResources) error { return fn(ctx, res.Inbox) })
		},
		ReadStatus:       store.InboxStatus,
		Concurrent:       false,
		ConsumerMismatch: memory.ErrInboxConsumerMismatch,
		Rows:             store.InboxRows,
	}
}

func memoryEntry(id ports.MessageID) ports.OutboxEntry {
	return ports.OutboxEntry{
		MessageID: id, OccurredAt: 1,
		Intent:        ports.PublishIntent{Destination: "orders.events", PartitionKey: "o-1"},
		AggregateType: "orders.Order", AggregateID: "o-1", AggregateVersion: 1,
	}
}
