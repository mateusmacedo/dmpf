package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"

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

// transactionControl matches the text pgx sends for Begin, Commit and Rollback.
func transactionControl(sql string) (string, bool) {
	first, _, _ := strings.Cut(strings.TrimSpace(sql), " ")
	switch operation := strings.ToUpper(first); operation {
	case "BEGIN", "COMMIT", "ROLLBACK":
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
	attributes := append([]attribute.KeyValue{semconv.DBSystemNamePostgreSQL}, d.endpoint...)
	name := semconv.DBSystemNamePostgreSQL.Value.AsString()
	if operation, ok := transactionControl(data.SQL); ok {
		name = operation
		attributes = append(attributes, semconv.DBOperationName(operation))
	} else {
		attributes = append(attributes, semconv.DBQueryText(data.SQL))
		if shape, declared := shapeOf(data.SQL); declared && shape.operation != "" {
			name = shape.operation
			attributes = append(attributes, semconv.DBOperationName(shape.operation))
			if shape.collection != "" {
				name += " " + shape.collection
				attributes = append(attributes, semconv.DBCollectionName(shape.collection))
			}
		}
	}
	ctx, span := d.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attributes...))
	return context.WithValue(ctx, dbSpanKey{}, span)
}

func (d dbTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(dbSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	switch tag := data.CommandTag; {
	case tag.Select():
		span.SetAttributes(semconv.DBResponseReturnedRows(int(tag.RowsAffected())))
	case tag.Insert(), tag.Update(), tag.Delete():
		span.SetAttributes(attribute.Int64(keyRowsAffected, tag.RowsAffected()))
	}
	if data.Err != nil {
		recordQueryError(span, data.Err)
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
