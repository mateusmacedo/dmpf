package otelboot_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func exportedNames(t *testing.T, exporter *tracetest.InMemoryExporter) []string {
	t.Helper()

	spans := exporter.GetSpans()
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}
	return names
}

type blockingExporter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once

	mu       sync.Mutex
	exported int
}

func newBlockingExporter() *blockingExporter {
	return &blockingExporter{entered: make(chan struct{}), release: make(chan struct{})}
}

func (e *blockingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.once.Do(func() { close(e.entered) })
	<-e.release
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exported += len(spans)
	return nil
}

func (e *blockingExporter) Shutdown(context.Context) error { return nil }

func (e *blockingExporter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exported
}

func endSpan(runtime *otelboot.Runtime, name string) {
	_, span := runtime.Tracer().Start(context.Background(), name,
		trace.WithAttributes(tracing.Attributes{}.TrafficClass(string(tracing.ClassWrite)).KeyValues()...))
	span.End()
}

func fillTheQueue(t *testing.T, runtime *otelboot.Runtime, exporter *blockingExporter) {
	t.Helper()
	endSpan(runtime, "first")
	select {
	case <-exporter.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the batch processor never started exporting the first span")
	}
	for range 10 {
		endSpan(runtime, "queued")
	}
	close(exporter.release)
}

func TestASampledSpanIsExportedByTheBatchProcessor(t *testing.T) {
	runtime, exporter := startedRuntime(t, nil)

	endSpan(runtime, "orders.place")
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := exportedNames(t, exporter); len(got) != 1 || got[0] != "orders.place" {
		t.Errorf("exported = %v, want [orders.place]", got)
	}
}

func TestAnUnsampledSpanIsNotExportedByTheProcessEvenWhenItFails(t *testing.T) {
	runtime, exporter := startedRuntime(t, func(config *otelboot.Config) {
		config.Sampling = tracing.Rates{tracing.ClassRead: 0}
	})

	_, span := runtime.Tracer().Start(context.Background(), "orders.find",
		trace.WithAttributes(tracing.Attributes{}.TrafficClass(string(tracing.ClassRead)).KeyValues()...))
	if span.SpanContext().IsSampled() || !span.IsRecording() {
		t.Fatal("the fixture wants a recorded span that is not sampled")
	}
	span.SetStatus(codes.Error, "")
	span.End()
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := exportedNames(t, exporter); len(got) != 0 {
		t.Errorf("exported = %v, want none: TRC-14 is kept by the tail sampling of the Collector (RF-E7)", got)
	}
}

func TestTheQueueSizeOfTheEnvironmentIsRespected(t *testing.T) {
	t.Setenv("OTEL_BSP_MAX_QUEUE_SIZE", "2")
	t.Setenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", "1")
	exporter := newBlockingExporter()
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) { config.TraceExporter = exporter })

	fillTheQueue(t, runtime, exporter)
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	if got := exporter.count(); got != 3 {
		t.Errorf("exported %d spans, want 3: the one in flight and the two the queue of OTEL_BSP_MAX_QUEUE_SIZE holds", got)
	}
}

func TestAFullQueueIsCountedByTheSDKObservability(t *testing.T) {
	t.Setenv("OTEL_GO_X_OBSERVABILITY", "true")
	t.Setenv("OTEL_BSP_MAX_QUEUE_SIZE", "2")
	t.Setenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", "1")
	exporter := newBlockingExporter()
	reader := sdkmetric.NewManualReader()
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.TraceExporter = exporter
		config.MetricReader = reader
	})

	fillTheQueue(t, runtime, exporter)
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := queueFull(t, reader); got != 8 {
		t.Errorf("otel.sdk.processor.span.processed{error.type=queue_full} = %d, want 8", got)
	}
}

func queueFull(t *testing.T, reader *sdkmetric.ManualReader) int64 {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, series := range scope.Metrics {
			if series.Name != "otel.sdk.processor.span.processed" {
				continue
			}
			sum, ok := series.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("%s is a %T, want a Sum[int64]", series.Name, series.Data)
			}
			var total int64
			for _, point := range sum.DataPoints {
				if value, _ := point.Attributes.Value(semconv.ErrorTypeKey); value == attribute.StringValue("queue_full") {
					total += point.Value
				}
			}
			return total
		}
	}
	return 0
}
