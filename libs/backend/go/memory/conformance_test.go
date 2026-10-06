package memory_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// KIT-04 over the in-memory realization: the suites testkit/providerkit
// exports replace the clauses this package used to duplicate by hand.

func TestUnitOfWorkConformsToTheKit(t *testing.T) {
	v := providerkit.UnitOfWork(providerkit.MemoryUnitOfWork)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped: %v", v.Skipped)
	}
}

func TestCommandInboxConformsToTheKit(t *testing.T) {
	v := providerkit.CommandInbox(func() providerkit.CommandInboxSubject {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Inbox { return tx.CommandInbox("kit.commands") })
		return providerkit.CommandInboxSubject{
			Within: uow.Within,
			// Store.txMu serializes every transaction, so a command never waits on
			// another; the Postgres realization runs the in-flight clause.
			Concurrent: false,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("skipped = %v, want exactly the in-flight clause", v.Skipped)
	}
}

func TestInboxConformsToTheKit(t *testing.T) {
	v := providerkit.Inbox(providerkit.MemoryInbox)
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("skipped = %v, want exactly the concurrency clause", v.Skipped)
	}
}
