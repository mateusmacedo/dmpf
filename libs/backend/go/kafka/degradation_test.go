package kafka_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

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

type unreachableCollector struct {
	runtime *otelboot.Runtime
	logs    *sdklog.LoggerProvider
	reader  *sdkmetric.PeriodicReader
	spans   *failingSpans
	records *failingLogs
	metrics *failingMetrics
}

func startAgainstAnUnreachableCollector(t *testing.T) *unreachableCollector {
	t.Helper()
	previousHandler := otel.GetErrorHandler()
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	previousTracer, previousMeter, previousPropagator := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()

	collector := &unreachableCollector{spans: &failingSpans{}, records: &failingLogs{}, metrics: &failingMetrics{}}
	collector.reader = sdkmetric.NewPeriodicReader(collector.metrics)
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "reservations", ServiceVersion: "1.0.0", ServiceInstanceID: "reservations-0", Role: "consumer"},
		TraceExporter: collector.spans,
		MetricReader:  collector.reader,
		LogLevel:      slog.LevelDebug,
	}
	collector.logs = otelboot.NewLoggerProvider(config, collector.records)
	config.LoggerProvider = collector.logs
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	collector.runtime = runtime
	t.Cleanup(func() {
		_ = runtime.Shutdown(context.Background())
		otel.SetTracerProvider(previousTracer)
		otel.SetMeterProvider(previousMeter)
		otel.SetTextMapPropagator(previousPropagator)
		otel.SetErrorHandler(previousHandler)
	})
	return collector
}

func (c *unreachableCollector) instrument(cfg *kafka.Config) {
	cfg.Tracer = c.runtime.Tracer()
	cfg.Instruments = c.runtime.Instruments()
	cfg.LoggerProvider = c.runtime.LoggerProvider()
}

func (c *unreachableCollector) assertLogsAndMetricsWereRefused(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := c.logs.ForceFlush(ctx); !errors.Is(err, errLogsDown) || c.records.records.Load() == 0 {
		t.Errorf("log flush = %v with %d records, want the records of the work refused by the collector", err, c.records.records.Load())
	}
	if err := c.reader.ForceFlush(ctx); !errors.Is(err, errMetricsDown) || c.metrics.exports.Load() == 0 {
		t.Errorf("metric flush = %v after %d exports, want the collection refused by the collector", err, c.metrics.exports.Load())
	}
}

func (c *unreachableCollector) assertSpansWereRefused(t *testing.T) {
	t.Helper()
	if err := c.runtime.ForceFlush(context.Background()); !errors.Is(err, errSpansDown) || c.spans.spans.Load() == 0 {
		t.Errorf("span flush = %v with %d spans, want the spans of the work refused by the collector", err, c.spans.spans.Load())
	}
}

func TestAnUnreachableCollectorDoesNotFailThePublication(t *testing.T) {
	collector := startAgainstAnUnreachableCollector(t)
	cfg := publishConfig()
	collector.instrument(&cfg)
	fake := kafka.NewFakeClient()
	pub, err := kafka.NewPublisherWith(cfg, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() = %v with the collector down, want nil: telemetry never fails the publication (NF Degradação)", err)
	}
	collector.assertLogsAndMetricsWereRefused(t)
	collector.assertSpansWereRefused(t)
	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() after the refused exports = %v, want nil", err)
	}
	if n := len(fake.Produced()); n != 2 {
		t.Fatalf("produced %d records, want 2", n)
	}
}

func TestAnUnreachableCollectorDoesNotFailTheConsumption(t *testing.T) {
	collector := startAgainstAnUnreachableCollector(t)
	sink := newSink()
	sink.on(0, func(ctx context.Context, _ int, ack ports.Acknowledger) error {
		_ = ack.Ack(ctx)
		return unavailable
	})
	sink.on(1, alwaysAck)
	c := newConsumer(sink)
	collector.instrument(&c.Config)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	flushed := make(chan struct{})

	consume(t, c, fake, func() bool {
		select {
		case <-flushed:
			return len(fake.Commits()) >= 2
		default:
		}
		if len(fake.Commits()) < 1 {
			return false
		}
		collector.assertLogsAndMetricsWereRefused(t)
		close(flushed)
		fake.Feed(topic, 0, recordsAt(1)...)
		return false
	})

	if got := commits(fake); len(got) != 2 || got[1] != 1 {
		t.Fatalf("commits = %v, want both records committed with the collector down (NF Degradação)", got)
	}
}
