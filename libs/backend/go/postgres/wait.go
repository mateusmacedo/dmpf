package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
)

// WaitForTables blocks until every table exists, for a role that reads the
// schema another role applies. It polls to_regclass, which answers NULL for an
// absent table instead of raising an error on the server.
func WaitForTables(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, logs log.LoggerProvider, tables ...string) error {
	logger := loggerOf(logs)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var reported string
	var reportedAt time.Time
	for {
		missing, err := firstMissing(ctx, pool, tables)
		if err != nil {
			// WHY: with the deadline expiring mid-write, pgx v5.10 returns the pgproto3
			// writeError over the i/o timeout, without the context error (pgproto3/pgproto3.go:62).
			if ctxErr := ctx.Err(); ctxErr != nil {
				err = ctxErr
			}
			return fmt.Errorf("wait for tables: %w", err)
		}
		if missing == "" {
			return nil
		}
		if missing != reported || time.Since(reportedAt) >= waitReportEvery {
			logger.InfoContext(ctx, "waiting for table", string(semconv.DBCollectionNameKey), missing)
			reported, reportedAt = missing, time.Now()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for table %s: %w", missing, ctx.Err())
		case <-ticker.C:
		}
	}
}

const waitReportEvery = 30 * time.Second

func loggerOf(provider log.LoggerProvider) *slog.Logger {
	return logging.NewLogger(provider, reflect.TypeFor[Tx]().PkgPath())
}

const selectTableExists = "SELECT to_regclass($1) IS NOT NULL"

var _ = declare(selectTableExists, "SELECT", "")

func firstMissing(ctx context.Context, pool *pgxpool.Pool, tables []string) (string, error) {
	for _, table := range tables {
		var present bool
		if err := pool.QueryRow(ctx, selectTableExists, table).Scan(&present); err != nil {
			return "", err
		}
		if !present {
			return table, nil
		}
	}
	return "", nil
}
