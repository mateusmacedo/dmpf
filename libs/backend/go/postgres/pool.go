package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
)

// NewPool builds the pool with the query tracer of the process. It does not
// reach the database: readiness is the ping that follows.
func NewPool(ctx context.Context, dsn string, tracer trace.Tracer) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = NewQueryTracer(tracer)
	return pgxpool.NewWithConfig(ctx, config)
}

// AssertOwnOutbox refuses a database whose outbox holds a destination the
// caller does not publish, because the relay drains without filtering by
// destination and would carry away another context's record.
//
// It takes the destinations rather than the channel catalogue so the storage
// provider does not depend on the transport module to read the keys of a map.
func AssertOwnOutbox(ctx context.Context, pool *pgxpool.Pool, destinations []string) error {
	const query = `SELECT destination FROM dmpf_outbox
		WHERE status IN ('pending', 'publishing') AND destination <> ALL($1)
		LIMIT 1`

	var foreign string
	switch err := pool.QueryRow(ctx, query, destinations).Scan(&foreign); {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("outbox destinations: %w", err)
	default:
		return fmt.Errorf("outbox holds %q, which this context does not publish: the database is shared with another context", foreign)
	}
}
