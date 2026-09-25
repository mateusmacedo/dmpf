//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// testDatabase is this package's own database: the suite drops its public
// schema, and a database another project migrates is never a test fixture.
const testDatabase = "postgres_test"

// probeSchema is a table of the tests, not of the kernel: the kernel owns no
// aggregate, so the Table and the unit of work are proven over this one.
const probeSchema = `
CREATE TABLE IF NOT EXISTS probes (
  tenant_id text   NOT NULL,
  probe_id  text   NOT NULL,
  version   bigint NOT NULL,
  snapshot  jsonb  NOT NULL,
  CONSTRAINT probes_pkey PRIMARY KEY (tenant_id, probe_id)
);

CREATE INDEX IF NOT EXISTS probes_probe_id_idx ON probes (probe_id);

CREATE TABLE IF NOT EXISTS tagged_probes (
  tenant_id text   NOT NULL,
  probe_id  text   NOT NULL,
  version   bigint NOT NULL,
  label     text   NOT NULL,
  snapshot  jsonb  NOT NULL,
  CONSTRAINT tagged_probes_pkey PRIMARY KEY (tenant_id, probe_id)
);

CREATE INDEX IF NOT EXISTS tagged_probes_tenant_id_label_idx ON tagged_probes (tenant_id, label);
CREATE INDEX IF NOT EXISTS tagged_probes_probe_id_idx ON tagged_probes (probe_id);
CREATE INDEX IF NOT EXISTS tagged_probes_label_idx ON tagged_probes (label);`

var allCapabilities = []postgres.Capability{postgres.Outbox, postgres.Inbox}

var (
	cleanSchemaOnce sync.Once
	cleanSchemaErr  error
)

// openPool fails the run in CI when DMPF_PG_DSN is unset (fail-closed: an
// integration suite must never pass by skipping) and skips with setup
// instructions everywhere else. Tables are truncated before and after so
// package tests, forced to -p 1 (R10), never observe another test's rows.
func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return openPoolWith(t, func(*pgxpool.Config) {})
}

func openPoolWith(t *testing.T, configure func(*pgxpool.Config)) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DMPF_PG_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DMPF_PG_DSN is empty in CI: integration tests must not skip silently")
		}
		t.Skip("DMPF_PG_DSN not set; run `docker compose -f infra/local/docker-compose.yml --profile postgres up -d` and export DMPF_PG_DSN=postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	}

	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v, want nil", err)
	}
	ensureDatabase(t, ctx, cfg.ConnConfig)
	cfg.ConnConfig.Database = testDatabase
	configure(cfg)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	// A table an earlier run created, under a name the schema no longer
	// declares, would otherwise survive and hide a missing CREATE.
	cleanSchemaOnce.Do(func() {
		_, cleanSchemaErr = pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	})
	if cleanSchemaErr != nil {
		t.Fatalf("recreate schema public = %v, want nil", cleanSchemaErr)
	}
	if err := postgres.Migrate(ctx, pool, allCapabilities, probeSchema); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}

	truncate(t, ctx, pool)
	t.Cleanup(func() { truncate(t, ctx, pool) })

	return pool
}

// ensureDatabase creates the test database through the server the DSN names.
// A concurrent creation by another process is the same outcome, not a failure.
func ensureDatabase(t *testing.T, ctx context.Context, admin *pgx.ConnConfig) {
	t.Helper()

	conn, err := pgx.ConnectConfig(ctx, admin)
	if err != nil {
		t.Fatalf("connect to the admin database = %v, want nil", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	var found bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", testDatabase).Scan(&found); err != nil {
		t.Fatalf("look up %s = %v, want nil", testDatabase, err)
	}
	if found {
		return
	}
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{testDatabase}.Sanitize())
	var pgErr *pgconn.PgError
	if err != nil && (!errors.As(err, &pgErr) || pgErr.Code != "42P04") {
		t.Fatalf("CREATE DATABASE %s = %v, want nil", testDatabase, err)
	}
}

func truncate(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "TRUNCATE outbox, inbox, quarantine, probes, tagged_probes"); err != nil {
		t.Fatalf("TRUNCATE = %v, want nil", err)
	}
}

func TestPing(t *testing.T) {
	pool := openPool(t)

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() = %v, want nil", err)
	}
}
