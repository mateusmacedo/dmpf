package providerkit_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/memory"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb"
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
			return res.Outbox.Enqueue(ctx, entry(dmpfports.MessageID("m-"+strconv.Itoa(n))))
		},
		Kept:             func() int { return len(store.Entries()) },
		ArmCommitFailure: store.FailNextCommit,
		Commits:          store.Commits,
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
		Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} })
			return uow.Within(ctx, func(ctx context.Context, res inboxResources) error { return fn(ctx, res.Inbox) })
		},
		ReadStatus: store.InboxStatus,
		// memory serializes every transaction on one mutex, so the race clause
		// has nothing to observe here; Postgres runs it.
		Concurrent:       false,
		ConsumerMismatch: memory.ErrInboxConsumerMismatch,
		Rows:             store.InboxRows,
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

// racyInbox is the negative vector of INB-06: it checks the key and then
// inserts without holding anything in between, so two concurrent first
// receptions both see the first branch. The suite must name INB-06.
type racyInbox struct {
	mu        sync.Mutex
	committed map[string]dmpfports.Status
}

type racyPending struct {
	inbox *racyInbox
	key   string
	done  bool
}

func (p *racyPending) Complete(_ context.Context, c dmpfports.Completion) error {
	p.inbox.mu.Lock()
	defer p.inbox.mu.Unlock()
	p.inbox.committed[p.key] = c.Status
	p.done = true
	return nil
}

func (p *racyPending) Completed() bool { return p.done }

type racyBound struct {
	inbox    *racyInbox
	consumer string
}

func (b racyBound) Register(_ context.Context, r dmpfports.Receipt) (dmpfports.Reception, error) {
	if r.Consumer != b.consumer {
		return dmpfports.Reception{}, errors.New("racy: consumer mismatch")
	}
	key := b.consumer + "/" + string(r.MessageID)
	b.inbox.mu.Lock()
	status, present := b.inbox.committed[key]
	b.inbox.mu.Unlock()
	// The check is done and the lock released before the insert: this is the
	// window INB-06 forbids.
	if present {
		if status == dmpfports.StatusProcessed {
			return dmpfports.ProcessedReception(), nil
		}
		return dmpfports.RejectedReception(), nil
	}
	return dmpfports.FirstReception(&racyPending{inbox: b.inbox, key: key}), nil
}

func racyInboxSubject() providerkit.InboxSubject {
	in := &racyInbox{committed: map[string]dmpfports.Status{}}
	return providerkit.InboxSubject{
		Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
			return fn(ctx, racyBound{inbox: in, consumer: consumer})
		},
		ReadStatus: func(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool) {
			in.mu.Lock()
			defer in.mu.Unlock()
			s, ok := in.committed[consumer+"/"+string(id)]
			return s, ok
		},
		Concurrent: true,
	}
}

func TestACheckThenInsertInboxIsReprovedOnINB06(t *testing.T) {
	v := providerkit.Inbox(racyInboxSubject)
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "INB-06" && strings.Contains(d.Detail, "saw the first branch") {
			named = true
		}
	}
	if !named {
		t.Fatalf("a check-then-insert inbox was not reproved on INB-06: %v", v.Failures())
	}
}
