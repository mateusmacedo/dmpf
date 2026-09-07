package providerkit_test

import (
	"context"
	"errors"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/providerkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

// The in-memory realization of dmpf-application is the first candidate of the
// suites: it needs no infrastructure, and it is what serviceskit builds on.

type outboxResources struct{ Outbox dmpfports.Outbox }

func entry(id dmpfports.MessageID) dmpfports.OutboxEntry {
	return dmpfports.OutboxEntry{MessageID: id, OccurredAt: 1, Intent: dmpfports.PublishIntent{Destination: "orders.events", PartitionKey: "o-1"}, AggregateType: "orders.Order", AggregateID: "o-1", AggregateVersion: 1}
}

func memoryUoW() providerkit.UnitOfWorkSubject[outboxResources] {
	store := memory.New()
	n := 0
	return providerkit.UnitOfWorkSubject[outboxResources]{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) outboxResources { return outboxResources{Outbox: tx.Outbox()} }),
		Write: func(ctx context.Context, res outboxResources) error {
			n++
			return res.Outbox.Enqueue(ctx, entry(dmpfports.MessageID("m-"+string(rune('0'+n)))))
		},
		Kept:             func() int { return len(store.Entries()) },
		ArmCommitFailure: store.FailNextCommit,
	}
}

func TestMemoryUnitOfWorkConforms(t *testing.T) {
	v := providerkit.UnitOfWork(memoryUoW)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("memory can inject a commit failure; nothing should be skipped: %v", v.Skipped)
	}
}

type inboxResources struct{ Inbox dmpfports.Inbox }

func memoryInbox() providerkit.InboxSubject {
	store := memory.New()
	return providerkit.InboxSubject{
		Within: func(consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} })
			return uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error { return fn(ctx, res.Inbox) })
		},
		ReadStatus: store.InboxStatus,
		// memory serializes every transaction on one mutex, so the race clause
		// has nothing to observe here; Postgres runs it.
		Concurrent: false,
	}
}

func TestMemoryInboxConforms(t *testing.T) {
	v := providerkit.Inbox(memoryInbox)
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("expected exactly the concurrency clause skipped, got %v", v.Skipped)
	}
}

// A unit of work that commits even when the callback fails: the suite must
// name UOW-06.
type lenientUoW struct {
	store *memory.Store
	inner dmpfports.UnitOfWork[outboxResources]
}

func (u lenientUoW) Within(ctx context.Context, fn func(context.Context, outboxResources) error) error {
	var cbErr error
	err := u.inner.Within(ctx, func(ctx context.Context, res outboxResources) error {
		cbErr = fn(ctx, res)
		return nil
	})
	return errors.Join(err, cbErr)
}

func TestALenientUnitOfWorkIsReproved(t *testing.T) {
	v := providerkit.UnitOfWork(func() providerkit.UnitOfWorkSubject[outboxResources] {
		s := memoryUoW()
		store := memory.New()
		s.UoW = lenientUoW{store: store, inner: memory.NewUnitOfWork(store, func(tx *memory.Tx) outboxResources { return outboxResources{Outbox: tx.Outbox()} })}
		s.Kept = func() int { return len(store.Entries()) }
		s.ArmCommitFailure = store.FailNextCommit
		return s
	})
	if v.OK() {
		t.Fatal("a unit of work that commits a failing callback passed")
	}
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "UOW-06" {
			named = true
		}
	}
	if !named {
		t.Fatalf("UOW-06 not named: %v", v.Failures())
	}
}
