package reservations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func tracedQuery(t *testing.T, end pgx.TraceQueryEndData) (tracetest.SpanStub, trace.SpanContext) {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	parentCtx, parent := provider.Tracer("db-test").Start(context.Background(), "parent")
	tracer := dbTracer{tracer: provider.Tracer("db-test")}
	ctx := tracer.TraceQueryStart(parentCtx, nil, pgx.TraceQueryStartData{SQL: "SELECT secret FROM t WHERE id = $1", Args: []any{"p-1"}})
	tracer.TraceQueryEnd(ctx, nil, end)
	parent.End()

	for _, span := range spans.GetSpans() {
		if span.Name == "dmpf.db.query" {
			return span, parent.SpanContext()
		}
	}
	t.Fatalf("no dmpf.db.query span among %d", len(spans.GetSpans()))
	return tracetest.SpanStub{}, trace.SpanContext{}
}

func TestTheQueryTracerOpensAClientSpanWithoutSQL(t *testing.T) {
	span, parent := tracedQuery(t, pgx.TraceQueryEndData{})

	if span.SpanKind != trace.SpanKindClient {
		t.Fatalf("kind = %v, want client", span.SpanKind)
	}
	if span.Parent.SpanID() != parent.SpanID() {
		t.Fatalf("parent = %s, want %s", span.Parent.SpanID(), parent.SpanID())
	}
	for _, attribute := range span.Attributes {
		if value := attribute.Value.String(); strings.Contains(value, "SELECT") || strings.Contains(value, "p-1") {
			t.Fatalf("attribute %s = %q leaks the statement or its arguments (TRC-15)", attribute.Key, value)
		}
	}
	if span.Status.Code != codes.Unset {
		t.Fatalf("status = %v, want unset", span.Status.Code)
	}
}

func TestAQueryOutsideATracedOperationOpensNoSpan(t *testing.T) {
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	tracer := dbTracer{tracer: provider.Tracer("db-test")}

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	if got := len(spans.GetSpans()); got != 0 {
		t.Fatalf("spans = %d, want 0 — a poll without a parent is not traced", got)
	}
}

func TestAFailedQueryMarksTheSpanWithoutTheMessage(t *testing.T) {
	span, _ := tracedQuery(t, pgx.TraceQueryEndData{Err: errors.New("boom: password=hunter2")})

	if span.Status.Code != codes.Error || strings.Contains(span.Status.Description, "hunter2") {
		t.Fatalf("status = %+v, want error without the message", span.Status)
	}
	for _, attribute := range span.Attributes {
		if string(attribute.Key) == "dmpf.error.category" && attribute.Value.AsString() == "storage" {
			return
		}
	}
	t.Fatalf("attributes = %v, want dmpf.error.category=storage", span.Attributes)
}
