package otelboot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

func startedRuntime(t *testing.T, mutate func(*otelboot.Config)) (*otelboot.Runtime, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	config := validConfig()
	config.TraceExporter = exporter
	config.MetricReader = sdkmetric.NewManualReader()
	if mutate != nil {
		mutate(&config)
	}

	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return runtime, exporter
}

func TestStartRefusesAConfigurationItCannotValidate(t *testing.T) {
	config := validConfig()
	config.Propagator = nil
	config.TraceExporter = tracetest.NewInMemoryExporter()

	before := otel.GetTextMapPropagator()

	runtime, err := otelboot.Start(context.Background(), config)
	if !errors.Is(err, otelboot.ErrPropagatorRequired) {
		t.Fatalf("Start() = %v, want ErrPropagatorRequired", err)
	}
	if runtime != nil {
		t.Error("Start() returned a runtime along with the failure")
	}
	if otel.GetTextMapPropagator() != before {
		t.Error("a refused Start replaced the global propagator")
	}
}

func TestStartRefusesToBootWithoutATraceExporter(t *testing.T) {
	if _, err := otelboot.Start(context.Background(), validConfig()); !errors.Is(err, otelboot.ErrExporterRequired) {
		t.Fatalf("Start() = %v, want ErrExporterRequired", err)
	}
}

func TestStartRegistersThePropagatorItWasGiven(t *testing.T) {
	composite := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	startedRuntime(t, func(config *otelboot.Config) { config.Propagator = composite })

	fields := otel.GetTextMapPropagator().Fields()
	if !strings.Contains(strings.Join(fields, ","), "baggage") {
		t.Errorf("the global propagator injects %v, want the configured one", fields)
	}
}

func TestASecondStartIsRefusedWhileTheFirstIsRunning(t *testing.T) {
	startedRuntime(t, nil)

	config := validConfig()
	config.TraceExporter = tracetest.NewInMemoryExporter()

	if _, err := otelboot.Start(context.Background(), config); !errors.Is(err, otelboot.ErrAlreadyStarted) {
		t.Fatalf("Start() = %v, want ErrAlreadyStarted", err)
	}
}

func TestTheProcessRestartsAfterAShutdown(t *testing.T) {
	runtime, _ := startedRuntime(t, nil)
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	config := validConfig()
	config.TraceExporter = tracetest.NewInMemoryExporter()

	again, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() after Shutdown = %v, want nil", err)
	}
	t.Cleanup(func() { _ = again.Shutdown(context.Background()) })
}

func TestTheRuntimeTracerProducesSpansThroughTheSingleProcessor(t *testing.T) {
	runtime, exporter := startedRuntime(t, nil)

	_, span := runtime.Tracer().Start(context.Background(), "orders.place",
		trace.WithAttributes(tracing.Attributes{}.TrafficClass(string(tracing.ClassMaintenance)).KeyValues()...))
	span.End()

	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	if got := exportedNames(t, exporter); len(got) != 1 || got[0] != "orders.place" {
		t.Errorf("exported = %v, want [orders.place]", got)
	}
}

func TestAnUnsampledSpanThatFailsStillLeavesTheProcess(t *testing.T) {
	runtime, exporter := startedRuntime(t, func(config *otelboot.Config) {
		config.Sampling = tracing.Rates{tracing.ClassRead: 0}
	})

	_, span := runtime.Tracer().Start(context.Background(), "orders.find",
		trace.WithAttributes(tracing.Attributes{}.TrafficClass(string(tracing.ClassRead)).KeyValues()...))
	if span.SpanContext().IsSampled() {
		t.Fatal("the fixture sampled the span; the rate of read is zero")
	}
	if !span.IsRecording() {
		t.Fatal("the span is not recording; the sampler dropped it instead of recording it")
	}
	span.SetStatus(codes.Error, "")
	span.End()

	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	if got := exportedNames(t, exporter); len(got) != 1 {
		t.Errorf("exported = %v, want the failed span (TRC-14)", got)
	}
}

func TestTheRuntimeMeterCarriesThePlatformInstruments(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) { config.MetricReader = reader })

	runtime.Instruments().Requests.Add(context.Background(), 1)

	if got := counterValue(t, reader, metrics.RequestsTotal); got != 1 {
		t.Errorf("%s = %d, want 1", metrics.RequestsTotal, got)
	}
}

func TestTheResourceOfTheProvidersIdentifiesTheService(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	startedRuntime(t, func(config *otelboot.Config) {
		config.MetricReader = reader
		config.Sheets = []resilience.Sheet{resilience.Defaults("payments")}
	})

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}

	attributes := index(collected.Resource.Attributes())
	if got := attributes[semconv.ServiceNameKey]; got != "orders" {
		t.Errorf("%s = %q, want %q", semconv.ServiceNameKey, got, "orders")
	}
	sheetKey := attribute.Key(otelboot.SheetAttributePrefix + "payments." + resilience.FieldMaxAttempts)
	if got := attributes[sheetKey]; got != "3" {
		t.Errorf("%s = %q, want the sheet in the resource (RES-40)", sheetKey, got)
	}
}

func TestTheEffectiveSheetsAreLoggedOnceAtStartUp(t *testing.T) {
	var out bytes.Buffer
	startedRuntime(t, func(config *otelboot.Config) {
		config.Logger = slog.New(slog.NewJSONHandler(&out, nil))
		config.Sheets = []resilience.Sheet{resilience.Defaults("payments"), resilience.Defaults("ledger")}
	})

	records := 0
	dependencies := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("the start-up record is not JSON: %v", err)
		}
		records++
		if record["level"] != "INFO" {
			t.Errorf("level = %v, want INFO", record["level"])
		}
		dependencies[record["dependency"].(string)] = true
	}

	if records != 2 || !dependencies["payments"] || !dependencies["ledger"] {
		t.Errorf("logged %d records for %v, want one per sheet", records, dependencies)
	}
}

func TestARuntimeWithoutALoggerStillStarts(t *testing.T) {
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.Sheets = []resilience.Sheet{resilience.Defaults("payments")}
	})

	if runtime.Logger() == nil {
		t.Error("Logger() = nil; a caller should never have to nil-check it")
	}
}

func TestShutdownIsIdempotentAndClosesBothProviders(t *testing.T) {
	runtime, _ := startedRuntime(t, nil)

	for range 3 {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil on every call", err)
		}
	}
}

func TestTheRuntimeMeterBuildsInstrumentsOfItsOwn(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) { config.MetricReader = reader })

	counter, err := runtime.Meter().Int64Counter("dmpf_test_counter")
	if err != nil {
		t.Fatalf("Int64Counter() = %v", err)
	}
	counter.Add(context.Background(), 2)

	if got := counterValue(t, reader, "dmpf_test_counter"); got != 2 {
		t.Errorf("dmpf_test_counter = %d, want 2", got)
	}
}

func TestARuntimeWithoutAMetricReaderStillStarts(t *testing.T) {
	config := validConfig()
	config.TraceExporter = tracetest.NewInMemoryExporter()

	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() without a metric reader = %v, want nil: a service may ship traces only", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	runtime.Instruments().Requests.Add(context.Background(), 1)
}
