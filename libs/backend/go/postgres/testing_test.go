//go:build integration

package postgres_test

import (
	"context"
	"sync"
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
CREATE INDEX IF NOT EXISTS tagged_probes_label_idx ON tagged_probes (label);

CREATE TABLE IF NOT EXISTS unique_probes (
  tenant_id text   NOT NULL,
  probe_id  text   NOT NULL,
  version   bigint NOT NULL,
  name      text   NOT NULL,
  code      text   NOT NULL,
  label     text   NOT NULL,
  ref_a     text   NOT NULL,
  ref_b     text   NOT NULL,
  CONSTRAINT unique_probes_pkey PRIMARY KEY (tenant_id, probe_id),
  CONSTRAINT unique_probes_tenant_id_name_key UNIQUE (tenant_id, name),
  CONSTRAINT unique_probes_tenant_id_code_key UNIQUE (tenant_id, code),
  CONSTRAINT unique_probes_tenant_id_label_key UNIQUE (tenant_id, label),
  CONSTRAINT unique_probes_tenant_id_ref_a_ref_b_key UNIQUE (tenant_id, ref_a, ref_b)
);

CREATE INDEX IF NOT EXISTS unique_probes_probe_id_idx ON unique_probes (probe_id);

CREATE TABLE IF NOT EXISTS guarded_heads (
  tenant_id text   NOT NULL,
  head_id   text   NOT NULL,
  version   bigint NOT NULL,
  stage     text   NOT NULL,
  CONSTRAINT guarded_heads_pkey PRIMARY KEY (tenant_id, head_id)
);

CREATE TABLE IF NOT EXISTS guarded_series (
  tenant_id     text   NOT NULL,
  entry_id      text   NOT NULL,
  version       bigint NOT NULL,
  owner_id      text   NOT NULL,
  status        text   NOT NULL,
  supersedes_id text,
  CONSTRAINT guarded_series_pkey PRIMARY KEY (tenant_id, entry_id),
  CONSTRAINT guarded_series_tenant_id_supersedes_id_key UNIQUE (tenant_id, supersedes_id)
);

CREATE INDEX IF NOT EXISTS guarded_series_owner_id_idx ON guarded_series (owner_id);`

var allCapabilities = []postgres.Capability{postgres.Outbox, postgres.Inbox, postgres.ExternalAttempt}

var probeTableGrants = []postgres.Grant{postgres.Mutable("probes"), postgres.Mutable("tagged_probes"), postgres.Mutable("unique_probes"), postgres.Mutable("guarded_heads"), postgres.Mutable("guarded_series")}

func openPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return openPoolWith(t, func(*pgxpool.Config) {})
}

func openPoolWith(t testing.TB, configure func(*pgxpool.Config)) *pgxpool.Pool {
	t.Helper()
	admin := openAdminPool(t)
	pg.GrantRuntime(t, admin, pg.Options{Capabilities: allCapabilities, Grants: probeTableGrants})
	cfg := pg.RuntimeConfig(t, "postgres")
	configure(cfg)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)
	adminOf.Store(pool, admin)
	t.Cleanup(func() { adminOf.Delete(pool) })
	return pool
}

var adminPools, adminOf sync.Map

func openAdminPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if pool, ok := adminPools.Load(t); ok {
		return pool.(*pgxpool.Pool)
	}
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, pg.Config(t, "postgres"))
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool, allCapabilities, probeSchema); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}
	truncate(t, ctx, pool)
	t.Cleanup(func() { truncate(t, ctx, pool) })
	adminPools.Store(t, pool)
	t.Cleanup(func() { adminPools.Delete(t) })
	return pool
}

func truncate(t testing.TB, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if admin, ok := adminOf.Load(pool); ok {
		pool = admin.(*pgxpool.Pool)
	}

	if _, err := pool.Exec(ctx, "TRUNCATE outbox, inbox, quarantine, external_attempt, probes, tagged_probes, unique_probes, guarded_heads, guarded_series"); err != nil {
		t.Fatalf("TRUNCATE = %v, want nil", err)
	}
}

func TestPing(t *testing.T) {
	pool := openPool(t)

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() = %v, want nil", err)
	}
}
