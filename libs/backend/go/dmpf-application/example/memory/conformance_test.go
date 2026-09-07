package memory_test

import (
	"context"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/providerkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

// KIT-04 over the in-memory realization: the suites dmpf-testkit/providerkit
// exports replace the clauses this package used to duplicate by hand.

type outboxResources struct{ Outbox dmpfports.Outbox }

func kitEntry(id dmpfports.MessageID) dmpfports.OutboxEntry {
	return dmpfports.OutboxEntry{
		MessageID: id, OccurredAt: 1,
		Intent:        dmpfports.PublishIntent{Destination: "orders.events", PartitionKey: "o-1"},
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
				return res.Outbox.Enqueue(ctx, kitEntry(dmpfports.MessageID("m-"+string(rune('0'+n)))))
			},
			Kept:             func() int { return len(store.Entries()) },
			ArmCommitFailure: store.FailNextCommit,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped: %v", v.Skipped)
	}
}

type inboxResources struct{ Inbox dmpfports.Inbox }

func TestInboxConformsToTheKit(t *testing.T) {
	v := providerkit.Inbox(func() providerkit.InboxSubject {
		store := memory.New()
		return providerkit.InboxSubject{
			Within: func(consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
				uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} })
				return uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error { return fn(ctx, res.Inbox) })
			},
			ReadStatus: store.InboxStatus,
			// Store.txMu serializes every transaction (tx.go), so the two-insert
			// race has nothing to observe here; the Postgres realization runs it.
			Concurrent: false,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("skipped = %v, want exactly the concurrency clause", v.Skipped)
	}
}
