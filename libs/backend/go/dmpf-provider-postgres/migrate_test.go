//go:build integration

package dmpfpostgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
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
		for _, table := range []string{"dmpf_outbox", "dmpf_example_orders"} {
			if !exists(t, ctx, pool, tableExistsQuery, table) {
				t.Errorf("table %s does not exist", table)
			}
		}
	})

	t.Run("constraints", func(t *testing.T) {
		cases := []struct {
			name           string
			constraintType string
		}{
			{"dmpf_outbox_message_id_unique", "UNIQUE"},
			{"dmpf_outbox_status_check", "CHECK"},
			{"dmpf_outbox_available_at_check", "CHECK"},
			{"dmpf_outbox_payload_not_empty", "CHECK"},
		}
		for _, c := range cases {
			if !exists(t, ctx, pool, constraintExistsQuery, "dmpf_outbox", c.name, c.constraintType) {
				t.Errorf("constraint %s (%s) does not exist", c.name, c.constraintType)
			}
		}
	})

	t.Run("published_at index", func(t *testing.T) {
		if !exists(t, ctx, pool, indexExistsQuery, "dmpf_outbox_published_at_idx") {
			t.Error("index dmpf_outbox_published_at_idx does not exist")
		}
	})
}

func exists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) bool {
	t.Helper()

	var found bool
	if err := pool.QueryRow(ctx, query, args...).Scan(&found); err != nil {
		t.Fatalf("query = %v, want nil", err)
	}
	return found
}
