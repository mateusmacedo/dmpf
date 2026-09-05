package otelboot_test

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

func endedSpan(name string, sampled, failed bool) sdktrace.ReadOnlySpan {
	var flags trace.TraceFlags
	if sampled {
		flags = trace.FlagsSampled
	}
	stub := tracetest.SpanStub{
		Name: name,
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{1},
			SpanID:     trace.SpanID{1},
			TraceFlags: flags,
		}),
	}
	if failed {
		stub.Status = sdktrace.Status{Code: codes.Error}
	}
	return stub.Snapshot()
}

func exportedNames(t *testing.T, exporter *tracetest.InMemoryExporter) []string {
	t.Helper()

	spans := exporter.GetSpans()
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}
	return names
}

func newProcessor(t *testing.T, exporter sdktrace.SpanExporter, options otelboot.ProcessorOptions) *otelboot.ClassAwareProcessor {
	t.Helper()

	processor, err := otelboot.NewClassAwareProcessor(exporter, options)
	if err != nil {
		t.Fatalf("NewClassAwareProcessor() = %v", err)
	}
	t.Cleanup(func() { _ = processor.Shutdown(context.Background()) })
	return processor
}

func TestAProcessorWithoutAnExporterIsRefused(t *testing.T) {
	if _, err := otelboot.NewClassAwareProcessor(nil, otelboot.ProcessorOptions{}); !errors.Is(err, otelboot.ErrExporterRequired) {
		t.Fatalf("NewClassAwareProcessor(nil) = %v, want ErrExporterRequired", err)
	}
}

func TestASampledSpanIsExported(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("sampled", true, false))
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := exportedNames(t, exporter); len(got) != 1 || got[0] != "sampled" {
		t.Errorf("exported = %v, want [sampled]", got)
	}
}

func TestAnUnsampledSpanThatFailedIsExportedAnyway(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("recorded-error", false, true))
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := exportedNames(t, exporter); len(got) != 1 || got[0] != "recorded-error" {
		t.Errorf("exported = %v, want [recorded-error]: an error is exported even unsampled (TRC-14)", got)
	}
}

func TestAnUnsampledSpanWithoutAnErrorNeverReachesTheExporter(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	processor.OnEnd(endedSpan("recorded-only", false, false))
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := exportedNames(t, exporter); len(got) != 0 {
		t.Errorf("exported = %v, want none: a recorded span with no error is not telemetry to ship", got)
	}
}

func TestOnStartDoesNotTouchTheSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})

	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	_, span := provider.Tracer("t").Start(context.Background(), "started")
	defer span.End()

	processor.OnStart(context.Background(), span.(sdktrace.ReadWriteSpan))

	if got := exportedNames(t, exporter); len(got) != 0 {
		t.Errorf("exported = %v, want none: OnStart ships nothing", got)
	}
}

// The scenario of the spec: 1000 root write spans, 100 within the 10% rate, 10
// of the 900 outside it end in error. The exporter must hold exactly 110.
func TestTheSampledSpansAndTheFailedOnesAreTheOnlyOnesExported(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{})
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	const total, within = 1000, 100
	sampled, failed := 0, 0
	for i := range total {
		var id trace.TraceID
		id[0] = 1
		if i < within {
			binary.BigEndian.PutUint64(id[8:16], 0)
		} else {
			binary.BigEndian.PutUint64(id[8:16], ^uint64(0)-uint64(i))
		}

		decision := sampler.ShouldSample(sdktrace.SamplingParameters{
			ParentContext: context.Background(),
			TraceID:       id,
			Attributes:    tracing.Attributes{}.TrafficClass(string(tracing.ClassWrite)).KeyValues(),
		})
		if decision.Decision == sdktrace.Drop {
			t.Fatalf("span %d was dropped; the sampler must never drop", i)
		}

		isSampled := decision.Decision == sdktrace.RecordAndSample
		if isSampled {
			sampled++
		}
		fails := !isSampled && failed < 10
		if fails {
			failed++
		}
		processor.OnEnd(endedSpan("span", isSampled, fails))
	}

	if sampled != within || failed != 10 {
		t.Fatalf("the fixture produced %d sampled and %d failed spans, want %d and 10", sampled, failed, within)
	}
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := len(exporter.GetSpans()); got != within+10 {
		t.Errorf("exported = %d spans, want %d: the sampled ones plus the failures", got, within+10)
	}
}

// blockingExporter holds the worker inside ExportSpans until it is released, so
// the queue fills in a way the test controls rather than races with.
type blockingExporter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once

	mu       sync.Mutex
	exported []sdktrace.ReadOnlySpan
}

func newBlockingExporter() *blockingExporter {
	return &blockingExporter{entered: make(chan struct{}), release: make(chan struct{})}
}

func (e *blockingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.once.Do(func() {
		close(e.entered)
		<-e.release
	})
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exported = append(e.exported, spans...)
	return nil
}

func (e *blockingExporter) Shutdown(context.Context) error { return nil }

func (e *blockingExporter) names() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make([]string, 0, len(e.exported))
	for _, span := range e.exported {
		names = append(names, span.Name())
	}
	return names
}

func TestAFullQueueSacrificesTheOldestSpanThatCarriesNoError(t *testing.T) {
	exporter := newBlockingExporter()
	reader := sdkmetric.NewManualReader()
	instruments, err := metrics.New(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test"))
	if err != nil {
		t.Fatalf("metrics.New() = %v", err)
	}

	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{
		QueueSize: 2,
		Dropped:   instruments.SpansDropped,
	})

	processor.OnEnd(endedSpan("in-flight", true, false))
	<-exporter.entered

	processor.OnEnd(endedSpan("oldest-ok", true, false))
	processor.OnEnd(endedSpan("newer-ok", true, false))
	processor.OnEnd(endedSpan("failure", false, true))
	processor.OnEnd(endedSpan("last-ok", true, false))

	close(exporter.release)
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	got := exporter.names()
	want := map[string]bool{"in-flight": true, "failure": true, "last-ok": true}
	if len(got) != len(want) {
		t.Fatalf("exported = %v, want the three survivors of %v", got, want)
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("exported %q; the queue should have dropped it before a failure", name)
		}
	}

	if dropped := counterValue(t, reader, metrics.SpansDroppedTotal); dropped != 2 {
		t.Errorf("%s = %d, want 2", metrics.SpansDroppedTotal, dropped)
	}
}

func TestAQueueFullOfFailuresRefusesTheNewSpanInsteadOfLosingOne(t *testing.T) {
	exporter := newBlockingExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{QueueSize: 2})

	processor.OnEnd(endedSpan("in-flight", true, false))
	<-exporter.entered

	processor.OnEnd(endedSpan("failure-a", false, true))
	processor.OnEnd(endedSpan("failure-b", false, true))
	processor.OnEnd(endedSpan("refused", true, false))

	close(exporter.release)
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	for _, name := range exporter.names() {
		if name == "refused" {
			t.Error("the new span displaced a queued failure; a failure is what the queue is for")
		}
	}
}

func TestTheDroppedCounterIsOptional(t *testing.T) {
	exporter := newBlockingExporter()
	processor := newProcessor(t, exporter, otelboot.ProcessorOptions{QueueSize: 1})

	processor.OnEnd(endedSpan("in-flight", true, false))
	<-exporter.entered

	processor.OnEnd(endedSpan("queued", true, false))
	processor.OnEnd(endedSpan("dropped", true, false))

	close(exporter.release)
	if err := processor.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
}
