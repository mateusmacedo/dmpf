//go:build integration

package dmpfpostgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

func openConcurrencyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DMPF_PG_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DMPF_PG_DSN is empty in CI")
		}
		t.Skip("DMPF_PG_DSN not set")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v", err)
	}
	cfg.MaxConns = 4

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() = %v", err)
	}
	t.Cleanup(pool.Close)

	if err := dmpfpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() = %v", err)
	}

	if _, err := pool.Exec(ctx, "TRUNCATE dmpf_outbox, dmpf_inbox, dmpf_quarantine, dmpf_example_orders, dmpf_example_reservations"); err != nil {
		t.Fatalf("TRUNCATE = %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "TRUNCATE dmpf_outbox, dmpf_inbox, dmpf_quarantine, dmpf_example_orders, dmpf_example_reservations")
	})
	return pool
}

func TestConcurrentRegisterProcessedUnblocksWithR2(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()

	aRegistered := make(chan struct{})
	bClassified := make(chan string, 1)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("A: Begin() = %v", err)
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("A: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		_ = r.Match(
			func(p dmpfports.Pending) error {
				return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 200})
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)
		close(aRegistered)
		// B needs to be blocked on the key before A commits, or the assertion
		// below would pass without ever exercising INB-06.
		time.Sleep(100 * time.Millisecond)
		if err := pgxTx.Commit(ctx); err != nil {
			t.Errorf("A: Commit() = %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		<-aRegistered

		// WHY: small sleep to ensure B's Register starts after A has inserted
		// but before A commits, so B blocks on the unique constraint lock.
		time.Sleep(50 * time.Millisecond)

		start := time.Now()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("B: Begin() = %v", err)
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("B: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		if waited := time.Since(start); waited < 40*time.Millisecond {
			t.Errorf("B: Register() returned after %v, want it blocked on A's open transaction (INB-06)", waited)
		}

		var branch string
		_ = r.Match(
			func(p dmpfports.Pending) error {
				branch = "first"
				return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 300})
			},
			func() error { branch = "processed"; return nil },
			func() error { branch = "rejected"; return nil },
			func() error { branch = "collision"; return nil },
		)
		bClassified <- branch
		_ = pgxTx.Rollback(ctx)
	}()

	wg.Wait()
	close(bClassified)

	branch := <-bClassified
	if branch != "processed" {
		t.Fatalf("B branch = %q, want %q — after A commits processed, B sees R2", branch, "processed")
	}
}

func TestConcurrentRegisterRejectedUnblocksWithR3(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()

	aRegistered := make(chan struct{})
	bClassified := make(chan string, 1)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("A: Begin() = %v", err)
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("A: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		_ = r.Match(
			func(p dmpfports.Pending) error {
				return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusRejected, At: 200, LastError: "test"})
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)
		close(aRegistered)
		// B needs to be blocked on the key before A commits, or the assertion
		// below would pass without ever exercising INB-06.
		time.Sleep(100 * time.Millisecond)
		if err := pgxTx.Commit(ctx); err != nil {
			t.Errorf("A: Commit() = %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		<-aRegistered
		time.Sleep(50 * time.Millisecond)

		start := time.Now()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("B: Begin() = %v", err)
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("B: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		if waited := time.Since(start); waited < 40*time.Millisecond {
			t.Errorf("B: Register() returned after %v, want it blocked on A's open transaction (INB-06)", waited)
		}

		var branch string
		_ = r.Match(
			func(p dmpfports.Pending) error {
				branch = "first"
				return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 300})
			},
			func() error { branch = "processed"; return nil },
			func() error { branch = "rejected"; return nil },
			func() error { branch = "collision"; return nil },
		)
		bClassified <- branch
		_ = pgxTx.Rollback(ctx)
	}()

	wg.Wait()
	close(bClassified)

	branch := <-bClassified
	if branch != "rejected" {
		t.Fatalf("B branch = %q, want %q — after A commits rejected, B sees R3", branch, "rejected")
	}
}

func TestConcurrentRegisterRollbackUnblocksWithR1(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()

	aRegistered := make(chan struct{})
	bResult := make(chan string, 1)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("A: Begin() = %v", err)
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("A: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		_ = r.Match(
			func(p dmpfports.Pending) error {
				return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 200})
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)
		close(aRegistered)
		time.Sleep(100 * time.Millisecond)
		_ = pgxTx.Rollback(ctx)
	}()

	go func() {
		defer wg.Done()
		<-aRegistered
		time.Sleep(50 * time.Millisecond)

		start := time.Now()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("B: Begin() = %v", err)
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("B: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		if waited := time.Since(start); waited < 40*time.Millisecond {
			t.Errorf("B: Register() returned after %v, want it blocked on A's open transaction (INB-06)", waited)
		}

		var branch string
		_ = r.Match(
			func(p dmpfports.Pending) error {
				branch = "first"
				return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 300})
			},
			func() error { branch = "processed"; return nil },
			func() error { branch = "rejected"; return nil },
			func() error { branch = "collision"; return nil },
		)
		bResult <- branch
		if err := pgxTx.Commit(ctx); err != nil {
			t.Errorf("B: Commit() = %v", err)
		}
	}()

	wg.Wait()
	close(bResult)

	branch := <-bResult
	if branch != "first" {
		t.Fatalf("B branch = %q, want %q — after A rolls back, B gets R1", branch, "first")
	}
}

func TestConcurrentRegisterLockTimeoutReturnsErrRegisterTimeout(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()

	aRegistered := make(chan struct{})
	bDone := make(chan struct{})

	var (
		bErr     error
		bElapsed time.Duration
	)

	go func() {
		defer close(bDone)
		<-aRegistered
		time.Sleep(50 * time.Millisecond)

		start := time.Now()
		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			bErr = err
			return
		}
		tx := dmpfpostgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 300*time.Millisecond)

		_, bErr = inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		bElapsed = time.Since(start)
		_ = pgxTx.Rollback(ctx)
	}()

	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("A: Begin() = %v", err)
	}
	tx := dmpfpostgres.NewTx(pgxTx)
	inbox := tx.Inbox("orders", 0)

	r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
	if err != nil {
		t.Fatalf("A: Register() = %v", err)
	}
	_ = r.Match(
		func(p dmpfports.Pending) error {
			return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 200})
		},
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)
	close(aRegistered)

	<-bDone
	_ = pgxTx.Rollback(ctx)

	if !errors.Is(bErr, dmpfports.ErrRegisterTimeout) {
		t.Fatalf("B Register() = %v, want ErrRegisterTimeout", bErr)
	}

	var pgErr *pgconn.PgError
	if !errors.As(bErr, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("B Register() = %v, want the driver error with SQLSTATE 55P03 (lock_not_available) reachable through errors.As", bErr)
	}
	if bElapsed < 200*time.Millisecond {
		t.Fatalf("B returned in %v, faster than the 300ms lock_timeout: the server did not interrupt the wait", bElapsed)
	}
	if bElapsed > 2*time.Second {
		t.Fatalf("B waited %v, much longer than 300ms lock_timeout — lock_timeout did not work", bElapsed)
	}
}
