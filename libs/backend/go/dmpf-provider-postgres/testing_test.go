//go:build integration

package dmpfpostgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

// openPool fails the run in CI when DMPF_PG_DSN is unset (fail-closed: an
// integration suite must never pass by skipping) and skips with setup
// instructions everywhere else. Tables are truncated before and after so
// package tests, forced to -p 1 (R10), never observe another test's rows.
func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DMPF_PG_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DMPF_PG_DSN is empty in CI: integration tests must not skip silently")
		}
		t.Skip("DMPF_PG_DSN not set; run `docker compose -f infra/local/docker-compose.yml --profile postgres up -d` and export DMPF_PG_DSN=postgres://app:app@localhost:5432/app?sslmode=disable")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	if err := dmpfpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}

	truncate(t, ctx, pool)
	t.Cleanup(func() { truncate(t, ctx, pool) })

	return pool
}

func truncate(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "TRUNCATE dmpf_outbox, dmpf_inbox, dmpf_quarantine, dmpf_example_orders, dmpf_example_reservations"); err != nil {
		t.Fatalf("TRUNCATE = %v, want nil", err)
	}
}

func TestPing(t *testing.T) {
	pool := openPool(t)

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() = %v, want nil", err)
	}
}
