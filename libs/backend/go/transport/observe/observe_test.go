package observe_test

import (
	"bytes"
	"context"
	"errors"
	"go.opentelemetry.io/otel/attribute"
	"log/slog"
	"strings"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

var errBoom = errors.New("boom: secret detail that must never leave the process")

func op() resilience.Operation {
	return resilience.Operation{Dependency: "orders", Method: "Place", Kind: resilience.Remote, Deadline: time.Second, EstimatedDuration: 100 * time.Millisecond}
}

func failing(ctx context.Context, _ resilience.Operation, _ func(context.Context) error) error {
	return errBoom
}

func TestTracingRecordsCategoryAndNeverTheMessage(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	cfg := observe.Config{Service: "checkout", SpanPrefix: "dmpf.test ", Tracer: tp.Tracer("t"), Category: func(error) string { return "boom" }}
	err := observe.Tracing(cfg)(failing)(context.Background(), op(), nil)
	if !errors.Is(err, errBoom) {
		t.Fatalf("decorated call = %v, want the original error", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Name != "dmpf.test Place" {
		t.Fatalf("spans = %v, want one named dmpf.test Place", spans)
	}
	attrs := map[string]string{}
	for _, kv := range spans[0].Attributes {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	if attrs["dmpf.service"] != "checkout" || attrs["dmpf.dependency"] != "orders" || attrs["dmpf.operation"] != "Place" {
		t.Errorf("attributes = %v (TRC-04)", attrs)
	}
	if attrs["dmpf.error.category"] != "boom" || attrs["dmpf.outcome_category"] != "boom" {
		t.Errorf("categories = %v (TRC-12)", attrs)
	}
	if spans[0].Status.Description != "" {
		t.Errorf("status description %q leaks the message (TRC-12)", spans[0].Status.Description)
	}
	for _, kv := range spans[0].Attributes {
		if strings.Contains(kv.Value.AsString(), "secret") {
			t.Errorf("attribute %s carries the message", kv.Key)
		}
	}
}

func TestMetricsRecordsTheThreeServiceSeries(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("t"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := observe.Config{Service: "checkout", Clock: clock.NewFake(start), Instruments: instruments, Category: func(error) string { return "boom" }}
	_ = observe.Metrics(cfg)(failing)(context.Background(), op(), nil)
	_ = observe.Metrics(cfg)(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	points := map[string]int{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Sum[int64]:
				points[m.Name] = len(data.DataPoints)
			case metricdata.Histogram[float64]:
				points[m.Name] = len(data.DataPoints)
			}
		}
	}
	if points[metrics.RequestsTotal] != 2 {
		t.Errorf("%s has %d data points, want 2 (ok and boom)", metrics.RequestsTotal, points[metrics.RequestsTotal])
	}
	if points[metrics.ErrorsTotal] != 1 {
		t.Errorf("%s has %d data points, want 1", metrics.ErrorsTotal, points[metrics.ErrorsTotal])
	}
	if points[metrics.RequestDurationSeconds] != 2 {
		t.Errorf("%s has %d data points, want 2", metrics.RequestDurationSeconds, points[metrics.RequestDurationSeconds])
	}
}

func TestMetricsWithoutInstrumentsStillWrapsTheCall(t *testing.T) {
	ran := false
	err := observe.Metrics(observe.Config{})(resilience.Direct)(context.Background(), op(), func(context.Context) error { ran = true; return nil })
	if err != nil || !ran {
		t.Fatalf("call = %v, ran = %v", err, ran)
	}
}

func TestLoggingWritesTheCategoryAndNotTheMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	cfg := observe.Config{Logger: logger, Category: func(error) string { return "boom" }}

	_ = observe.Logging(cfg)(failing)(context.Background(), op(), nil)

	out := buf.String()
	if !strings.Contains(out, "error_category=boom") || !strings.Contains(out, "dependency=orders") {
		t.Fatalf("log = %q, want category and dependency", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("log = %q carries the error message (LOG-13)", out)
	}
	buf.Reset()
	_ = observe.Logging(cfg)(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })
	if buf.Len() != 0 {
		t.Fatalf("a successful call logged %q", buf.String())
	}
}

func TestSlotsFillsExactlyTheThreeObservabilityPositions(t *testing.T) {
	slots := observe.Slots(observe.Config{})
	if slots.Tracing == nil || slots.Metrics == nil || slots.Logging == nil {
		t.Fatal("an observability position is empty (RES-23)")
	}
	if slots.Bulkhead != nil || slots.Breaker != nil || slots.RateLimit != nil || slots.Retry != nil || slots.Timeout != nil {
		t.Fatal("Slots filled a position that is the caller's")
	}
}

func TestMetricsKeepsOneSeriesPerMethodAndCategoryAcrossRepeatedCalls(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("t"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := observe.Config{Service: "checkout", Clock: clock.NewFake(start), Instruments: instruments, Category: func(error) string { return "boom" }}
	decorated := observe.Metrics(cfg)
	for range 50 {
		_ = decorated(failing)(context.Background(), op(), nil)
		_ = decorated(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metrics.RequestsTotal {
				continue
			}
			sum := m.Data.(metricdata.Sum[int64])
			if len(sum.DataPoints) != 2 {
				t.Fatalf("%s has %d data points, want 2 (ok and boom) after 100 calls", m.Name, len(sum.DataPoints))
			}
			for _, dp := range sum.DataPoints {
				if dp.Value != 50 {
					t.Fatalf("series %v counted %d, want 50", dp.Attributes.Encoded(attribute.DefaultEncoder()), dp.Value)
				}
			}
			return
		}
	}
	t.Fatalf("series %q not recorded", metrics.RequestsTotal)
}
