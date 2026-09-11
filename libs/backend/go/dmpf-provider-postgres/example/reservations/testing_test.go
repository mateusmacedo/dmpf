//go:build integration

package reservationspg_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

// Duplicated from the parent package's harness on purpose: a _test.go file is
// never importable, so reservationspg_test cannot reuse dmpfpostgres_test's
// openPool. An exported test kit is KRN-11's.
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

	truncate(t, pool)
	t.Cleanup(func() { truncate(t, pool) })

	return pool
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(context.Background(), "TRUNCATE dmpf_outbox, dmpf_inbox, dmpf_quarantine, dmpf_example_orders, dmpf_example_reservations"); err != nil {
		t.Fatalf("TRUNCATE = %v, want nil", err)
	}
}
