package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WaitForTables blocks until every table exists, for a role that reads the
// schema another role applies. It polls to_regclass, which answers NULL for an
// absent table instead of raising an error on the server.
func WaitForTables(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, logger *slog.Logger, tables ...string) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var reported string
	var reportedAt time.Time
	for {
		missing, err := firstMissing(ctx, pool, tables)
		if err != nil {
			return fmt.Errorf("wait for tables: %w", err)
		}
		if missing == "" {
			return nil
		}
		if missing != reported || time.Since(reportedAt) >= waitReportEvery {
			logger.InfoContext(ctx, "waiting for table", "table", missing)
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

func firstMissing(ctx context.Context, pool *pgxpool.Pool, tables []string) (string, error) {
	for _, table := range tables {
		var present bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&present); err != nil {
			return "", err
		}
		if !present {
			return table, nil
		}
	}
	return "", nil
}
