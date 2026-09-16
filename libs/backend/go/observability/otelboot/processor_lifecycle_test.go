package otelboot_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

type failingExporter struct{ err error }

func (e failingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return e.err }
func (e failingExporter) Shutdown(context.Context) error                             { return nil }

func TestForceFlushReportsAnExportFailure(t *testing.T) {
	boom := errors.New("collector refused the batch")
	processor := newProcessor(t, failingExporter{err: boom}, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("sampled", true, false))

	if err := processor.ForceFlush(context.Background()); !errors.Is(err, boom) {
		t.Errorf("ForceFlush() = %v, want the exporter's failure", err)
	}
}

// orderedExporter records what it received and when it was closed, so a test
// can assert the order Shutdown promises.
type orderedExporter struct {
	mu          sync.Mutex
	exported    []string
	closed      bool
	afterClosed []string
}

func (e *orderedExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, span := range spans {
		if e.closed {
			e.afterClosed = append(e.afterClosed, span.Name())
			continue
		}
		e.exported = append(e.exported, span.Name())
	}
	return nil
}

func (e *orderedExporter) Shutdown(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	return nil
}

func (e *orderedExporter) state() ([]string, []string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.exported...), append([]string(nil), e.afterClosed...), e.closed
}

func TestShutdownDrainsWhatIsQueuedBeforeItClosesTheExporter(t *testing.T) {
	exporter := &orderedExporter{}
	processor, err := otelboot.NewClassAwareProcessor(exporter, otelboot.ProcessorOptions{})
	if err != nil {
		t.Fatalf("NewClassAwareProcessor() = %v", err)
	}

	processor.OnEnd(endedSpan("queued", true, false))

	if err := processor.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	exported, afterClosed, closed := exporter.state()
	if len(exported) != 1 || exported[0] != "queued" {
		t.Errorf("exported = %v, want [queued] before the exporter closes", exported)
	}
	if len(afterClosed) != 0 {
		t.Errorf("exported %v after the exporter was closed", afterClosed)
	}
	if !closed {
		t.Error("Shutdown() left the exporter open")
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	processor, err := otelboot.NewClassAwareProcessor(tracetest.NewInMemoryExporter(), otelboot.ProcessorOptions{})
	if err != nil {
		t.Fatalf("NewClassAwareProcessor() = %v", err)
	}

	for range 3 {
		if err := processor.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil on every call", err)
		}
	}
}

func TestAfterShutdownASpanIsNoLongerQueued(t *testing.T) {
	exporter := &orderedExporter{}
	processor, err := otelboot.NewClassAwareProcessor(exporter, otelboot.ProcessorOptions{})
	if err != nil {
		t.Fatalf("NewClassAwareProcessor() = %v", err)
	}
	if err := processor.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	processor.OnEnd(endedSpan("too-late", true, false))
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Errorf("ForceFlush() after shutdown = %v, want nil", err)
	}

	exported, afterClosed, _ := exporter.state()
	if len(exported)+len(afterClosed) != 0 {
		t.Errorf("exported %v/%v, want none after shutdown", exported, afterClosed)
	}
}

func TestConcurrentOnEndIsSafe(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	var writers sync.WaitGroup
	for range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 50 {
				processor.OnEnd(endedSpan("concurrent", true, false))
			}
		}()
	}
	writers.Wait()

	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	if got := len(exporter.GetSpans()); got != 400 {
		t.Errorf("exported = %d spans, want 400", got)
	}
}

func TestForceFlushHonoursACancelledContext(t *testing.T) {
	exporter := newBlockingExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("in-flight", true, false))
	<-exporter.entered
	t.Cleanup(func() { close(exporter.release) })

	ctx, cancel := expiredContext(t)
	defer cancel()

	if err := processor.ForceFlush(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ForceFlush() = %v, want the deadline of the caller to be honoured", err)
	}
}

// expiredContext is a context whose deadline has already passed on a clock the
// test drives, so the assertion never waits on the wall clock.
func expiredContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	fake := clock.NewFake(time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC))
	ctx, cancel := fake.WithTimeout(context.Background(), time.Second)
	fake.Advance(2 * time.Second)
	return ctx, cancel
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

// signallingExporter reports each export as it happens, so a test can wait for
// the worker's own cycle instead of racing it.
type signallingExporter struct {
	err      error
	exported chan struct{}
}

func (e *signallingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	e.exported <- struct{}{}
	return e.err
}

func (e *signallingExporter) Shutdown(context.Context) error { return nil }

func TestAFailureTheWorkerMetOnItsOwnIsReportedByTheNextForceFlush(t *testing.T) {
	boom := errors.New("collector refused the batch")
	exporter := &signallingExporter{err: boom, exported: make(chan struct{}, 1)}
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("sampled", true, false))
	<-exporter.exported // the worker exported and failed on its own cycle

	if err := processor.ForceFlush(context.Background()); !errors.Is(err, boom) {
		t.Errorf("ForceFlush() = %v, want the failure the worker met between flushes", err)
	}
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Errorf("ForceFlush() = %v, want nil: a reported failure is not reported twice", err)
	}
}

func TestShutdownHonoursACancelledContext(t *testing.T) {
	exporter := newBlockingExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("in-flight", true, false))
	<-exporter.entered
	t.Cleanup(func() { close(exporter.release) })

	ctx, cancel := expiredContext(t)
	defer cancel()

	if err := processor.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown() = %v, want the deadline of the caller to be honoured", err)
	}
}

// countingExporter records how many times it was closed, which is what an
// idempotent Shutdown has to keep at one.
type countingExporter struct {
	mu        sync.Mutex
	shutdowns int
}

func (e *countingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return nil }

func (e *countingExporter) Shutdown(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdowns++
	return nil
}

func (e *countingExporter) closed() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.shutdowns
}

func TestShutdownClosesTheExporterOnlyOnce(t *testing.T) {
	exporter := &countingExporter{}
	processor, err := otelboot.NewClassAwareProcessor(exporter, otelboot.ProcessorOptions{})
	if err != nil {
		t.Fatalf("NewClassAwareProcessor() = %v", err)
	}

	for range 3 {
		if err := processor.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	}

	if got := exporter.closed(); got != 1 {
		t.Errorf("the exporter was closed %d times, want 1: an exporter is not required to tolerate a second Shutdown", got)
	}
}

func TestShutdownConcurrentlyClosesTheExporterOnlyOnce(t *testing.T) {
	exporter := &countingExporter{}
	processor, err := otelboot.NewClassAwareProcessor(exporter, otelboot.ProcessorOptions{})
	if err != nil {
		t.Fatalf("NewClassAwareProcessor() = %v", err)
	}

	var callers sync.WaitGroup
	for range 8 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			_ = processor.Shutdown(context.Background())
		}()
	}
	callers.Wait()

	if got := exporter.closed(); got != 1 {
		t.Errorf("the exporter was closed %d times under concurrent shutdown, want 1", got)
	}
}
