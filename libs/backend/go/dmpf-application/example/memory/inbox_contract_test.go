package memory_test

import (
	"context"
	"errors"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// The seven clauses below duplicate dmpfports.RunInboxContract on purpose
// (contract_test.go:11-13 explains why a shared kit is not importable here).
// Begin/Commit/Rollback become Within itself: this realization commits
// atomically around one callback, so a rollback is the callback's own error.

type inboxResources struct{ Inbox dmpfports.Inbox }

func bindInbox(consumer string) func(tx *memory.Tx) inboxResources {
	return func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} }
}

var errInboxContractRollback = errors.New("memory_test: inbox contract rollback")

func statusPtr(s dmpfports.Status) *dmpfports.Status { return &s }

// matchInboxBranch drives r.Match, completing the first branch's Pending with
// status when non-nil, and reports which of the four branches ran.
func matchInboxBranch(t *testing.T, r dmpfports.Reception, status *dmpfports.Status) string {
	t.Helper()
	var branch string
	err := r.Match(
		func(p dmpfports.Pending) error {
			branch = "first"
			if status == nil {
				return nil
			}
			return p.Complete(context.Background(), dmpfports.Completion{Status: *status})
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

// registerAndCommitInbox registers hash under consumer/id in its own
// transaction, completes a first reception as status, and commits.
func registerAndCommitInbox(t *testing.T, store *memory.Store, consumer string, id dmpfports.MessageID, hash string, status dmpfports.Status) {
	t.Helper()
	uow := memory.NewUnitOfWork(store, bindInbox(consumer))
	err := uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error {
		reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
			Consumer: consumer, MessageID: id, MessageType: "example", PayloadHash: hash,
		})
		if err != nil {
			return err
		}
		if branch := matchInboxBranch(t, reception, statusPtr(status)); branch != "first" {
			t.Fatalf("branch = %q, want %q for the first registration", branch, "first")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}
}

func TestMemoryInboxContract(t *testing.T) {
	t.Run("first reception is R1", func(t *testing.T) {
		store := memory.New()
		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		status, ok := store.InboxStatus("orders", "m-1")
		if !ok || status != dmpfports.StatusProcessed {
			t.Fatalf("InboxStatus() = (%v, %v), want (processed, true)", status, ok)
		}
	})

	t.Run("redelivery with the same hash after a processed commit is R2", func(t *testing.T) {
		store := memory.New()
		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		uow := memory.NewUnitOfWork(store, bindInbox("orders"))

		err := uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error {
			reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
				Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
			})
			if err != nil {
				return err
			}
			if branch := matchInboxBranch(t, reception, nil); branch != "processed" {
				t.Fatalf("branch = %q, want %q (INB-06)", branch, "processed")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
		if got := store.InboxRows(); got != 1 {
			t.Fatalf("InboxRows() = %d, want 1 — a redelivery writes nothing", got)
		}
	})

	t.Run("redelivery with the same hash after a rejected commit is R3", func(t *testing.T) {
		store := memory.New()
		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusRejected)
		uow := memory.NewUnitOfWork(store, bindInbox("orders"))

		err := uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error {
			reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
				Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
			})
			if err != nil {
				return err
			}
			if branch := matchInboxBranch(t, reception, nil); branch != "rejected" {
				t.Fatalf("branch = %q, want %q (INB-12)", branch, "rejected")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
	})

	t.Run("a divergent hash on a present key is R4", func(t *testing.T) {
		store := memory.New()
		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		uow := memory.NewUnitOfWork(store, bindInbox("orders"))

		err := uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error {
			reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
				Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h2",
			})
			if err != nil {
				return err
			}
			if branch := matchInboxBranch(t, reception, nil); branch != "collision" {
				t.Fatalf("branch = %q, want %q", branch, "collision")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
	})

	t.Run("first reception again after a rollback", func(t *testing.T) {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, bindInbox("orders"))

		err := uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error {
			reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
				Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
			})
			if err != nil {
				return err
			}
			matchInboxBranch(t, reception, statusPtr(dmpfports.StatusProcessed))
			return errInboxContractRollback
		})
		if !errors.Is(err, errInboxContractRollback) {
			t.Fatalf("Within() = %v, want errInboxContractRollback", err)
		}
		if _, ok := store.InboxStatus("orders", "m-1"); ok {
			t.Fatal("a rolled-back first reception must leave no committed row")
		}

		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		if _, ok := store.InboxStatus("orders", "m-1"); !ok {
			t.Fatal("the retry after rollback must commit — nothing was ever applied")
		}
	})

	t.Run("distinct consumers with the same MessageID do not dedupe", func(t *testing.T) {
		store := memory.New()
		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusProcessed)

		registerAndCommitInbox(t, store, "billing", "m-1", "h1", dmpfports.StatusProcessed)

		if _, ok := store.InboxStatus("billing", "m-1"); !ok {
			t.Fatal("a different consumer owns a disjoint key space")
		}
	})

	t.Run("registering a present key leaves the transaction usable", func(t *testing.T) {
		store := memory.New()
		registerAndCommitInbox(t, store, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		uow := memory.NewUnitOfWork(store, bindInbox("orders"))

		err := uow.Within(context.Background(), func(ctx context.Context, res inboxResources) error {
			reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
				Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
			})
			if err != nil {
				t.Fatalf("Register() = %v, want nil — a present key is a result, not a constraint error (INB-04)", err)
			}
			matchInboxBranch(t, reception, nil)

			reception, err = res.Inbox.Register(ctx, dmpfports.Receipt{
				Consumer: "orders", MessageID: "m-2", MessageType: "example", PayloadHash: "h2",
			})
			if err != nil {
				t.Fatalf("a subsequent Register() in the same transaction = %v, want nil", err)
			}
			if branch := matchInboxBranch(t, reception, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
				t.Fatalf("branch = %q, want %q", branch, "first")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
		if _, ok := store.InboxStatus("orders", "m-2"); !ok {
			t.Fatal("the second key must have committed alongside the first")
		}
	})
}
