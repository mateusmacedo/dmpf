package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const keyRowsAffected = "dmpf.db.rows_affected"

type dbSpanKey struct{}

type statementShape struct{ operation, collection string }

var declaredStatements sync.Map

// declare names the operation and the collection of a statement by its text,
// because semconv forbids deriving them from SQL that holds more than one.
func declare(sql, operation, collection string) string {
	if _, known := declaredStatements.Load(sql); !known {
		declaredStatements.Store(sql, statementShape{operation: operation, collection: collection})
	}
	return sql
}

func shapeOf(sql string) (statementShape, bool) {
	shape, ok := declaredStatements.Load(sql)
	if !ok {
		return statementShape{}, false
	}
	return shape.(statementShape), true
}

// transactionControl matches the text pgx sends for Begin, Commit and Rollback,
// and for the savepoint a nested Begin opens (tx.go:171,309,322): the partial
// undo must not read as the transaction's ROLLBACK, nor carry its sp_N name.
func transactionControl(sql string) (string, bool) {
	first, rest, _ := strings.Cut(strings.TrimSpace(sql), " ")
	switch operation := strings.ToUpper(first); operation {
	case "SAVEPOINT":
		return operation, true
	case "RELEASE":
		return "RELEASE SAVEPOINT", true
	case "ROLLBACK":
		if next, _, _ := strings.Cut(strings.TrimSpace(rest), " "); strings.EqualFold(next, "TO") {
			return "ROLLBACK TO SAVEPOINT", true
		}
		return operation, true
	case "BEGIN", "COMMIT":
		return operation, true
	}
	return "", false
}

// NewQueryTracer opens a client span per query inside a traced operation only:
// the relay's claim loop polls without a parent, and a root span per poll is
// noise.
func NewQueryTracer(tracer trace.Tracer) pgx.QueryTracer { return dbTracer{tracer: tracer} }

func newEndpointTracer(tracer trace.Tracer, conn *pgx.ConnConfig) dbTracer {
	endpoint := []attribute.KeyValue{semconv.ServerAddress(conn.Host), semconv.ServerPort(int(conn.Port))}
	if conn.Database != "" {
		endpoint = append(endpoint, semconv.DBNamespace(conn.Database))
	}
	return dbTracer{tracer: tracer, endpoint: endpoint}
}

type dbTracer struct {
	tracer   trace.Tracer
	endpoint []attribute.KeyValue
}

func (d dbTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	ctx, span := d.start(ctx, data.SQL)
	return context.WithValue(ctx, dbSpanKey{}, span)
}

func (d dbTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(dbSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	finish(span, data.CommandTag, data.Err)
}

type dbBatchKey struct{}

// batchSpans starts every statement at the send: the server answers only after
// the one Sync, so the statements of a batch are in flight together (conn.go:1247).
type batchSpans struct {
	queued   []*pgx.QueuedQuery
	sent     time.Time
	reported int
	failed   bool
}

func (d dbTracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchStartData) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return context.WithValue(ctx, dbBatchKey{}, &batchSpans{queued: data.Batch.QueuedQueries, sent: time.Now()})
}

func (d dbTracer) TraceBatchQuery(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchQueryData) {
	batch, ok := ctx.Value(dbBatchKey{}).(*batchSpans)
	if !ok {
		return
	}
	d.traceBatched(ctx, batch, data.SQL, data.CommandTag, data.Err)
}

// TraceBatchEnd gives a server error to the first statement without a result, as
// the server skips the rest until the Sync; any other failure ends every statement
// still in flight, since the client cannot tell which one the server was running.
func (d dbTracer) TraceBatchEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchEndData) {
	batch, ok := ctx.Value(dbBatchKey{}).(*batchSpans)
	if !ok || data.Err == nil || batch.failed {
		return
	}
	for !batch.failed && batch.reported < len(batch.queued) {
		d.traceBatched(ctx, batch, batch.queued[batch.reported].SQL, pgconn.CommandTag{}, data.Err)
	}
}

func (d dbTracer) traceBatched(ctx context.Context, batch *batchSpans, sql string, tag pgconn.CommandTag, err error) {
	_, span := d.start(ctx, sql, trace.WithTimestamp(batch.sent))
	batch.reported++
	_, refused := errors.AsType[*pgconn.PgError](err)
	batch.failed = batch.failed || refused
	finish(span, tag, err)
}

func (d dbTracer) start(ctx context.Context, sql string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	attributes := append([]attribute.KeyValue{semconv.DBSystemNamePostgreSQL}, d.endpoint...)
	name := semconv.DBSystemNamePostgreSQL.Value.AsString()
	if operation, ok := transactionControl(sql); ok {
		name = operation
		attributes = append(attributes, semconv.DBOperationName(operation))
	} else {
		attributes = append(attributes, semconv.DBQueryText(sql))
		if shape, declared := shapeOf(sql); declared && shape.operation != "" {
			name = shape.operation
			attributes = append(attributes, semconv.DBOperationName(shape.operation))
			if shape.collection != "" {
				name += " " + shape.collection
				attributes = append(attributes, semconv.DBCollectionName(shape.collection))
			}
		}
	}
	opts = append(opts, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attributes...))
	return d.tracer.Start(ctx, name, opts...)
}

func finish(span trace.Span, tag pgconn.CommandTag, err error) {
	switch {
	case tag.Select():
		span.SetAttributes(semconv.DBResponseReturnedRows(int(tag.RowsAffected())))
	case tag.Insert(), tag.Update(), tag.Delete():
		span.SetAttributes(attribute.Int64(keyRowsAffected, tag.RowsAffected()))
	}
	if err != nil {
		recordQueryError(span, err)
	}
	span.End()
}

// recordQueryError keeps the SQLSTATE alone: Message, Detail and Hint quote the
// values of the offending row.
func recordQueryError(span trace.Span, err error) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code == "" {
		tracing.RecordError(span, semconv.ErrorTypeOther.Value.AsString())
		return
	}
	span.SetAttributes(semconv.DBResponseStatusCode(pgErr.Code))
	tracing.RecordError(span, pgErr.Code)
}
