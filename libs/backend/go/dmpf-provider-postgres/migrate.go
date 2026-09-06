package dmpfpostgres

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Migrate applies schema.sql, creating dmpf_outbox, dmpf_inbox,
// dmpf_quarantine, dmpf_example_orders and dmpf_example_reservations. Every
// statement is CREATE TABLE/INDEX IF NOT EXISTS, so calling it more than once
// is a no-op: there is no external migration tool and no version table.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	return err
}
