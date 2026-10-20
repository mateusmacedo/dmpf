package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

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
		"savepoint sp_1":                       "SAVEPOINT",
		"release savepoint sp_1":               "RELEASE SAVEPOINT",
		"rollback to savepoint sp_1":           "ROLLBACK TO SAVEPOINT",
		"ROLLBACK TO sp_1":                     "ROLLBACK TO SAVEPOINT",
		"release sp_1":                         "RELEASE SAVEPOINT",
	} {
		t.Run(want, func(t *testing.T) {
			span, _ := traceStatement(t, NewQueryTracer, sql, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag(want)})

			if span.Name != want {
				t.Fatalf("name = %q, want %q", span.Name, want)
			}
			requireString(t, span, "db.operation.name", want)
			requireAbsent(t, span, "db.query.text", "db.collection.name", "dmpf.db.rows_affected", "db.response.returned_rows")
			requireNoAttributeContains(t, span, "sp_1")
			if strings.Contains(span.Name, "sp_1") {
				t.Fatalf("name %q carries the savepoint pgx numbered", span.Name)
			}
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
	span, _ := traceStatement(t, NewQueryTracer, selectFirstMissingTable, pgx.TraceQueryEndData{})

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
		"firstMissing":     selectFirstMissingTable,
		"firstUngranted":   selectFirstUngranted,
		"assertOwnOutbox":  selectForeignDestination,
		"divergentUnique":  selectFirstDivergentUnique,
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
		compiled.exists:    {operation: "SELECT", collection: "orders"},
		relation.statement: {operation: "SELECT", collection: "orders"},
		relation.owner:     {operation: "SELECT", collection: "orders"},
	} {
		if got, ok := shapeOf(sql); !ok || got != want {
			t.Fatalf("shape of %q = %+v (declared %v), want %+v", sql, got, ok, want)
		}
	}
}

type batchStep struct {
	sql   string
	after time.Duration
	end   pgx.TraceBatchQueryData
}

type batchWindow struct{ before, sent time.Time }

func traceBatch(t *testing.T, tracer pgx.QueryTracer, parentTracer trace.Tracer, queued []string, steps []batchStep, ends ...error) batchWindow {
	t.Helper()
	batches, ok := tracer.(pgx.BatchTracer)
	if !ok {
		t.Fatalf("%T is not a pgx.BatchTracer: a Save under the savepoint would leave no span", tracer)
	}
	ctx := context.Background()
	if parentTracer != nil {
		var parent trace.Span
		ctx, parent = parentTracer.Start(ctx, "parent")
		defer parent.End()
	}
	batch := &pgx.Batch{}
	for _, sql := range queued {
		batch.Queue(sql)
	}
	var window batchWindow
	window.before = time.Now()
	ctx = batches.TraceBatchStart(ctx, nil, pgx.TraceBatchStartData{Batch: batch})
	window.sent = time.Now()
	for _, step := range steps {
		time.Sleep(step.after)
		step.end.SQL = step.sql
		batches.TraceBatchQuery(ctx, nil, step.end)
	}
	for _, err := range ends {
		batches.TraceBatchEnd(ctx, nil, pgx.TraceBatchEndData{Err: err})
	}
	return window
}

func batchExporter(t *testing.T) (*tracetest.InMemoryExporter, trace.Tracer) {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return spans, provider.Tracer("db-test")
}

func queriesOf(spans *tracetest.InMemoryExporter) []tracetest.SpanStub {
	var out []tracetest.SpanStub
	for _, span := range spans.GetSpans() {
		if span.Name != "parent" {
			out = append(out, span)
		}
	}
	return out
}

func namesOf(spans []tracetest.SpanStub) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}
	return names
}

func requireSentWith(t *testing.T, window batchWindow, spans []tracetest.SpanStub) {
	t.Helper()
	for _, span := range spans {
		if span.StartTime.Before(window.before) || span.StartTime.After(window.sent) {
			t.Fatalf("span %s starts at %v, want the send of the batch, within [%v, %v]", span.Name, span.StartTime, window.before, window.sent)
		}
	}
}

func requireFailedWith(t *testing.T, span tracetest.SpanStub, errorType string) {
	t.Helper()
	if span.Status.Code != codes.Error || span.Status.Description != "" {
		t.Fatalf("span %s status = %+v, want error without description", span.Name, span.Status)
	}
	requireString(t, span, "error.type", errorType)
}

var savepointBatch = []string{"SAVEPOINT dmpf_unique", claimStatement, "RELEASE SAVEPOINT dmpf_unique"}

const resultsAfterTheSync = 20 * time.Millisecond

func TestEveryStatementOfABatchIsInFlightFromTheSendUntilItsResult(t *testing.T) {
	spans, tracer := batchExporter(t)
	tags := []string{"SAVEPOINT", "UPDATE 1", "RELEASE"}

	window := traceBatch(t, NewQueryTracer(tracer), tracer, savepointBatch, []batchStep{
		{savepointBatch[0], resultsAfterTheSync, pgx.TraceBatchQueryData{CommandTag: pgconn.NewCommandTag(tags[0])}},
		{savepointBatch[1], 0, pgx.TraceBatchQueryData{Args: []any{"p-1"}, CommandTag: pgconn.NewCommandTag(tags[1])}},
		{savepointBatch[2], 0, pgx.TraceBatchQueryData{CommandTag: pgconn.NewCommandTag(tags[2])}},
	}, nil)

	got := queriesOf(spans)
	if names := namesOf(got); !slices.Equal(names, []string{"SAVEPOINT", "UPDATE outbox", "RELEASE SAVEPOINT"}) {
		t.Fatalf("spans = %v, want one per statement", names)
	}
	requireSentWith(t, window, got)
	parent := spans.GetSpans()[len(spans.GetSpans())-1]
	for n, span := range got {
		if span.SpanKind != trace.SpanKindClient || span.Parent.SpanID() != parent.SpanContext.SpanID() {
			t.Fatalf("span %s kind %v parent %s, want a client span under %s", span.Name, span.SpanKind, span.Parent.SpanID(), parent.SpanContext.SpanID())
		}
		if took := span.EndTime.Sub(span.StartTime); took < resultsAfterTheSync {
			t.Fatalf("span %s took %v, want at least the %v until its result was read", span.Name, took, resultsAfterTheSync)
		}
		alone, _ := traceStatement(t, NewQueryTracer, savepointBatch[n], pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag(tags[n])})
		if span.Name != alone.Name || !slices.Equal(span.Attributes, alone.Attributes) {
			t.Fatalf("batched %s %v, want what the statement sent alone carries: %s %v", span.Name, span.Attributes, alone.Name, alone.Attributes)
		}
		requireNoAttributeContains(t, span, "p-1", "dmpf_unique")
	}
	requireInt(t, got[1], "dmpf.db.rows_affected", 1)
}

func TestAStatementThatFailsInABatchCarriesItsSQLSTATEAndEndsTheSpans(t *testing.T) {
	spans, tracer := batchExporter(t)
	violation := fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", Detail: "Key (name)=(p-1) already exists."})

	window := traceBatch(t, NewQueryTracer(tracer), tracer, savepointBatch, []batchStep{
		{savepointBatch[0], 0, pgx.TraceBatchQueryData{CommandTag: pgconn.NewCommandTag("SAVEPOINT")}},
		{savepointBatch[1], 0, pgx.TraceBatchQueryData{Err: violation}},
	}, violation, violation)

	got := queriesOf(spans)
	if names := namesOf(got); !slices.Equal(names, []string{"SAVEPOINT", "UPDATE outbox"}) {
		t.Fatalf("spans = %v, want the RELEASE the server skipped left out", names)
	}
	requireSentWith(t, window, got)
	requireFailedWith(t, got[1], "23505")
	requireString(t, got[1], "db.response.status_code", "23505")
	requireNoAttributeContains(t, got[1], "p-1", "already exists")
	if got[0].Status.Code != codes.Unset {
		t.Fatalf("SAVEPOINT status = %+v, want unset: its own result came back", got[0].Status)
	}
}

func TestAServerErrorThePipelineReportsAtTheEndGoesToTheFirstStatementWithoutAResult(t *testing.T) {
	spans, tracer := batchExporter(t)
	violation := &pgconn.PgError{Code: "23505", Detail: "Key (name)=(p-1) already exists."}

	traceBatch(t, NewQueryTracer(tracer), tracer, savepointBatch, []batchStep{
		{savepointBatch[0], 0, pgx.TraceBatchQueryData{CommandTag: pgconn.NewCommandTag("SAVEPOINT")}},
	}, violation, violation)

	got := queriesOf(spans)
	if names := namesOf(got); !slices.Equal(names, []string{"SAVEPOINT", "UPDATE outbox"}) {
		t.Fatalf("spans = %v, want the error on the UPDATE and the RELEASE the server skipped left out", names)
	}
	requireFailedWith(t, got[1], "23505")
	requireNoAttributeContains(t, got[1], "p-1", "already exists")
}

func TestABatchThatFailsBeforeAnyResultEndsEveryStatementWithIt(t *testing.T) {
	spans, tracer := batchExporter(t)

	window := traceBatch(t, NewQueryTracer(tracer), tracer, savepointBatch, nil, context.DeadlineExceeded, context.DeadlineExceeded)

	got := queriesOf(spans)
	if names := namesOf(got); !slices.Equal(names, []string{"SAVEPOINT", "UPDATE outbox", "RELEASE SAVEPOINT"}) {
		t.Fatalf("spans = %v, want every statement in flight ended once: the client cannot tell which one ran", names)
	}
	requireSentWith(t, window, got)
	for _, span := range got {
		requireFailedWith(t, span, "_OTHER")
	}
}

func TestABatchThatFailsAfterAResultEndsTheStatementsStillInFlight(t *testing.T) {
	spans, tracer := batchExporter(t)
	lost := errors.New("read tcp: connection reset by peer")

	traceBatch(t, NewQueryTracer(tracer), tracer, savepointBatch, []batchStep{
		{savepointBatch[0], 0, pgx.TraceBatchQueryData{CommandTag: pgconn.NewCommandTag("SAVEPOINT")}},
	}, lost, lost)

	got := queriesOf(spans)
	if names := namesOf(got); !slices.Equal(names, []string{"SAVEPOINT", "UPDATE outbox", "RELEASE SAVEPOINT"}) {
		t.Fatalf("spans = %v, want one per statement", names)
	}
	if got[0].Status.Code != codes.Unset {
		t.Fatalf("SAVEPOINT status = %+v, want unset: its result was read", got[0].Status)
	}
	requireFailedWith(t, got[1], "_OTHER")
	requireFailedWith(t, got[2], "_OTHER")
}

func TestAStatementThatFailsWithoutTheServerEndsTheStatementsStillInFlight(t *testing.T) {
	spans, tracer := batchExporter(t)
	lost := errors.New("read tcp: i/o timeout")

	traceBatch(t, NewQueryTracer(tracer), tracer, savepointBatch, []batchStep{
		{savepointBatch[0], 0, pgx.TraceBatchQueryData{CommandTag: pgconn.NewCommandTag("SAVEPOINT")}},
		{savepointBatch[1], 0, pgx.TraceBatchQueryData{Err: lost}},
	}, lost, lost)

	got := queriesOf(spans)
	if names := namesOf(got); !slices.Equal(names, []string{"SAVEPOINT", "UPDATE outbox", "RELEASE SAVEPOINT"}) {
		t.Fatalf("spans = %v, want the RELEASE ended too: no server skipped it", names)
	}
	if got[0].Status.Code != codes.Unset {
		t.Fatalf("SAVEPOINT status = %+v, want unset: its result was read", got[0].Status)
	}
	requireFailedWith(t, got[1], "_OTHER")
	requireFailedWith(t, got[2], "_OTHER")
}

func TestABatchOutsideATracedOperationOpensNoSpan(t *testing.T) {
	spans, tracer := batchExporter(t)

	traceBatch(t, NewQueryTracer(tracer), nil, savepointBatch, []batchStep{
		{savepointBatch[0], 0, pgx.TraceBatchQueryData{}},
		{savepointBatch[1], 0, pgx.TraceBatchQueryData{}},
		{savepointBatch[2], 0, pgx.TraceBatchQueryData{}},
	}, context.Canceled)

	if got := len(spans.GetSpans()); got != 0 {
		t.Fatalf("spans = %d, want 0", got)
	}
}

func TestThePoolTracesBatchesWithOrWithoutMetrics(t *testing.T) {
	for name, opts := range map[string][]PoolOption{
		"plain":   nil,
		"metered": {WithMeterProvider(metricnoop.NewMeterProvider())},
	} {
		t.Run(name, func(t *testing.T) {
			spans, tracer := batchExporter(t)
			pool, err := NewPool(context.Background(), "postgres://u:p@db.example:6543/orders?sslmode=disable", tracer, opts...)
			if err != nil {
				t.Fatalf("NewPool() = %v", err)
			}
			t.Cleanup(pool.Close)

			traceBatch(t, pool.Config().ConnConfig.Tracer, tracer, savepointBatch[:1], []batchStep{{savepointBatch[0], 0, pgx.TraceBatchQueryData{}}}, nil)

			got := queriesOf(spans)
			if len(got) != 1 {
				t.Fatalf("spans = %v, want the SAVEPOINT", namesOf(got))
			}
			requireString(t, got[0], "server.address", "db.example")
		})
	}
}
