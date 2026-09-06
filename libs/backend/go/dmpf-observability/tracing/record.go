package tracing

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// EventAttempt is the name of the event a repeated attempt leaves on the span.
const EventAttempt = "dmpf.retry.attempt"

// KeyPreviousCategory is the category of the failure that caused the retry.
const KeyPreviousCategory = "dmpf.retry.previous_category"

// RecordError marks the span as failed by category. It deliberately does not
// call span.RecordError: that would attach the error message, which leaves the
// process without passing through redaction (TRC-12, TRC-15).
func RecordError(span trace.Span, category string) {
	if span == nil {
		return
	}

	span.SetStatus(codes.Error, "")
	if category != "" {
		span.SetAttributes(attribute.String(KeyErrorCategory, category))
	}
}

// AttemptEvent records a repeated attempt on the span of the operation, with
// the attempt number and the category that caused it (TRC-11).
func AttemptEvent(span trace.Span, attempt int, previousCategory string) {
	if span == nil {
		return
	}

	recorded := []attribute.KeyValue{attribute.Int(KeyAttempt, attempt)}
	if previousCategory != "" {
		recorded = append(recorded, attribute.String(KeyPreviousCategory, previousCategory))
	}
	span.AddEvent(EventAttempt, trace.WithAttributes(recorded...))
}

// EventBulkheadSaturated is the event a refused call leaves on the span when
// the pool and its queue are full (RES-23).
const EventBulkheadSaturated = "dmpf.bulkhead.saturated"

// BulkheadSaturated records the refusal on the span. It is a named function
// rather than a generic AddEvent so the set of events the platform emits stays
// closed, like the attribute builder.
func BulkheadSaturated(span trace.Span, dependency string) {
	if span == nil {
		return
	}
	span.AddEvent(EventBulkheadSaturated, trace.WithAttributes(
		attribute.String(KeyDependency, dependency),
	))
}
