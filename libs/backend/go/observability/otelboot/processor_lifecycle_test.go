package otelboot_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

type failingExporter struct{ err error }

func (e failingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return e.err }
func (e failingExporter) Shutdown(context.Context) error                             { return nil }

func TestForceFlushReportsAnExportFailure(t *testing.T) {
	refused := errors.New("collector unreachable")
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.TraceExporter = failingExporter{err: refused}
	})

	endSpan(runtime, "orders.place")

	if err := runtime.ForceFlush(context.Background()); !errors.Is(err, refused) {
		t.Fatalf("ForceFlush() = %v, want the export failure", err)
	}
}

type orderedExporter struct {
	mu       sync.Mutex
	exported int
	closes   int
	late     bool
}

func (e *orderedExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closes > 0 {
		e.late = true
	}
	e.exported += len(spans)
	return nil
}

func (e *orderedExporter) Shutdown(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closes++
	return nil
}

func TestShutdownDrainsWhatIsQueuedBeforeItClosesTheExporterOnce(t *testing.T) {
	exporter := &orderedExporter{}
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) { config.TraceExporter = exporter })

	for range 5 {
		endSpan(runtime, "orders.place")
	}
	for range 3 {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() = %v", err)
		}
	}

	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	if exporter.exported != 5 || exporter.late || exporter.closes != 1 {
		t.Fatalf("exported=%d late=%v closes=%d, want the five spans drained before a single close",
			exporter.exported, exporter.late, exporter.closes)
	}
}

func counterValue(t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, series := range scope.Metrics {
			if series.Name != name {
				continue
			}
			sum, ok := series.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("%s is a %T, want a Sum[int64]", name, series.Data)
			}
			var total int64
			for _, point := range sum.DataPoints {
				total += point.Value
			}
			return total
		}
	}
	return 0
}
