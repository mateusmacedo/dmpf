package postgres

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

const migrateLockID = 0x646d7066

// Migrate applies schema.sql, creating outbox, inbox,
// quarantine, dmpf_example_orders and dmpf_example_reservations, and then
// each schema a context declares, under the same lock and in the same
// transaction. Every statement is CREATE TABLE/INDEX IF NOT EXISTS, so calling
// it more than once is a no-op: there is no external migration tool and no
// version table.
//
// WHY: IF NOT EXISTS is not concurrency-safe — two replicas racing on the same
// schema make one fail with a unique violation on pg_type. The lock is
// transaction-scoped, so it needs no cleanup path.
func Migrate(ctx context.Context, pool *pgxpool.Pool, contextSchemas ...string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(migrateLockID)); err != nil {
		return err
	}
	for _, statements := range append([]string{schema}, contextSchemas...) {
		if _, err := tx.Exec(ctx, statements); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
