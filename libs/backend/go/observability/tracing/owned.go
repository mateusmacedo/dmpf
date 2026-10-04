package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

type ownedSpanKey struct{}

func WithOwnedSpan(ctx context.Context, span trace.Span) context.Context {
	if span == nil {
		return ctx
	}
	return context.WithValue(trace.ContextWithSpan(ctx, span), ownedSpanKey{}, span)
}

// OwnsSpan compares span contexts, not the spans: the SDK non-recording span
// (otel/sdk v1.47.0 trace/span.go:861) holds a TraceState slice, and == panics.
func OwnsSpan(ctx context.Context) (trace.Span, bool) {
	owned, marked := ctx.Value(ownedSpanKey{}).(trace.Span)
	if !marked || !owned.SpanContext().IsValid() {
		return nil, false
	}
	if !trace.SpanFromContext(ctx).SpanContext().Equal(owned.SpanContext()) {
		return nil, false
	}
	return owned, true
}
