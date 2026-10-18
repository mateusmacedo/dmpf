package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
)

// WaitForTables blocks until every table exists, for a role that reads the
// schema another role applies. It polls to_regclass, which answers NULL for an
// absent table instead of raising an error on the server.
func WaitForTables(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, logs log.LoggerProvider, tables ...string) error {
	return waitUntil(ctx, interval, logs, "table", func(ctx context.Context) (awaited, error) {
		table, err := firstMissing(ctx, pool, tables)
		return awaited{table: table}, err
	})
}

// WHY: an owner or a superuser holds every privilege once the table exists, so
// an app without a migrator waits as before.
func WaitForGrants(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, logs log.LoggerProvider, grants ...Grant) error {
	return waitUntil(ctx, interval, logs, "grant", func(ctx context.Context) (awaited, error) {
		return firstUngranted(ctx, pool, grants)
	})
}

// WHY: only a divergence converges by waiting, for a replica that starts before
// the migrator adds the constraint; a malformed declaration or a server error never converges.
func WaitForUniques(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, logs log.LoggerProvider, sets ...UniqueSet) error {
	return waitUntil(ctx, interval, logs, "unique constraint", func(ctx context.Context) (awaited, error) {
		divergent, err := firstDivergentUnique(ctx, pool, sets)
		return awaited{unique: divergent}, err
	})
}

type awaited struct {
	table     string
	privilege Privilege
	unique    divergence
}

func (p awaited) String() string {
	if p.privilege == "" {
		return p.table
	}
	return fmt.Sprintf("%s on %s", p.privilege, p.table)
}

func (p awaited) interrupted(what string, cause error) error {
	if p.unique != (divergence{}) {
		return fmt.Errorf("wait for %s: %w: %w", what, p.unique.err(), cause)
	}
	return fmt.Errorf("wait for %s %s: %w", what, p, cause)
}

func waitUntil(ctx context.Context, interval time.Duration, logs log.LoggerProvider, what string, next func(context.Context) (awaited, error)) error {
	logger := loggerOf(logs)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var reported awaited
	var reportedAt time.Time
	for {
		missing, err := next(ctx)
		if err != nil {
			// WHY: with the deadline expiring mid-write, pgx v5.10 returns the pgproto3
			// writeError over the i/o timeout, without the context error (pgproto3/pgproto3.go:62).
			if ctxErr := ctx.Err(); ctxErr != nil {
				if reported != (awaited{}) {
					return reported.interrupted(what, ctxErr)
				}
				err = ctxErr
			}
			return fmt.Errorf("wait for %ss: %w", what, err)
		}
		if missing == (awaited{}) {
			return nil
		}
		if missing != reported || time.Since(reportedAt) >= waitReportEvery {
			reportWaiting(ctx, logger, missing)
			reported, reportedAt = missing, time.Now()
		}
		select {
		case <-ctx.Done():
			return missing.interrupted(what, ctx.Err())
		case <-ticker.C:
		}
	}
}

func reportWaiting(ctx context.Context, logger *slog.Logger, missing awaited) {
	if missing.unique != (divergence{}) {
		logger.InfoContext(ctx, "waiting for unique constraint",
			string(semconv.DBCollectionNameKey), missing.unique.table,
			keyConstraintName, missing.unique.constraint,
			keyConstraintDivergence, missing.unique.reason)
		return
	}
	if missing.privilege == "" {
		logger.InfoContext(ctx, "waiting for table", string(semconv.DBCollectionNameKey), missing.table)
		return
	}
	logger.InfoContext(ctx, "waiting for table privilege",
		string(semconv.DBCollectionNameKey), missing.table,
		string(semconv.DBOperationNameKey), string(missing.privilege))
}

const waitReportEvery = 30 * time.Second

const (
	keyConstraintName       = "dmpf.db.constraint.name"
	keyConstraintDivergence = "dmpf.db.constraint.divergence"
)

func loggerOf(provider log.LoggerProvider) *slog.Logger {
	return logging.NewLogger(provider, reflect.TypeFor[Tx]().PkgPath())
}

const selectFirstMissingTable = `
SELECT d.tablename FROM unnest($1::text[]) WITH ORDINALITY AS d(tablename, position)
WHERE to_regclass('public.' || quote_ident(lower(d.tablename))) IS NULL
ORDER BY d.position LIMIT 1`

var _ = declare(selectFirstMissingTable, "SELECT", "")

func firstMissing(ctx context.Context, pool *pgxpool.Pool, tables []string) (string, error) {
	var table string
	switch err := pool.QueryRow(ctx, selectFirstMissingTable, tables).Scan(&table); {
	case errors.Is(err, pgx.ErrNoRows):
		return "", nil
	case err != nil:
		return "", err
	}
	return table, nil
}

const selectFirstUngranted = `
SELECT d.tablename, d.privilege FROM unnest($1::text[], $2::text[]) WITH ORDINALITY AS d(tablename, privilege, position)
WHERE NOT COALESCE(has_table_privilege(to_regclass('public.' || quote_ident(lower(d.tablename)))::oid, d.privilege), false)
ORDER BY d.position LIMIT 1`

var _ = declare(selectFirstUngranted, "SELECT", "")

func firstUngranted(ctx context.Context, pool *pgxpool.Pool, grants []Grant) (awaited, error) {
	var tables, privileges []string
	for _, g := range grants {
		for _, p := range g.Privileges {
			tables = append(tables, g.Table)
			privileges = append(privileges, string(p))
		}
	}
	var missing awaited
	switch err := pool.QueryRow(ctx, selectFirstUngranted, tables, privileges).Scan(&missing.table, &missing.privilege); {
	case errors.Is(err, pgx.ErrNoRows):
		return awaited{}, nil
	case err != nil:
		return awaited{}, err
	}
	return missing, nil
}
