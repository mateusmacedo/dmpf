package tracing_test

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func testTracer(t *testing.T) trace.Tracer {
	t.Helper()

	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(tracetest.NewSpanRecorder()))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	return provider.Tracer("observability")
}

func TestAnOwnedSpanIsFoundAndIsTheCurrentSpan(t *testing.T) {
	_, span := testTracer(t).Start(context.Background(), "send orders.events")
	defer span.End()

	ctx := tracing.WithOwnedSpan(context.Background(), span)

	owned, ok := tracing.OwnsSpan(ctx)
	if !ok {
		t.Fatal("OwnsSpan() = false, want the marked span")
	}
	if !owned.SpanContext().Equal(span.SpanContext()) {
		t.Fatalf("OwnsSpan() = %v, want %v", owned.SpanContext(), span.SpanContext())
	}
	if current := trace.SpanFromContext(ctx); !current.SpanContext().Equal(span.SpanContext()) {
		t.Fatalf("SpanFromContext() = %v, want the owned span as the current one", current.SpanContext())
	}
}

func TestASpanThatIsOnlyCurrentIsNotOwned(t *testing.T) {
	_, span := testTracer(t).Start(context.Background(), "grpc call")
	defer span.End()

	if _, ok := tracing.OwnsSpan(trace.ContextWithSpan(context.Background(), span)); ok {
		t.Fatal("OwnsSpan() = true for an unmarked span, want false: its caller did not hand it over")
	}
}

func TestAChildSpanEndsTheOwnership(t *testing.T) {
	tracer := testTracer(t)
	_, span := tracer.Start(context.Background(), "send orders.events")
	defer span.End()

	childCtx, child := tracer.Start(tracing.WithOwnedSpan(context.Background(), span), "db.query")
	defer child.End()

	if _, ok := tracing.OwnsSpan(childCtx); ok {
		t.Fatal("OwnsSpan() = true under a child span, want false: the marked span is no longer current")
	}
}

func TestANilOrInvalidSpanIsNeverOwned(t *testing.T) {
	ctx := context.Background()

	if got := tracing.WithOwnedSpan(ctx, nil); got != ctx {
		t.Fatal("WithOwnedSpan(nil) changed the context, want it untouched")
	}
	if _, ok := tracing.OwnsSpan(tracing.WithOwnedSpan(ctx, trace.SpanFromContext(ctx))); ok {
		t.Fatal("OwnsSpan() = true for an invalid span, want false: it identifies no span")
	}
	if _, ok := tracing.OwnsSpan(ctx); ok {
		t.Fatal("OwnsSpan() = true on an empty context, want false")
	}
}

func TestAnUnsampledSpanIsOwnedWithoutPanicking(t *testing.T) {
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	tracer := provider.Tracer("observability")
	_, span := tracer.Start(context.Background(), "send orders.events")
	defer span.End()
	if span.IsRecording() || !span.SpanContext().IsValid() {
		t.Fatalf("span recording = %v, valid = %v, want an unsampled span with a valid context", span.IsRecording(), span.SpanContext().IsValid())
	}

	ctx := tracing.WithOwnedSpan(context.Background(), span)
	if _, ok := tracing.OwnsSpan(ctx); !ok {
		t.Fatal("OwnsSpan() = false for a marked unsampled span, want true")
	}
	childCtx, child := tracer.Start(ctx, "db.query")
	defer child.End()
	if _, ok := tracing.OwnsSpan(childCtx); ok {
		t.Fatal("OwnsSpan() = true under an unsampled child, want false")
	}
}
