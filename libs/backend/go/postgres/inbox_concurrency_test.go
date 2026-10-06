//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func openConcurrencyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return openPoolWith(t, func(cfg *pgxpool.Config) { cfg.MaxConns = 4 })
}

func requireReleased(t *testing.T, aReleasing <-chan struct{}) {
	t.Helper()
	select {
	case <-aReleasing:
	default:
		t.Errorf("B: Register() returned before A released its transaction, want it blocked on A's open transaction (INB-06)")
	}
}

func startBlockedRegister(t *testing.T, ctx context.Context, pool *pgxpool.Pool, aRegistered <-chan struct{}, bRegistering chan<- struct{}, aReleasing <-chan struct{}, wg *sync.WaitGroup, classified chan<- string) {
	t.Helper()
	go func() {
		defer wg.Done()
		<-aRegistered
		close(bRegistering)

		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("B: Begin() = %v", err)
			return
		}
		tx := postgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("B: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		requireReleased(t, aReleasing)

		var branch string
		_ = r.Match(
			func(p ports.Pending) error {
				branch = "first"
				return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 300})
			},
			func() error { branch = "processed"; return nil },
			func() error { branch = "rejected"; return nil },
			func() error { branch = "collision"; return nil },
		)
		classified <- branch
		_ = pgxTx.Rollback(ctx)
	}()
}

func TestConcurrentRegisterProcessedUnblocksWithR2(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()

	aRegistered := make(chan struct{})
	bRegistering := make(chan struct{})
	aReleasing := make(chan struct{})
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
		tx := postgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("A: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		_ = r.Match(
			func(p ports.Pending) error {
				return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 200})
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)
		close(aRegistered)
		// B needs to be blocked on the key before A commits, or the assertion
		// below would pass without ever exercising INB-06.
		<-bRegistering
		time.Sleep(50 * time.Millisecond)
		close(aReleasing)
		if err := pgxTx.Commit(ctx); err != nil {
			t.Errorf("A: Commit() = %v", err)
		}
	}()

	startBlockedRegister(t, ctx, pool, aRegistered, bRegistering, aReleasing, &wg, bClassified)

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
	bRegistering := make(chan struct{})
	aReleasing := make(chan struct{})
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
		tx := postgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("A: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		_ = r.Match(
			func(p ports.Pending) error {
				return p.Complete(ctx, ports.Completion{Status: ports.StatusRejected, At: 200, LastError: "test"})
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)
		close(aRegistered)
		// B needs to be blocked on the key before A commits, or the assertion
		// below would pass without ever exercising INB-06.
		<-bRegistering
		time.Sleep(50 * time.Millisecond)
		close(aReleasing)
		if err := pgxTx.Commit(ctx); err != nil {
			t.Errorf("A: Commit() = %v", err)
		}
	}()

	startBlockedRegister(t, ctx, pool, aRegistered, bRegistering, aReleasing, &wg, bClassified)

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
	bRegistering := make(chan struct{})
	aReleasing := make(chan struct{})
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
		tx := postgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("A: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		_ = r.Match(
			func(p ports.Pending) error {
				return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 200})
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)
		close(aRegistered)
		<-bRegistering
		time.Sleep(50 * time.Millisecond)
		close(aReleasing)
		_ = pgxTx.Rollback(ctx)
	}()

	go func() {
		defer wg.Done()
		<-aRegistered
		close(bRegistering)

		pgxTx, err := pool.Begin(ctx)
		if err != nil {
			t.Errorf("B: Begin() = %v", err)
			return
		}
		tx := postgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 0)

		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Errorf("B: Register() = %v", err)
			_ = pgxTx.Rollback(ctx)
			return
		}
		requireReleased(t, aReleasing)

		var branch string
		_ = r.Match(
			func(p ports.Pending) error {
				branch = "first"
				return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 300})
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
		tx := postgres.NewTx(pgxTx)
		inbox := tx.Inbox("orders", 300*time.Millisecond)

		_, bErr = inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		bElapsed = time.Since(start)
		_ = pgxTx.Rollback(ctx)
	}()

	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("A: Begin() = %v", err)
	}
	tx := postgres.NewTx(pgxTx)
	inbox := tx.Inbox("orders", 0)

	r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
	if err != nil {
		t.Fatalf("A: Register() = %v", err)
	}
	_ = r.Match(
		func(p ports.Pending) error {
			return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 200})
		},
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)
	close(aRegistered)

	<-bDone
	_ = pgxTx.Rollback(ctx)

	if !errors.Is(bErr, ports.ErrRegisterTimeout) {
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
