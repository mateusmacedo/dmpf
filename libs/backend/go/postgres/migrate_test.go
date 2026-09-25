//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	tableExistsQuery      = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`
	constraintExistsQuery = `SELECT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name = $1 AND constraint_name = $2 AND constraint_type = $3)`
	indexExistsQuery      = `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := openPool(t)

	if err := postgres.Migrate(context.Background(), pool, allCapabilities, probeSchema); err != nil {
		t.Fatalf("second Migrate() = %v, want nil", err)
	}
}

func TestMigrateAppliesTheSchemaOfAContext(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	const index = "migrate_context_schema_idx"
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP INDEX IF EXISTS "+index) })

	if err := postgres.Migrate(ctx, pool, allCapabilities, "CREATE INDEX IF NOT EXISTS "+index+" ON probes (version)"); err != nil {
		t.Fatalf("Migrate() with a context schema = %v, want nil", err)
	}
	if !exists(t, ctx, pool, indexExistsQuery, index) {
		t.Fatalf("index %s does not exist: the context schema was not applied", index)
	}
}

func TestMigrateCreatesTheOutboxSchema(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	t.Run("tables", func(t *testing.T) {
		for _, table := range []string{"outbox", "inbox", "quarantine"} {
			if !exists(t, ctx, pool, tableExistsQuery, table) {
				t.Errorf("table %s does not exist", table)
			}
		}
	})

	t.Run("constraints", func(t *testing.T) {
		cases := []struct {
			table          string
			name           string
			constraintType string
		}{
			{"outbox", "outbox_pkey", "PRIMARY KEY"},
			{"outbox", "outbox_message_id_key", "UNIQUE"},
			{"outbox", "outbox_status_check", "CHECK"},
			{"outbox", "outbox_available_at_check", "CHECK"},
			{"outbox", "outbox_payload_check", "CHECK"},
			{"inbox", "inbox_consumer_name_message_id_key", "UNIQUE"},
			{"inbox", "inbox_status_check", "CHECK"},
			{"quarantine", "quarantine_pkey", "PRIMARY KEY"},
			{"quarantine", "quarantine_envelope_check", "CHECK"},
		}
		for _, c := range cases {
			if !exists(t, ctx, pool, constraintExistsQuery, c.table, c.name, c.constraintType) {
				t.Errorf("constraint %s (%s) on %s does not exist", c.name, c.constraintType, c.table)
			}
		}
	})

	t.Run("indexes", func(t *testing.T) {
		for _, idx := range []string{"outbox_published_at_idx", "outbox_claim_idx", "inbox_retention_idx", "quarantine_consumer_name_reason_idx"} {
			if !exists(t, ctx, pool, indexExistsQuery, idx) {
				t.Errorf("index %s does not exist", idx)
			}
		}
	})
}

func TestMigrateRefusesAnUnknownCapability(t *testing.T) {
	pool := openPool(t)

	if err := postgres.Migrate(context.Background(), pool, []postgres.Capability{"ledger"}); err == nil {
		t.Fatal("Migrate() with an unknown capability = nil, want an error naming it")
	}
}

func TestMigrateInboxStatusCheckRejectsInvalidStatus(t *testing.T) {
	pool := openPool(t)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO inbox (consumer_name, message_id, message_type, payload_hash, received_at, processed_at, status)
		VALUES ('test', 'm-1', 'example', 'h1', 100, 100, 'processing')`)
	if err == nil {
		t.Fatal("INSERT with status 'processing' should fail, want CHECK violation (23514)")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("err = %v, want SQLSTATE 23514 (check_violation)", err)
	}
}

func exists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) bool {
	t.Helper()

	var found bool
	if err := pool.QueryRow(ctx, query, args...).Scan(&found); err != nil {
		t.Fatalf("query = %v, want nil", err)
	}
	return found
}
