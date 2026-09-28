//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestWaitForTablesReturnsOnceEveryTableExists(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS late_probes"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS late_probes") })

	done := make(chan error, 1)
	go func() { done <- postgres.WaitForTables(ctx, pool, 20*time.Millisecond, "outbox", "late_probes") }()

	select {
	case err := <-done:
		t.Fatalf("WaitForTables() = %v before late_probes existed", err)
	case <-time.After(150 * time.Millisecond):
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE late_probes (id bigint)"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForTables() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForTables() did not return after the table was created")
	}
}

func TestWaitForTablesStopsWithTheContext(t *testing.T) {
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := postgres.WaitForTables(ctx, pool, 20*time.Millisecond, "never_created")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForTables() = %v, want context.DeadlineExceeded", err)
	}
}
