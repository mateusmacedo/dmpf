package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const tracedSQL = "SELECT secret FROM t WHERE id = $1"

func traceStatement(t *testing.T, newTracer func(trace.Tracer) pgx.QueryTracer, sql string, end pgx.TraceQueryEndData) (tracetest.SpanStub, trace.SpanContext) {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	parentCtx, parent := provider.Tracer("db-test").Start(context.Background(), "parent")
	tracer := newTracer(provider.Tracer("db-test"))
	ctx := tracer.TraceQueryStart(parentCtx, nil, pgx.TraceQueryStartData{SQL: sql, Args: []any{"p-1"}})
	tracer.TraceQueryEnd(ctx, nil, end)
	parent.End()

	for _, span := range spans.GetSpans() {
		if span.Name != "parent" {
			return span, parent.SpanContext()
		}
	}
	t.Fatalf("no query span among %d", len(spans.GetSpans()))
	return tracetest.SpanStub{}, trace.SpanContext{}
}

func tracedQuery(t *testing.T, end pgx.TraceQueryEndData) (tracetest.SpanStub, trace.SpanContext) {
	t.Helper()
	return traceStatement(t, NewQueryTracer, tracedSQL, end)
}

func attributeOf(span tracetest.SpanStub, key string) (attribute.Value, bool) {
	for _, kv := range span.Attributes {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func requireString(t *testing.T, span tracetest.SpanStub, key, want string) {
	t.Helper()
	if got, ok := attributeOf(span, key); !ok || got.AsString() != want {
		t.Fatalf("%s = %q (present %v), want %q; attributes %v", key, got.String(), ok, want, span.Attributes)
	}
}

func requireInt(t *testing.T, span tracetest.SpanStub, key string, want int64) {
	t.Helper()
	if got, ok := attributeOf(span, key); !ok || got.AsInt64() != want {
		t.Fatalf("%s = %s (present %v), want %d; attributes %v", key, got.String(), ok, want, span.Attributes)
	}
}

func requireAbsent(t *testing.T, span tracetest.SpanStub, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if got, ok := attributeOf(span, key); ok {
			t.Fatalf("%s = %q, want absent; attributes %v", key, got.String(), span.Attributes)
		}
	}
}

func requireNoAttributeContains(t *testing.T, span tracetest.SpanStub, fragments ...string) {
	t.Helper()
	for _, kv := range span.Attributes {
		for _, fragment := range fragments {
			if strings.Contains(kv.Value.String(), fragment) {
				t.Fatalf("attribute %s = %q carries %q", kv.Key, kv.Value.String(), fragment)
			}
		}
	}
	if strings.Contains(span.Name, "p-1") {
		t.Fatalf("name %q carries an argument", span.Name)
	}
}

func TestTheQueryTracerRecordsTheParameterizedTextWithoutTheArguments(t *testing.T) {
	span, parent := tracedQuery(t, pgx.TraceQueryEndData{})

	if span.SpanKind != trace.SpanKindClient {
		t.Fatalf("kind = %v, want client", span.SpanKind)
	}
	if span.Parent.SpanID() != parent.SpanID() {
		t.Fatalf("parent = %s, want %s", span.Parent.SpanID(), parent.SpanID())
	}
	requireString(t, span, "db.system.name", "postgresql")
	requireString(t, span, "db.query.text", tracedSQL)
	requireNoAttributeContains(t, span, "p-1")
	requireAbsent(t, span, "dmpf.dependency")
	if span.Status.Code != codes.Unset {
		t.Fatalf("status = %v, want unset", span.Status.Code)
	}
}

func TestAQueryOutsideATracedOperationOpensNoSpan(t *testing.T) {
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	tracer := NewQueryTracer(provider.Tracer("db-test"))

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	if got := len(spans.GetSpans()); got != 0 {
		t.Fatalf("spans = %d, want 0 — a poll without a parent is not traced", got)
	}
}

func TestAFailureWithoutSQLSTATEIsTypedOtherWithoutTheMessage(t *testing.T) {
	span, _ := tracedQuery(t, pgx.TraceQueryEndData{Err: errors.New("boom: password=hunter2")})

	if span.Status.Code != codes.Error || span.Status.Description != "" {
		t.Fatalf("status = %+v, want error without description", span.Status)
	}
	requireString(t, span, "error.type", "_OTHER")
	requireAbsent(t, span, "db.response.status_code")
	requireNoAttributeContains(t, span, "hunter2", "boom")
}

func TestAServerErrorIsTypedByItsSQLSTATEAlone(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:    "23505",
		Message: "duplicate key value violates unique constraint",
		Detail:  "Key (message_id)=(m-1) already exists.",
		Hint:    "hint-text",
	}
	span, _ := tracedQuery(t, pgx.TraceQueryEndData{Err: fmt.Errorf("insert: %w", pgErr)})

	if span.Status.Code != codes.Error || span.Status.Description != "" {
		t.Fatalf("status = %+v, want error without description", span.Status)
	}
	requireString(t, span, "db.response.status_code", "23505")
	requireString(t, span, "error.type", "23505")
	requireNoAttributeContains(t, span, "m-1", "duplicate", "hint-text")
}

func TestTransactionControlCarriesOnlyItsOperation(t *testing.T) {
	for sql, want := range map[string]string{
		"begin isolation level read committed": "BEGIN",
		"commit":                               "COMMIT",
		"rollback":                             "ROLLBACK",
	} {
		t.Run(want, func(t *testing.T) {
			span, _ := traceStatement(t, NewQueryTracer, sql, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag(want)})

			if span.Name != want {
				t.Fatalf("name = %q, want %q", span.Name, want)
			}
			requireString(t, span, "db.operation.name", want)
			requireAbsent(t, span, "db.query.text", "db.collection.name", "dmpf.db.rows_affected", "db.response.returned_rows")
		})
	}
}

func TestADeclaredStatementNamesTheSpanByOperationAndCollection(t *testing.T) {
	span, _ := traceStatement(t, NewQueryTracer, claimStatement, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag("UPDATE 2")})

	if span.Name != "UPDATE outbox" {
		t.Fatalf("name = %q, want %q", span.Name, "UPDATE outbox")
	}
	requireString(t, span, "db.operation.name", "UPDATE")
	requireString(t, span, "db.collection.name", "outbox")
	requireString(t, span, "db.query.text", claimStatement)
}

func TestAStatementWithoutCollectionIsNamedByItsOperation(t *testing.T) {
	span, _ := traceStatement(t, NewQueryTracer, selectTableExists, pgx.TraceQueryEndData{})

	if span.Name != "SELECT" {
		t.Fatalf("name = %q, want SELECT", span.Name)
	}
	requireString(t, span, "db.operation.name", "SELECT")
	requireAbsent(t, span, "db.collection.name")
}

func TestAnUndeclaredStatementIsNamedAfterTheSystem(t *testing.T) {
	span, _ := traceStatement(t, NewQueryTracer, "CREATE TABLE x (id int); DROP TABLE y", pgx.TraceQueryEndData{})

	if span.Name != "postgresql" {
		t.Fatalf("name = %q, want postgresql", span.Name)
	}
	requireAbsent(t, span, "db.operation.name", "db.collection.name")
}

func TestTheCommandTagCountsRowsByKind(t *testing.T) {
	cases := []struct {
		tag  string
		key  string
		want int64
	}{
		{"SELECT 3", "db.response.returned_rows", 3},
		{"UPDATE 0", "dmpf.db.rows_affected", 0},
		{"INSERT 0 1", "dmpf.db.rows_affected", 1},
		{"DELETE 7", "dmpf.db.rows_affected", 7},
	}
	for _, c := range cases {
		t.Run(c.tag, func(t *testing.T) {
			span, _ := tracedQuery(t, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag(c.tag)})

			requireInt(t, span, c.key, c.want)
			other := map[string]string{"db.response.returned_rows": "dmpf.db.rows_affected", "dmpf.db.rows_affected": "db.response.returned_rows"}[c.key]
			requireAbsent(t, span, other)
		})
	}
}

func TestThePoolGivesTheTracerItsEndpointWithoutTheCredentials(t *testing.T) {
	const dsn = "postgres://dmpf_user:hunter2@db.example:6543/orders?sslmode=disable"
	for name, opts := range map[string][]PoolOption{
		"plain":   nil,
		"metered": {WithMeterProvider(metricnoop.NewMeterProvider())},
	} {
		t.Run(name, func(t *testing.T) {
			span, _ := traceStatement(t, func(tracer trace.Tracer) pgx.QueryTracer {
				pool, err := NewPool(context.Background(), dsn, tracer, opts...)
				if err != nil {
					t.Fatalf("NewPool() = %v", err)
				}
				t.Cleanup(pool.Close)
				return pool.Config().ConnConfig.Tracer
			}, tracedSQL, pgx.TraceQueryEndData{})

			requireString(t, span, "db.namespace", "orders")
			requireString(t, span, "server.address", "db.example")
			requireInt(t, span, "server.port", 6543)
			requireNoAttributeContains(t, span, "hunter2", "dmpf_user", "sslmode")
		})
	}
}

func TestADSNWithoutDatabaseLeavesTheNamespaceOut(t *testing.T) {
	span, _ := traceStatement(t, func(tracer trace.Tracer) pgx.QueryTracer {
		pool, err := NewPool(context.Background(), "postgres://dmpf_user:hunter2@db.example:6543/?sslmode=disable", tracer)
		if err != nil {
			t.Fatalf("NewPool() = %v", err)
		}
		t.Cleanup(pool.Close)
		return pool.Config().ConnConfig.Tracer
	}, tracedSQL, pgx.TraceQueryEndData{})

	requireAbsent(t, span, "db.namespace")
	requireString(t, span, "server.address", "db.example")
	requireNoAttributeContains(t, span, "hunter2", "dmpf_user")
}

func TestEveryKernelStatementIsDeclared(t *testing.T) {
	for name, sql := range map[string]string{
		"claim":            claimStatement,
		"markPublished":    markPublishedStatement,
		"reschedule":       rescheduleStatement,
		"fail":             failStatement,
		"insertOutbox":     insertOutbox,
		"insertInbox":      insertInbox,
		"insertCommand":    insertCommand,
		"selectInbox":      selectInbox,
		"updateInbox":      updateInbox,
		"resetLock":        resetLockTimeout,
		"insertQuarantine": insertQuarantine,
		"deletePublished":  deletePublished,
		"deleteInbox":      deleteInbox,
		"deleteExpired":    deleteExpiredInbox,
		"selectSignals":    selectSignals,
		"selectOutbox":     selectOutboxSignals,
		"tableExists":      selectTableExists,
		"assertOwnOutbox":  selectForeignDestination,
	} {
		if _, ok := shapeOf(sql); !ok {
			t.Errorf("%s is not declared: its span would carry neither operation nor collection", name)
		}
	}
}

func TestTheLockTimeoutIsDeclaredAsASet(t *testing.T) {
	shape, ok := shapeOf(lockTimeoutStatement(1500))
	if !ok || shape.operation != "SET" || shape.collection != "" {
		t.Fatalf("shape = %+v (declared %v), want SET without collection", shape, ok)
	}
}

func TestATableDeclaresTheStatementsItCompiles(t *testing.T) {
	table := Table[string, string]{
		Name: "orders", IDColumn: "order_id", Columns: []string{"snapshot"},
		Encode: func(string) ([]any, error) { return nil, nil },
		Decode: func(func(...any) error) (string, error) { return "", nil },
	}
	compiled := table.compile()
	relation := table.Relation("snapshot")

	for sql, want := range map[string]statementShape{
		compiled.selectOne: {operation: "SELECT", collection: "orders"},
		compiled.owner:     {operation: "SELECT", collection: "orders"},
		compiled.insert:    {operation: "INSERT", collection: "orders"},
		compiled.update:    {operation: "UPDATE", collection: "orders"},
		relation.statement: {operation: "SELECT", collection: "orders"},
		relation.owner:     {operation: "SELECT", collection: "orders"},
	} {
		if got, ok := shapeOf(sql); !ok || got != want {
			t.Fatalf("shape of %q = %+v (declared %v), want %+v", sql, got, ok, want)
		}
	}
}
