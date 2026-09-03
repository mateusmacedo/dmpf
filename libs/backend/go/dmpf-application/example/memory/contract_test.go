package memory_test

import (
	"context"
	"errors"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
)

// The six clauses below duplicate dmpfports.RunUnitOfWorkContract on purpose:
// a _test.go file is never importable, so the suite that lives in dmpf-ports
// cannot be reused here. A test kit exported as a package is KRN-11's.

var (
	errContractCallbackFailed = errors.New("memory_test: contract callback failed")
	errContractCommitRefused  = errors.New("memory_test: contract commit refused")
)

func TestNewUnitOfWorkHonoursTheWithinContract(t *testing.T) {
	t.Run("commits every write of the single transaction it opens", func(t *testing.T) {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, bind)

		err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
			if err := res.Outbox.Enqueue(ctx, entry("m-000001")); err != nil {
				return err
			}
			return res.Outbox.Enqueue(ctx, entry("m-000002"))
		})

		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
		if got := len(store.Entries()); got != 2 {
			t.Fatalf("kept %d writes, want 2 (UOW-01, UOW-02)", got)
		}
		if got := store.WithinCalls(); got != 1 {
			t.Fatalf("WithinCalls() = %d, want 1", got)
		}
		if got := store.Commits(); got != 1 {
			t.Fatalf("Commits() = %d, want 1", got)
		}
	})

	t.Run("invokes the callback exactly once", func(t *testing.T) {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, bind)
		calls := 0

		if err := uow.Within(context.Background(), func(context.Context, resources) error {
			calls++
			return nil
		}); err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}

		if calls != 1 {
			t.Fatalf("callback ran %d times, want 1 (UOW-09)", calls)
		}
	})

	t.Run("returns ctx.Err() without invoking the callback", func(t *testing.T) {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, bind)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		invoked := false

		err := uow.Within(ctx, func(context.Context, resources) error {
			invoked = true
			return nil
		})

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Within() = %v, want context.Canceled (CTX-21)", err)
		}
		if invoked {
			t.Fatal("the callback ran on an already cancelled context")
		}
		if got := len(store.Entries()); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})

	t.Run("rolls back and returns the callback error unwrapped", func(t *testing.T) {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, bind)

		err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
			if err := res.Outbox.Enqueue(ctx, entry("m-000001")); err != nil {
				return err
			}
			return errContractCallbackFailed
		})

		if !errors.Is(err, errContractCallbackFailed) {
			t.Fatalf("Within() = %v, want the callback error", err)
		}
		if got := len(store.Entries()); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})

	t.Run("returns the commit error and keeps nothing", func(t *testing.T) {
		store := memory.New()
		store.FailNextCommit(errContractCommitRefused)
		uow := memory.NewUnitOfWork(store, bind)

		err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
			return res.Outbox.Enqueue(ctx, entry("m-000001"))
		})

		if !errors.Is(err, errContractCommitRefused) {
			t.Fatalf("Within() = %v, want the commit error as the provider produced it", err)
		}
		if got := len(store.Entries()); got != 0 {
			t.Fatalf("kept %d writes, want 0 — a failed commit persists nothing", got)
		}
		if got := store.Commits(); got != 0 {
			t.Fatalf("Commits() = %d, want 0 — the commit did not install state", got)
		}
	})

	t.Run("propagates a panic and discards the transaction", func(t *testing.T) {
		store := memory.New()
		uow := memory.NewUnitOfWork(store, bind)

		func() {
			defer func() {
				if recovered := recover(); recovered != "memory_test: contract boom" {
					t.Fatalf("recovered %v, want \"memory_test: contract boom\" (ERR-22)", recovered)
				}
			}()

			_ = uow.Within(context.Background(), func(ctx context.Context, res resources) error {
				if err := res.Outbox.Enqueue(ctx, entry("m-000001")); err != nil {
					return err
				}
				panic("memory_test: contract boom")
			})
		}()

		if got := len(store.Entries()); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})
}
