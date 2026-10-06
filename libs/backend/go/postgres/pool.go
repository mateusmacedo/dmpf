package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.43.0/dbconv"
	"go.opentelemetry.io/otel/trace"
)

const meterName = "github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

var durationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

type PoolOption func(*poolOptions)

type poolOptions struct{ meterProvider metric.MeterProvider }

func WithMeterProvider(provider metric.MeterProvider) PoolOption {
	return func(o *poolOptions) { o.meterProvider = provider }
}

// NewPool builds the pool with the query tracer of the process. It does not
// reach the database: readiness is the ping that follows.
func NewPool(ctx context.Context, dsn string, tracer trace.Tracer, opts ...PoolOption) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	var options poolOptions
	for _, opt := range opts {
		opt(&options)
	}
	queries := newEndpointTracer(tracer, config.ConnConfig)
	config.ConnConfig.Tracer = queries
	if options.meterProvider == nil {
		return pgxpool.NewWithConfig(ctx, config)
	}

	meter := options.meterProvider.Meter(meterName)
	instruments, err := newPoolInstruments(meter, poolNameOf(config.ConnConfig))
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = meteredTracer{QueryTracer: queries, instruments: instruments}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := instruments.observe(meter, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func DescribeDSN(dsn string) slog.Value {
	config, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return slog.StringValue("invalid")
	}
	user := "unset"
	if userDeclared(dsn) {
		user = "set"
	}
	return slog.GroupValue(
		slog.String("host", config.Host),
		slog.Int("port", int(config.Port)),
		slog.String("database", config.Database),
		slog.String("sslmode", sslModeOf(dsn)),
		slog.String("user", user),
	)
}

// WHY: pgconn turns sslmode into a tls.Config and keeps no trace of the
// declared mode (pgconn@v5.10.0/config.go:778-788, default "prefer").
func sslModeOf(dsn string) string {
	mode := os.Getenv("PGSSLMODE")
	if parsed, err := url.Parse(dsn); err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		if declared := parsed.Query().Get("sslmode"); declared != "" {
			mode = declared
		}
	} else {
		for _, field := range strings.Fields(dsn) {
			if declared, found := strings.CutPrefix(field, "sslmode="); found {
				mode = strings.Trim(declared, "'")
			}
		}
	}
	if mode == "" {
		return "prefer"
	}
	return mode
}

func userDeclared(dsn string) bool {
	if os.Getenv("PGUSER") != "" {
		return true
	}
	if parsed, err := url.Parse(dsn); err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		return parsed.User.Username() != "" || parsed.Query().Get("user") != ""
	}
	for _, field := range strings.Fields(dsn) {
		if declared, found := strings.CutPrefix(field, "user="); found && strings.Trim(declared, "'") != "" {
			return true
		}
	}
	return false
}

func poolNameOf(conn *pgx.ConnConfig) string {
	return conn.Host + ":" + strconv.Itoa(int(conn.Port)) + "/" + conn.Database
}

type poolInstruments struct {
	name            string
	count           dbconv.ClientConnectionCountObservable
	max             dbconv.ClientConnectionMaxObservable
	pendingRequests dbconv.ClientConnectionPendingRequestsObservable
	timeouts        dbconv.ClientConnectionTimeoutsObservable
	waitTime        dbconv.ClientConnectionWaitTime
	pending         *atomic.Int64
}

func newPoolInstruments(meter metric.Meter, name string) (poolInstruments, error) {
	count, errCount := dbconv.NewClientConnectionCountObservable(meter)
	maxConns, errMax := dbconv.NewClientConnectionMaxObservable(meter)
	pendingRequests, errPending := dbconv.NewClientConnectionPendingRequestsObservable(meter)
	timeouts, errTimeouts := dbconv.NewClientConnectionTimeoutsObservable(meter)
	waitTime, errWait := dbconv.NewClientConnectionWaitTime(meter, metric.WithExplicitBucketBoundaries(durationBoundaries...))
	if err := errors.Join(errCount, errMax, errPending, errTimeouts, errWait); err != nil {
		return poolInstruments{}, fmt.Errorf("pool instruments: %w", err)
	}
	return poolInstruments{
		name:            name,
		count:           count,
		max:             maxConns,
		pendingRequests: pendingRequests,
		timeouts:        timeouts,
		waitTime:        waitTime,
		pending:         new(atomic.Int64),
	}, nil
}

func (p poolInstruments) observe(meter metric.Meter, pool *pgxpool.Pool) error {
	named := metric.WithAttributes(p.count.AttrClientConnectionPoolName(p.name))
	idle := metric.WithAttributes(p.count.AttrClientConnectionPoolName(p.name), p.count.AttrClientConnectionState(dbconv.ClientConnectionStateIdle))
	used := metric.WithAttributes(p.count.AttrClientConnectionPoolName(p.name), p.count.AttrClientConnectionState(dbconv.ClientConnectionStateUsed))
	_, err := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		stat := pool.Stat()
		o.ObserveInt64(p.count.Inst(), int64(stat.IdleConns()), idle)
		o.ObserveInt64(p.count.Inst(), int64(stat.AcquiredConns()), used)
		o.ObserveInt64(p.max.Inst(), int64(stat.MaxConns()), named)
		o.ObserveInt64(p.pendingRequests.Inst(), p.pending.Load(), named)
		o.ObserveInt64(p.timeouts.Inst(), stat.CanceledAcquireCount(), named)
		return nil
	}, p.count.Inst(), p.max.Inst(), p.pendingRequests.Inst(), p.timeouts.Inst())
	return err
}

type acquireStartedKey struct{}

type meteredTracer struct {
	pgx.QueryTracer
	instruments poolInstruments
}

func (t meteredTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	t.instruments.pending.Add(1)
	return context.WithValue(ctx, acquireStartedKey{}, time.Now())
}

func (t meteredTracer) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	t.instruments.pending.Add(-1)
	started, ok := ctx.Value(acquireStartedKey{}).(time.Time)
	if !ok || data.Err != nil {
		return
	}
	t.instruments.waitTime.Record(ctx, time.Since(started).Seconds(), t.instruments.name)
}

const selectForeignDestination = `SELECT destination FROM outbox
		WHERE status IN ('pending', 'publishing') AND destination <> ALL($1)
		LIMIT 1`

var _ = declare(selectForeignDestination, "SELECT", "outbox")

// AssertOwnOutbox refuses a database whose outbox holds a destination the
// caller does not publish, because the relay drains without filtering by
// destination and would carry away another context's record.
//
// It takes the destinations rather than the channel catalogue so the storage
// provider does not depend on the transport module to read the keys of a map.
func AssertOwnOutbox(ctx context.Context, pool *pgxpool.Pool, destinations []string) error {
	var foreign string
	switch err := pool.QueryRow(ctx, selectForeignDestination, destinations).Scan(&foreign); {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("outbox destinations: %w", err)
	default:
		return fmt.Errorf("outbox holds %q, which this context does not publish: the database is shared with another context", foreign)
	}
}
