package dmpfports_test

// The suite below lives in a _test.go file on purpose: a _test.go file is
// never importable, so each realization (memory, Postgres) duplicates it. A
// test kit exported as a package is KRN-11's.

import (
	"context"
	"errors"
	"testing"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// InboxSubject is what a realization gives the contract so it can drive
// Register inside a transaction lifecycle simple enough for an in-memory
// fake to honour, and observe the committed status afterward.
type InboxSubject struct {
	Inbox      func(consumer string) dmpfports.Inbox
	Begin      func()
	Commit     func()
	Rollback   func()
	ReadStatus func(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool)
}

// matchBranch drives r.Match, completing the first branch's Pending with
// completeAs when non-nil, and reports which of the four branches ran.
func matchBranch(t *testing.T, r dmpfports.Reception, completeAs *dmpfports.Status) string {
	t.Helper()
	var branch string
	err := r.Match(
		func(p dmpfports.Pending) error {
			branch = "first"
			if completeAs == nil {
				return nil
			}
			return p.Complete(context.Background(), dmpfports.Completion{Status: *completeAs})
		},
		func() error { branch = "processed"; return nil },
		func() error { branch = "rejected"; return nil },
		func() error { branch = "collision"; return nil },
	)
	if err != nil {
		t.Fatalf("Match() = %v, want nil", err)
	}
	return branch
}

func statusPtr(s dmpfports.Status) *dmpfports.Status { return &s }

// registerAndCommit registers hash under consumer/id in its own transaction,
// completes a first reception as status, and commits.
func registerAndCommit(t *testing.T, s InboxSubject, consumer string, id dmpfports.MessageID, hash string, status dmpfports.Status) {
	t.Helper()
	s.Begin()
	reception, err := s.Inbox(consumer).Register(context.Background(), dmpfports.Receipt{
		Consumer: consumer, MessageID: id, MessageType: "example", PayloadHash: hash,
	})
	if err != nil {
		t.Fatalf("Register() = %v, want nil", err)
	}
	if branch := matchBranch(t, reception, statusPtr(status)); branch != "first" {
		t.Fatalf("branch = %q, want %q for the first registration", branch, "first")
	}
	s.Commit()
}

// RunInboxContract exercises the properties of §6.2/§6.4 that every
// realization of Inbox must honour, from a fake in-memory transaction.
func RunInboxContract(t *testing.T, newSubject func() InboxSubject) {
	t.Helper()

	t.Run("first reception is R1", func(t *testing.T) {
		s := newSubject()
		registerAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		status, ok := s.ReadStatus("orders", "m-1")
		if !ok || status != dmpfports.StatusProcessed {
			t.Fatalf("ReadStatus() = (%v, %v), want (processed, true)", status, ok)
		}
	})

	t.Run("redelivery with the same hash after a processed commit is R2", func(t *testing.T) {
		s := newSubject()
		registerAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		s.Begin()
		reception, err := s.Inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil", err)
		}
		if branch := matchBranch(t, reception, nil); branch != "processed" {
			t.Fatalf("branch = %q, want %q (INB-06)", branch, "processed")
		}
		s.Commit()
	})

	t.Run("redelivery with the same hash after a rejected commit is R3", func(t *testing.T) {
		s := newSubject()
		registerAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusRejected)

		s.Begin()
		reception, err := s.Inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil", err)
		}
		if branch := matchBranch(t, reception, nil); branch != "rejected" {
			t.Fatalf("branch = %q, want %q (INB-12)", branch, "rejected")
		}
		s.Commit()
	})

	t.Run("a divergent hash on a present key is R4", func(t *testing.T) {
		s := newSubject()
		registerAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		s.Begin()
		reception, err := s.Inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h2",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil", err)
		}
		if branch := matchBranch(t, reception, nil); branch != "collision" {
			t.Fatalf("branch = %q, want %q", branch, "collision")
		}
		s.Commit()
	})

	t.Run("first reception again after a rollback", func(t *testing.T) {
		s := newSubject()

		s.Begin()
		reception, err := s.Inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil", err)
		}
		matchBranch(t, reception, statusPtr(dmpfports.StatusProcessed))
		s.Rollback()

		if _, ok := s.ReadStatus("orders", "m-1"); ok {
			t.Fatal("a rolled-back first reception must leave no committed row")
		}

		s.Begin()
		reception, err = s.Inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil", err)
		}
		if branch := matchBranch(t, reception, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q — nothing was ever applied", branch, "first")
		}
		s.Commit()
	})

	t.Run("distinct consumers with the same MessageID do not dedupe", func(t *testing.T) {
		s := newSubject()
		registerAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		s.Begin()
		reception, err := s.Inbox("billing").Register(context.Background(), dmpfports.Receipt{
			Consumer: "billing", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil", err)
		}
		if branch := matchBranch(t, reception, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q — a different consumer owns a disjoint key space", branch, "first")
		}
		s.Commit()
	})

	t.Run("a receipt naming another consumer is refused", func(t *testing.T) {
		s := newSubject()
		s.Begin()
		defer s.Rollback()
		_, err := s.Inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "billing", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err == nil {
			t.Fatal("Register() = nil, want an error — the bound consumer is the key's owner, the receipt cannot rename it")
		}
	})

	t.Run("registering a present key leaves the transaction usable", func(t *testing.T) {
		s := newSubject()
		registerAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		s.Begin()
		inbox := s.Inbox("orders")

		reception, err := inbox.Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil — a present key is a result, not a constraint error (INB-04)", err)
		}
		matchBranch(t, reception, nil)

		reception, err = inbox.Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-2", MessageType: "example", PayloadHash: "h2",
		})
		if err != nil {
			t.Fatalf("a subsequent Register() in the same transaction = %v, want nil", err)
		}
		if branch := matchBranch(t, reception, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		s.Commit()

		if _, ok := s.ReadStatus("orders", "m-2"); !ok {
			t.Fatal("the second key must have committed alongside the first")
		}
	})
}

// memoryInboxRowKey and memoryInboxRow are the fake's committed state: a row
// exists only once its writing transaction has committed (§6.1).
type memoryInboxRowKey struct {
	consumer string
	id       dmpfports.MessageID
}

type memoryInboxRow struct {
	hash   string
	status dmpfports.Status
}

// memoryInboxStore is the fake transaction lifecycle Begin/Commit/Rollback
// drive: pending writes are invisible until Commit copies them into committed.
type memoryInboxStore struct {
	committed map[memoryInboxRowKey]memoryInboxRow
	pending   map[memoryInboxRowKey]memoryInboxRow
}

func newMemoryInboxStore() *memoryInboxStore {
	return &memoryInboxStore{committed: map[memoryInboxRowKey]memoryInboxRow{}}
}

func (s *memoryInboxStore) begin() { s.pending = map[memoryInboxRowKey]memoryInboxRow{} }

func (s *memoryInboxStore) commit() {
	for k, v := range s.pending {
		s.committed[k] = v
	}
	s.pending = nil
}

func (s *memoryInboxStore) rollback() { s.pending = nil }

func (s *memoryInboxStore) readStatus(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool) {
	row, ok := s.committed[memoryInboxRowKey{consumer: consumer, id: id}]
	return row.status, ok
}

// memoryInbox is the fake Inbox, bound to one consumer and reading only
// committed state — never the pending writes of its own transaction.
type memoryInbox struct {
	store    *memoryInboxStore
	consumer string
}

func (i *memoryInbox) Register(_ context.Context, r dmpfports.Receipt) (dmpfports.Reception, error) {
	if r.Consumer != i.consumer {
		return dmpfports.Reception{}, errConsumerMismatch
	}
	key := memoryInboxRowKey{consumer: i.consumer, id: r.MessageID}
	if existing, ok := i.store.committed[key]; ok {
		if existing.hash != r.PayloadHash {
			return dmpfports.CollisionReception(), nil
		}
		if existing.status == dmpfports.StatusProcessed {
			return dmpfports.ProcessedReception(), nil
		}
		return dmpfports.RejectedReception(), nil
	}
	return dmpfports.FirstReception(&memoryPending{store: i.store, key: key, hash: r.PayloadHash}), nil
}

var errConsumerMismatch = errors.New("inbox_contract_test: receipt consumer does not match inbox consumer")

var _ dmpfports.Inbox = (*memoryInbox)(nil)

// memoryPending is the fake's write half: Complete stages the row in the open
// transaction's pending set, visible only once the store commits.
type memoryPending struct {
	store     *memoryInboxStore
	key       memoryInboxRowKey
	hash      string
	completed bool
}

func (p *memoryPending) Complete(_ context.Context, c dmpfports.Completion) error {
	p.store.pending[p.key] = memoryInboxRow{hash: p.hash, status: c.Status}
	p.completed = true
	return nil
}

func (p *memoryPending) Completed() bool { return p.completed }

var _ dmpfports.Pending = (*memoryPending)(nil)

func newMemoryInboxSubject() InboxSubject {
	store := newMemoryInboxStore()
	return InboxSubject{
		Inbox:      func(consumer string) dmpfports.Inbox { return &memoryInbox{store: store, consumer: consumer} },
		Begin:      store.begin,
		Commit:     store.commit,
		Rollback:   store.rollback,
		ReadStatus: store.readStatus,
	}
}

func TestInboxContract(t *testing.T) {
	RunInboxContract(t, newMemoryInboxSubject)
}
