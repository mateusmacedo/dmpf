package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed outbox.sql
var outboxSchema string

//go:embed inbox.sql
var inboxSchema string

const migrateLockID = 0x646d7066

// Capability is a part of the kernel schema an app declares it uses, so its
// database carries only the structures its own roles touch.
type Capability string

const (
	// Outbox is what every producer needs.
	Outbox Capability = "outbox"
	// Inbox is what a consumer needs: the inbox and the quarantine where it
	// contains what it cannot process.
	Inbox Capability = "inbox"
)

type capabilityParts struct {
	schema string
	tables []string
}

var capabilities = map[Capability]capabilityParts{
	Outbox: {schema: outboxSchema, tables: []string{"outbox"}},
	Inbox:  {schema: inboxSchema, tables: []string{"inbox", "quarantine"}},
}

// Tables names the kernel tables the capabilities create, in declaration order
// and without repetition. An unknown capability contributes nothing.
func Tables(caps ...Capability) []string {
	var tables []string
	for _, c := range dedupe(caps) {
		tables = append(tables, capabilities[c].tables...)
	}
	return tables
}

// Migrate applies the kernel schema of each capability and then each schema a
// context declares, under the same lock and in the same transaction. Every
// statement is CREATE TABLE/INDEX IF NOT EXISTS, so calling it more than once
// is a no-op: there is no external migration tool and no version table.
//
// WHY: IF NOT EXISTS is not concurrency-safe — two replicas racing on the same
// schema make one fail with a unique violation on pg_type. The lock is
// transaction-scoped, so it needs no cleanup path.
func Migrate(ctx context.Context, pool *pgxpool.Pool, caps []Capability, contextSchemas ...string) error {
	statements := make([]string, 0, len(caps)+len(contextSchemas))
	for _, c := range dedupe(caps) {
		parts, ok := capabilities[c]
		if !ok {
			return fmt.Errorf("postgres: unknown capability %q", c)
		}
		statements = append(statements, parts.schema)
	}
	statements = append(statements, contextSchemas...)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(migrateLockID)); err != nil {
		return err
	}
	for _, s := range statements {
		if _, err := tx.Exec(ctx, s); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func dedupe(caps []Capability) []Capability {
	seen := make([]Capability, 0, len(caps))
	for _, c := range caps {
		if !slices.Contains(seen, c) {
			seen = append(seen, c)
		}
	}
	return seen
}
