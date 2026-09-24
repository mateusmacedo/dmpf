package memory_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// KIT-04 over the in-memory realization: the suites testkit/providerkit
// exports replace the clauses this package used to duplicate by hand.

type outboxResources struct{ Outbox ports.Outbox }

func kitEntry(id ports.MessageID) ports.OutboxEntry {
	return ports.OutboxEntry{
		MessageID: id, OccurredAt: 1,
		Intent:        ports.PublishIntent{Destination: "orders.events", PartitionKey: "o-1"},
		AggregateType: "orders.Order", AggregateID: "o-1", AggregateVersion: 1,
	}
}

func TestUnitOfWorkConformsToTheKit(t *testing.T) {
	v := providerkit.UnitOfWork(func() providerkit.UnitOfWorkSubject[outboxResources] {
		store := memory.New()
		n := 0
		return providerkit.UnitOfWorkSubject[outboxResources]{
			UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) outboxResources { return outboxResources{Outbox: tx.Outbox()} }),
			Write: func(ctx context.Context, res outboxResources) error {
				n++
				return res.Outbox.Enqueue(ctx, kitEntry(ports.MessageID("m-"+strconv.Itoa(n))))
			},
			Kept:             func() int { return len(store.Entries()) },
			ArmCommitFailure: store.FailNextCommit,
			Commits:          store.Commits,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped: %v", v.Skipped)
	}
	evidence.RecordVerdict(t, "provider", "memory-unit-of-work", v)
}

type inboxResources struct{ Inbox ports.Inbox }

func TestInboxConformsToTheKit(t *testing.T) {
	v := providerkit.Inbox(func() providerkit.InboxSubject {
		store := memory.New()
		return providerkit.InboxSubject{
			Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox ports.Inbox) error) error {
				uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} })
				return uow.Within(ctx, func(ctx context.Context, res inboxResources) error { return fn(ctx, res.Inbox) })
			},
			ReadStatus: store.InboxStatus,
			// Store.txMu serializes every transaction (tx.go), so the two-insert
			// race has nothing to observe here; the Postgres realization runs it.
			Concurrent:       false,
			ConsumerMismatch: memory.ErrInboxConsumerMismatch,
			Rows:             store.InboxRows,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("skipped = %v, want exactly the concurrency clause", v.Skipped)
	}
	evidence.RecordVerdict(t, "provider", "memory-inbox", v)
}
