//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	lognoop "go.opentelemetry.io/otel/log/noop"

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
	go func() {
		done <- postgres.WaitForTables(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), "outbox", "late_probes")
	}()

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
	err := postgres.WaitForTables(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), "never_created")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForTables() = %v, want context.DeadlineExceeded", err)
	}
}

func TestWaitForTablesNamesTheTableItWaitsFor(t *testing.T) {
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	logs := &recordingProcessor{}
	_ = postgres.WaitForTables(ctx, pool, 20*time.Millisecond, logs.provider(), "never_created")

	records := logs.named("waiting for table")
	if len(records) != 1 || records[0]["db.collection.name"] != "never_created" || records[0]["scope"] != postgresScope {
		t.Fatalf("log = %v, want one record naming never_created under db.collection.name, scoped to %s", records, postgresScope)
	}
	if _, free := records[0]["table"]; free {
		t.Fatalf("log = %v still carries the free key table", records)
	}
}
