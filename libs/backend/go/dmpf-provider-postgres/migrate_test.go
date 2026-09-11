//go:build integration

package dmpfpostgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

const (
	tableExistsQuery      = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`
	constraintExistsQuery = `SELECT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name = $1 AND constraint_name = $2 AND constraint_type = $3)`
	indexExistsQuery      = `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := openPool(t)

	if err := dmpfpostgres.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second Migrate() = %v, want nil", err)
	}
}

func TestMigrateCreatesTheOutboxSchema(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	t.Run("tables", func(t *testing.T) {
		for _, table := range []string{"dmpf_outbox", "dmpf_inbox", "dmpf_quarantine", "dmpf_example_orders", "dmpf_example_reservations"} {
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
			{"dmpf_outbox", "dmpf_outbox_message_id_unique", "UNIQUE"},
			{"dmpf_outbox", "dmpf_outbox_status_check", "CHECK"},
			{"dmpf_outbox", "dmpf_outbox_available_at_check", "CHECK"},
			{"dmpf_outbox", "dmpf_outbox_payload_not_empty", "CHECK"},
			{"dmpf_inbox", "dmpf_inbox_key", "UNIQUE"},
			{"dmpf_inbox", "dmpf_inbox_status_check", "CHECK"},
			{"dmpf_quarantine", "dmpf_quarantine_envelope_not_empty", "CHECK"},
		}
		for _, c := range cases {
			if !exists(t, ctx, pool, constraintExistsQuery, c.table, c.name, c.constraintType) {
				t.Errorf("constraint %s (%s) on %s does not exist", c.name, c.constraintType, c.table)
			}
		}
	})

	t.Run("indexes", func(t *testing.T) {
		for _, idx := range []string{"dmpf_outbox_published_at_idx", "dmpf_outbox_claim_idx", "dmpf_inbox_retention_idx", "dmpf_quarantine_reason_idx"} {
			if !exists(t, ctx, pool, indexExistsQuery, idx) {
				t.Errorf("index %s does not exist", idx)
			}
		}
	})
}

func TestMigrateInboxStatusCheckRejectsInvalidStatus(t *testing.T) {
	pool := openPool(t)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO dmpf_inbox (consumer_name, message_id, message_type, payload_hash, received_at, processed_at, status)
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
