// Package pg is the Postgres side of tb, kept apart so a test that only reads
// fixtures through tb does not pull the provider and pgx into its closure —
// which is what the V29/V30 guard in fitness would name.
package pg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

// PostgresDSN is the variable every Postgres-backed suite reads.
const PostgresDSN = "DMPF_PG_DSN"

const resetStatement = "TRUNCATE dmpf_outbox, dmpf_inbox, dmpf_quarantine, dmpf_example_orders, dmpf_example_reservations"

// OpenPool connects to the Postgres the suite runs against, migrates, and
// resets the five tables before and after the test, so package tests forced
// to -p 1 never observe another test's rows. Without the DSN it skips — or
// fails in CI, where an integration suite must never pass by skipping.
func OpenPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := tb.Env(t, PostgresDSN)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pg.OpenPool: pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := dmpfpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("pg.OpenPool: Migrate: %v", err)
	}
	ResetTables(t, pool)
	t.Cleanup(func() { ResetTables(t, pool) })
	return pool
}

// ResetTables empties the five DMPF tables of the example schema.
func ResetTables(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), resetStatement); err != nil {
		t.Fatalf("pg.ResetTables: %v", err)
	}
}
