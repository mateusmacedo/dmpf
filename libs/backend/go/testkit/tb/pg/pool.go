// Package pg is the Postgres side of tb, kept apart so a test that only reads
// fixtures through tb does not pull the provider and pgx into its closure —
// which is what the V29/V30 guard in fitness would name.
package pg

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// PostgresDSN is the variable every Postgres-backed suite reads.
const PostgresDSN = "DMPF_PG_DSN"

// kernelTables are the tables every DMPF context shares. A context with tables
// of its own names them at the call, because the kit cannot know a schema it
// does not own.
var kernelTables = []string{"outbox", "inbox", "quarantine", "dmpf_example_orders", "dmpf_example_reservations"}

// resetTimeout bounds every reset, so a lock left behind by a failed clause
// fails the cleanup instead of holding the binary until go test's -timeout.
const resetTimeout = 30 * time.Second

// OpenPool connects to the Postgres the suite runs against, migrates, and
// resets the five tables before and after the test, so package tests forced
// to -p 1 never observe another test's rows. Without the DSN it skips — or
// fails in CI, where an integration suite must never pass by skipping. The
// DSN must point at a loopback host: the reset is destructive, and a shared
// instance is never a test fixture.
func OpenPool(t testing.TB, extraTables ...string) *pgxpool.Pool {
	t.Helper()
	dsn := tb.Env(t, PostgresDSN)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pg.OpenPool: parse %s: %v", PostgresDSN, err)
	}
	if host := cfg.ConnConfig.Host; !loopback(host) {
		t.Fatalf("pg.OpenPool: %s points at %q; the harness truncates the DMPF tables and only accepts a loopback host", PostgresDSN, host)
	}
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pg.OpenPool: pgxpool.NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("pg.OpenPool: Migrate: %v", err)
	}
	ResetTables(t, pool, extraTables...)
	t.Cleanup(func() {
		// Errorf, not Fatalf: FailNow inside a cleanup skips the cleanups still
		// pending, and pool.Close is one of them.
		if err := reset(pool, extraTables); err != nil {
			t.Errorf("pg.OpenPool: reset after the test: %v", err)
		}
	})
	return pool
}

// ResetTables empties the shared DMPF tables plus the ones the caller names.
func ResetTables(t testing.TB, pool *pgxpool.Pool, extraTables ...string) {
	t.Helper()
	if err := reset(pool, extraTables); err != nil {
		t.Fatalf("pg.ResetTables: %v", err)
	}
}

func reset(pool *pgxpool.Pool, extraTables []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
	defer cancel()
	tables := append(append([]string(nil), kernelTables...), extraTables...)
	_, err := pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", "))
	return err
}

// loopback accepts localhost, the loopback addresses and a Unix socket
// directory — everything pgx resolves without leaving the machine.
func loopback(host string) bool {
	if host == "localhost" || strings.HasPrefix(host, "/") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
