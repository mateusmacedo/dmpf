package obstest

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

type DownCollector struct {
	Addr   string
	stalls bool
	held   atomic.Int64
}

func UnusedLoopbackAddr(t testing.TB) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func UnreachableCollector(t testing.TB) *DownCollector {
	t.Helper()
	return &DownCollector{Addr: UnusedLoopbackAddr(t)}
}

// StallingCollector accepts every connection and never answers, which is the
// collector a timeout has to cut off rather than the one that refuses.
func StallingCollector(t testing.TB) *DownCollector {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	collector := &DownCollector{Addr: listener.Addr().String(), stalls: true}
	var held []net.Conn
	accepting := make(chan struct{})
	go func() {
		defer close(accepting)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			held = append(held, conn)
			collector.held.Add(1)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-accepting
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	return collector
}

func (c *DownCollector) RequireEverySignalExporting(t testing.TB) {
	t.Helper()
	if !c.stalls {
		return
	}
	for deadline := time.Now().Add(2 * time.Second); c.held.Load() < 3 && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	if held := c.held.Load(); held < 3 {
		t.Errorf("the stalled collector holds %d connections, want one per signal: traces, metrics and logs exporting to it", held)
	}
}

func ReturnsWithin(t testing.TB, limit time.Duration, what string, run func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		run()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Errorf("%s did not return in %v with the export failing, want the telemetry never to hold it (NF Degradação)", what, limit)
	}
}

var (
	errSpansDown   = errors.New("collector unreachable: spans refused")
	errLogsDown    = errors.New("collector unreachable: logs refused")
	errMetricsDown = errors.New("collector unreachable: metrics refused")
)

type failingSpans struct{ spans atomic.Int64 }

func (e *failingSpans) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.spans.Add(int64(len(spans)))
	return errSpansDown
}
func (e *failingSpans) Shutdown(context.Context) error { return nil }

type failingLogs struct{ records atomic.Int64 }

func (e *failingLogs) Export(_ context.Context, records []sdklog.Record) error {
	e.records.Add(int64(len(records)))
	return errLogsDown
}
func (e *failingLogs) Shutdown(context.Context) error   { return nil }
func (e *failingLogs) ForceFlush(context.Context) error { return nil }

type failingMetrics struct{ exports atomic.Int64 }

func (e *failingMetrics) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(kind)
}
func (e *failingMetrics) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}
func (e *failingMetrics) Export(context.Context, *metricdata.ResourceMetrics) error {
	e.exports.Add(1)
	return errMetricsDown
}
func (e *failingMetrics) ForceFlush(context.Context) error { return nil }
func (e *failingMetrics) Shutdown(context.Context) error   { return nil }

// Refusal is a runtime whose three exporters refuse every export in process,
// for adapters whose own pipeline reports what the collector refused.
type Refusal struct {
	Runtime *otelboot.Runtime
	logs    *sdklog.LoggerProvider
	reader  *sdkmetric.PeriodicReader
	spans   *failingSpans
	records *failingLogs
	metrics *failingMetrics
}

func RefusedExports(t testing.TB, resource otelboot.Resource) *Refusal {
	t.Helper()
	previousHandler := otel.GetErrorHandler()
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	previousTracer, previousMeter, previousPropagator := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()

	r := &Refusal{spans: &failingSpans{}, records: &failingLogs{}, metrics: &failingMetrics{}}
	r.reader = sdkmetric.NewPeriodicReader(r.metrics)
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      resource,
		TraceExporter: r.spans,
		MetricReader:  r.reader,
		LogLevel:      slog.LevelDebug,
	}
	r.logs = otelboot.NewLoggerProvider(config, r.records)
	config.LoggerProvider = r.logs
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	r.Runtime = runtime
	t.Cleanup(func() {
		_ = runtime.Shutdown(context.Background())
		otel.SetTracerProvider(previousTracer)
		otel.SetMeterProvider(previousMeter)
		otel.SetTextMapPropagator(previousPropagator)
		otel.SetErrorHandler(previousHandler)
	})
	return r
}

// RequireLogsAndMetricsRefused polls the log flush because an adapter may emit
// its last record after the call under test returned.
func (r *Refusal) RequireLogsAndMetricsRefused(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := r.logs.ForceFlush(ctx)
		if errors.Is(err, errLogsDown) && r.records.records.Load() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("log flush = %v with %d records, want the records of the work refused by the collector", err, r.records.records.Load())
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := r.reader.ForceFlush(ctx); !errors.Is(err, errMetricsDown) || r.metrics.exports.Load() == 0 {
		t.Errorf("metric flush = %v after %d exports, want the collection refused by the collector", err, r.metrics.exports.Load())
	}
}

func (r *Refusal) RequireSpansRefused(t testing.TB) {
	t.Helper()
	if err := r.Runtime.ForceFlush(context.Background()); !errors.Is(err, errSpansDown) || r.spans.spans.Load() == 0 {
		t.Errorf("span flush = %v with %d spans, want the spans of the work refused by the collector", err, r.spans.spans.Load())
	}
}
