package otelboot_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
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

	// The test installs what it will look for, instead of reading whatever the
	// previous test left behind: comparing two TextMapPropagator values with !=
	// panics when the dynamic type is not comparable, and the composite one is a
	// slice.
	sentinel := propagation.TraceContext{}
	otel.SetTextMapPropagator(sentinel)

	runtime, err := otelboot.Start(context.Background(), config)
	if !errors.Is(err, otelboot.ErrPropagatorRequired) {
		t.Fatalf("Start() = %v, want ErrPropagatorRequired", err)
	}
	if runtime != nil {
		t.Error("Start() returned a runtime along with the failure")
	}
	if _, untouched := otel.GetTextMapPropagator().(propagation.TraceContext); !untouched {
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

func TestTheRuntimeMeterCarriesThePlatformInstruments(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) { config.MetricReader = reader })

	runtime.Instruments().Retries.Add(context.Background(), 1)

	if got := counterValue(t, reader, metrics.RetriesTotal); got != 1 {
		t.Errorf("%s = %d, want 1", metrics.RetriesTotal, got)
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
	for key, value := range attributes {
		if strings.HasPrefix(string(key), "dmpf.sheet.") {
			t.Errorf("%s = %q is on the resource, want the sheet out of it", key, value)
		}
	}
}

func TestTheEffectiveSheetsAreLoggedOnceAtStartUp(t *testing.T) {
	exporter := &recordingExporter{}
	var logs *sdklog.LoggerProvider
	startedRuntime(t, func(config *otelboot.Config) {
		config.Sheets = []resilience.Sheet{resilience.Defaults("payments"), resilience.Defaults("ledger")}
		logs = otelboot.NewLoggerProvider(*config, exporter)
		config.LoggerProvider = logs
	})
	if err := logs.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	dependencies := map[string]bool{}
	for _, record := range attributesOfRecords(exporter.records) {
		dependencies[record[tracing.KeyDependency].AsString()] = true
	}
	for _, record := range exporter.records {
		if record.Severity() != log.SeverityInfo {
			t.Errorf("severity = %v, want info", record.Severity())
		}
	}

	if len(exporter.records) != 2 || !dependencies["payments"] || !dependencies["ledger"] {
		t.Errorf("logged %d records for %v under %s past the processor, want one per sheet (RF-A3)", len(exporter.records), dependencies, tracing.KeyDependency)
	}
}

func TestEveryFieldOfTheSheetInEffectSurvivesTheAllowlistUnderDmpfSheet(t *testing.T) {
	for _, role := range []string{"api", "consumer", "relay"} {
		t.Run(role, func(t *testing.T) {
			sheet := resilience.Defaults("payments")
			sheet.Deadline = resilience.Declare(3 * time.Second)
			exporter := &recordingExporter{}
			var logs *sdklog.LoggerProvider
			startedRuntime(t, func(config *otelboot.Config) {
				config.Resource.Role = role
				config.Sheets = []resilience.Sheet{sheet}
				logs = otelboot.NewLoggerProvider(*config, exporter)
				config.LoggerProvider = logs
			})
			if err := logs.ForceFlush(context.Background()); err != nil {
				t.Fatalf("ForceFlush() = %v", err)
			}

			record := onlyRecord(t, attributesOfRecords(exporter.records))
			effective := sheet.Effective()
			for field, value := range effective {
				key := "dmpf.sheet." + field
				if got, carried := record[key]; !carried || got.AsString() != value {
					t.Errorf("%s = %v (carried %t), want %q past the processor (RES-40, RF-A6)", key, got, carried, value)
				}
			}
			if len(record) != len(effective)+1 || record[tracing.KeyDependency].AsString() != "payments" {
				t.Errorf("record = %v, want %s and the %d fields of the sheet, nothing else", record, tracing.KeyDependency, len(effective))
			}
		})
	}
}

func TestARuntimeWithoutALoggerStillStarts(t *testing.T) {
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.Sheets = []resilience.Sheet{resilience.Defaults("payments")}
	})

	if runtime.LoggerFor(libraryScope) == nil || runtime.LoggerProvider() == nil {
		t.Error("LoggerFor() or LoggerProvider() = nil; a caller should never have to nil-check them")
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

	runtime.Instruments().Retries.Add(context.Background(), 1)
}

type countingMetricExporter struct {
	metrics atomic.Int64
}

func (*countingMetricExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(kind)
}

func (*countingMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}

func (e *countingMetricExporter) Export(_ context.Context, collected *metricdata.ResourceMetrics) error {
	for _, scope := range collected.ScopeMetrics {
		e.metrics.Add(int64(len(scope.Metrics)))
	}
	return nil
}

func (*countingMetricExporter) ForceFlush(context.Context) error { return nil }
func (*countingMetricExporter) Shutdown(context.Context) error   { return nil }

func TestForceFlushExportsWhatTheTraceMetricAndLogPipelinesHold(t *testing.T) {
	t.Setenv("OTEL_BLRP_SCHEDULE_DELAY", "3600000")
	metricsOut, logs := &countingMetricExporter{}, &closingExporter{}
	rt, traces := startedRuntime(t, func(config *otelboot.Config) {
		config.MetricReader = sdkmetric.NewPeriodicReader(metricsOut, sdkmetric.WithInterval(time.Hour))
		config.LoggerProvider = otelboot.NewLoggerProvider(*config, logs)
	})
	endSpan(rt, "orders.place")
	rt.Instruments().Retries.Add(context.Background(), 1)
	rt.LoggerFor("orders").Info("placed")

	if err := rt.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v, want nil", err)
	}

	if spans, metrics, records := len(traces.GetSpans()), metricsOut.metrics.Load(), logs.exported(); spans != 1 || metrics == 0 || records != 1 {
		t.Fatalf("ForceFlush() exported %d spans, %d metrics and %d log records, want what each of the three pipelines held", spans, metrics, records)
	}
}
