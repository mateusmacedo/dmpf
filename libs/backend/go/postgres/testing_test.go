//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

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

// openPool runs the test in a database of its own (tb/pg), and truncates the
// tables before and after so a test that reuses the pool starts clean.
func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return openPoolWith(t, func(*pgxpool.Config) {})
}

func openPoolWith(t *testing.T, configure func(*pgxpool.Config)) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	cfg := pg.Config(t, "postgres")
	configure(cfg)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	if err := postgres.Migrate(ctx, pool, allCapabilities, probeSchema); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}

	truncate(t, ctx, pool)
	t.Cleanup(func() { truncate(t, ctx, pool) })

	return pool
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
