package tracing_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
)

func recordedSpan(t *testing.T, record func(span trace.Span)) sdktrace.ReadOnlySpan {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	_, span := provider.Tracer("dmpf-observability").Start(context.Background(), "under.test")
	record(span)
	span.End()

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	return ended[0]
}

func TestRecordErrorSetsTheStatusAndTheCategory(t *testing.T) {
	span := recordedSpan(t, func(span trace.Span) {
		tracing.RecordError(span, "timeout")
	})

	if span.Status().Code != codes.Error {
		t.Fatalf("status = %v, want Error", span.Status().Code)
	}
	if span.Status().Description != "" {
		t.Fatalf("status description = %q, want empty (TRC-12)", span.Status().Description)
	}

	found := false
	for _, kv := range span.Attributes() {
		if string(kv.Key) == tracing.KeyErrorCategory {
			found = true
			if got := kv.Value.AsString(); got != "timeout" {
				t.Fatalf("%s = %q, want \"timeout\"", tracing.KeyErrorCategory, got)
			}
		}
	}
	if !found {
		t.Fatalf("%s is absent from the span", tracing.KeyErrorCategory)
	}
}

// TestRecordErrorLeavesNoExceptionEvent contrasts RecordError with the SDK's own
// span.RecordError, which is what TRC-12 forbids: the second call below produces
// the exception event, and the assertion is that only it does.
func TestRecordErrorLeavesNoExceptionEvent(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.7:5432: connection refused")

	ours := recordedSpan(t, func(span trace.Span) {
		tracing.RecordError(span, "timeout")
	})
	theirs := recordedSpan(t, func(span trace.Span) {
		span.RecordError(cause)
	})

	if got := exceptionEvents(theirs); got == 0 {
		t.Fatal("span.RecordError produced no exception event, so this test proves nothing")
	}
	if got := exceptionEvents(ours); got != 0 {
		t.Fatalf("tracing.RecordError produced %d exception events, want 0 — the message must not reach the span (TRC-12)", got)
	}
}

func exceptionEvents(span sdktrace.ReadOnlySpan) int {
	found := 0
	for _, event := range span.Events() {
		if event.Name == "exception" {
			found++
		}
	}
	return found
}

func TestRecordErrorWithoutACategoryStillMarksTheStatus(t *testing.T) {
	span := recordedSpan(t, func(span trace.Span) {
		tracing.RecordError(span, "")
	})

	if span.Status().Code != codes.Error {
		t.Fatalf("status = %v, want Error even without a category", span.Status().Code)
	}
	for _, kv := range span.Attributes() {
		if string(kv.Key) == tracing.KeyErrorCategory {
			t.Fatalf("%s is present with no category, want it omitted", tracing.KeyErrorCategory)
		}
	}
}

func TestAttemptEventRecordsTheNumberAndThePreviousCategory(t *testing.T) {
	span := recordedSpan(t, func(span trace.Span) {
		tracing.AttemptEvent(span, 2, "timeout")
	})

	events := span.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Name != tracing.EventAttempt {
		t.Fatalf("event name = %q, want %q", events[0].Name, tracing.EventAttempt)
	}

	recorded := map[string]string{}
	for _, kv := range events[0].Attributes {
		recorded[string(kv.Key)] = kv.Value.String()
	}
	if recorded[tracing.KeyAttempt] != "2" {
		t.Errorf("%s = %q, want \"2\"", tracing.KeyAttempt, recorded[tracing.KeyAttempt])
	}
	if recorded[tracing.KeyPreviousCategory] != "timeout" {
		t.Errorf("%s = %q, want \"timeout\"", tracing.KeyPreviousCategory, recorded[tracing.KeyPreviousCategory])
	}
}

func TestAttemptEventOmitsAnAbsentPreviousCategory(t *testing.T) {
	span := recordedSpan(t, func(span trace.Span) {
		tracing.AttemptEvent(span, 1, "")
	})

	for _, kv := range span.Events()[0].Attributes {
		if string(kv.Key) == tracing.KeyPreviousCategory {
			t.Fatalf("%s is present with no category, want it omitted", tracing.KeyPreviousCategory)
		}
	}
}

func TestANilSpanIsToleratedByBothRecorders(t *testing.T) {
	tracing.RecordError(nil, "timeout")
	tracing.AttemptEvent(nil, 1, "timeout")
}
