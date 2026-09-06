//go:build integration

package dmpfpostgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

// The six clauses below are the ones dmpfports.UnitOfWork.Within enumerates,
// with the same names memory/contract_test.go gives them: two realizations of
// one contract have to be readable side by side. A _test.go file is never
// importable, so the suite cannot be shared — a test kit is KRN-11's.

var errContractCallbackFailed = errors.New("dmpfpostgres_test: contract callback failed")

// writer is the resource set these tests bind to an open transaction. It writes
// through Conn() because the repository is Fase 5's: the contract of Within is
// provable with any write whose effect a SELECT can see.
type writer struct{ tx *dmpfpostgres.Tx }

func (w writer) write(ctx context.Context, id string) error {
	_, err := w.tx.Conn().Exec(ctx,
		`INSERT INTO dmpf_example_orders (order_id, version, snapshot) VALUES ($1, 1, '{}')`, id)
	return err
}

func bindWriter(tx *dmpfpostgres.Tx) writer { return writer{tx: tx} }

func kept(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var count int
	// Background, never the test's ctx: the cancelled-commit clause asserts
	// through a context it has just cancelled.
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM dmpf_example_orders").Scan(&count); err != nil {
		t.Fatalf("count(*) = %v, want nil", err)
	}
	return count
}

func TestWithinHonoursTheContract(t *testing.T) {
	t.Run("commits every write of the single transaction it opens", func(t *testing.T) {
		pool := openPool(t)
		uow := dmpfpostgres.NewUnitOfWork(pool, bindWriter)

		err := uow.Within(context.Background(), func(ctx context.Context, res writer) error {
			if err := res.write(ctx, "o-1"); err != nil {
				return err
			}
			return res.write(ctx, "o-2")
		})

		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
		if got := kept(t, pool); got != 2 {
			t.Fatalf("kept %d writes, want 2 (UOW-01, UOW-02)", got)
		}
	})

	t.Run("invokes the callback exactly once", func(t *testing.T) {
		pool := openPool(t)
		uow := dmpfpostgres.NewUnitOfWork(pool, bindWriter)
		calls := 0

		if err := uow.Within(context.Background(), func(context.Context, writer) error {
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
		pool := openPool(t)
		uow := dmpfpostgres.NewUnitOfWork(pool, bindWriter)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		invoked := false

		err := uow.Within(ctx, func(context.Context, writer) error {
			invoked = true
			return nil
		})

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Within() = %v, want context.Canceled (CTX-21)", err)
		}
		if invoked {
			t.Fatal("the callback ran on an already cancelled context")
		}
		if got := kept(t, pool); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})

	t.Run("rolls back and returns the callback error unwrapped", func(t *testing.T) {
		pool := openPool(t)
		uow := dmpfpostgres.NewUnitOfWork(pool, bindWriter)

		err := uow.Within(context.Background(), func(ctx context.Context, res writer) error {
			if err := res.write(ctx, "o-1"); err != nil {
				return err
			}
			return errContractCallbackFailed
		})

		if !errors.Is(err, errContractCallbackFailed) {
			t.Fatalf("Within() = %v, want the callback error", err)
		}
		if got := kept(t, pool); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})

	t.Run("returns the commit error and keeps nothing", func(t *testing.T) {
		pool := openPool(t)
		uow := dmpfpostgres.NewUnitOfWork(pool, bindWriter)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Cancelling the parent inside fn is what makes Commit fail without an
		// injectable failure: the memory realization arms one, a real database
		// has to be made to refuse.
		err := uow.Within(ctx, func(ctx context.Context, res writer) error {
			if err := res.write(ctx, "o-1"); err != nil {
				return err
			}
			cancel()
			return nil
		})

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Within() = %v, want the commit error as the provider produced it", err)
		}
		if got := kept(t, pool); got != 0 {
			t.Fatalf("kept %d writes, want 0 — a failed commit persists nothing (UOW-07)", got)
		}
	})

	t.Run("propagates a panic and discards the transaction", func(t *testing.T) {
		pool := openPool(t)
		uow := dmpfpostgres.NewUnitOfWork(pool, bindWriter)

		func() {
			defer func() {
				if recovered := recover(); recovered != "dmpfpostgres_test: contract boom" {
					t.Fatalf("recovered %v, want \"dmpfpostgres_test: contract boom\" (ERR-22)", recovered)
				}
			}()

			_ = uow.Within(context.Background(), func(ctx context.Context, res writer) error {
				if err := res.write(ctx, "o-1"); err != nil {
					return err
				}
				panic("dmpfpostgres_test: contract boom")
			})
		}()

		if got := kept(t, pool); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})
}
