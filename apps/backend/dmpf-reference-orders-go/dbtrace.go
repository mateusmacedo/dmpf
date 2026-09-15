package dmpfreferenceorders

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
)

type dbSpanKey struct{}

// dbTracer opens a client span per query inside a traced operation only: the
// relay's claim loop polls without a parent, and a root span per poll is noise.
// The span never carries the statement or its arguments (TRC-15).
type dbTracer struct {
	tracer trace.Tracer
}

func (d dbTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	ctx, span := d.tracer.Start(ctx, "dmpf.db.query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(tracing.Attributes{}.Dependency("postgres").KeyValues()...))
	return context.WithValue(ctx, dbSpanKey{}, span)
}

func (d dbTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(dbSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	if data.Err != nil {
		tracing.RecordError(span, "storage")
	}
	span.End()
}
