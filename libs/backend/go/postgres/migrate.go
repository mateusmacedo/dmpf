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

//go:embed external_attempt.sql
var externalAttemptSchema string

//go:embed external_attempt_validate.sql
var externalAttemptValidation string

const migrateLockID = 0x646d7066

// Capability is a part of the kernel schema an app declares it uses, so its
// database carries only the structures its own roles touch.
type Capability string

const (
	// Outbox is what every producer needs.
	Outbox Capability = "outbox"
	// Inbox is what a consumer needs: the inbox and the quarantine where it
	// contains what it cannot process.
	Inbox           Capability = "inbox"
	ExternalAttempt Capability = "external_attempt"
)

type capabilityParts struct {
	schema string
	// WHY: VALIDATE CONSTRAINT needs only SHARE UPDATE EXCLUSIVE, but in the schema
	// transaction it would run under the ACCESS EXCLUSIVE that ADD CONSTRAINT holds
	// until commit; so it runs after that commit.
	validations []string
	tables      []string
	grants      []Grant
}

var capabilities = map[Capability]capabilityParts{
	Outbox: {
		schema: outboxSchema,
		tables: []string{"outbox"},
		grants: []Grant{{Table: "outbox", Privileges: readWrite}},
	},
	Inbox: {
		schema: inboxSchema,
		tables: []string{"inbox", "quarantine"},
		grants: []Grant{{Table: "inbox", Privileges: readWrite}, AppendOnly("quarantine")},
	},
	ExternalAttempt: {
		schema:      externalAttemptSchema,
		validations: []string{externalAttemptValidation},
		tables:      []string{"external_attempt"},
		grants:      []Grant{{Table: "external_attempt", Privileges: readWrite}},
	},
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

// Migrate applies each capability's kernel schema, then the context's schemas,
// in one locked transaction, and then each capability's validations, each in a
// locked transaction of its own. Every statement is idempotent, so a repeated
// call is a no-op: there is no migration tool and no version table. A failing
// validation returns its error and keeps the committed schema.
//
// WHY: IF NOT EXISTS is not concurrency-safe — two replicas racing on the same
// schema make one fail with a unique violation on pg_type. The lock is
// transaction-scoped, so it needs no cleanup path.
func Migrate(ctx context.Context, pool *pgxpool.Pool, caps []Capability, contextSchemas ...string) error {
	statements := make([]string, 0, len(caps)+len(contextSchemas))
	var validations []string
	for _, c := range dedupe(caps) {
		parts, ok := capabilities[c]
		if !ok {
			return fmt.Errorf("postgres: unknown capability %q", c)
		}
		statements = append(statements, parts.schema)
		validations = append(validations, parts.validations...)
	}
	statements = append(statements, contextSchemas...)

	if err := applyLocked(ctx, pool, statements...); err != nil {
		return err
	}
	for _, v := range validations {
		if err := applyLocked(ctx, pool, v); err != nil {
			return err
		}
	}
	return nil
}

func applyLocked(ctx context.Context, pool *pgxpool.Pool, statements ...string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, pinSearchPath); err != nil {
		return err
	}
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
