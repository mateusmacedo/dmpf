package relay

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/log"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

type noTelemetry struct{}

func (noTelemetry) Tracer() trace.Tracer                { return tracenoop.NewTracerProvider().Tracer("") }
func (noTelemetry) MeterProvider() metric.MeterProvider { return metricnoop.NewMeterProvider() }
func (noTelemetry) LoggerProvider() log.LoggerProvider  { return lognoop.NewLoggerProvider() }

func TestTheRelayRunsOverTheTelemetryOfTheRuntime(t *testing.T) {
	ctx := context.Background()
	logs := &recordingExporter{}
	spans := tracetest.NewInMemoryExporter()
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "orders", ServiceVersion: "dev", ServiceInstanceID: "orders-relay-1"},
		TraceExporter: spans,
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	operational := validConfig()
	operational.System = "kafka"

	got := Instrument(operational, runtime, nil)

	if got.LogValue().String() != operational.LogValue().String() {
		t.Fatalf("Instrument() = %v, want the operational values of %v", got.LogValue(), operational.LogValue())
	}
	if got.System != "kafka" {
		t.Fatalf("System = %q, want the one the context declared", got.System)
	}
	if got.MeterProvider == nil || got.MeterProvider != runtime.MeterProvider() {
		t.Fatalf("MeterProvider = %v, want the runtime's %v", got.MeterProvider, runtime.MeterProvider())
	}
	if got.Tracer == nil || got.LoggerProvider != runtime.LoggerProvider() {
		t.Fatalf("Tracer = %v, LoggerProvider = %v, want both from the runtime", got.Tracer, got.LoggerProvider)
	}
	_, span := got.Tracer.Start(ctx, "outbox drain probe")
	span.End()
	logging.NewLogger(got.LoggerProvider, relayScope).WarnContext(ctx, publishFailed)
	if err := runtime.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush() = %v, want nil", err)
	}
	exported := spans.GetSpans()
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	if len(exported) != 1 || exported[0].Name != "outbox drain probe" {
		t.Fatalf("runtime exported spans %v, want the relay's probe", exported.Snapshots())
	}
	if len(logs.records) != 1 || logs.records[0].InstrumentationScope().Name != relayScope {
		t.Fatalf("relay log records = %d, want one under %s", len(logs.records), relayScope)
	}
}

func TestInstrumentAddressesEachDestinationThroughTheGivenFunction(t *testing.T) {
	address := func(destination string) string { return "topic." + destination }

	got := Instrument(validConfig(), noTelemetry{}, address)

	if got.Address == nil || got.Address("orders") != "topic.orders" {
		t.Fatal("Address does not resolve through the function the context gave")
	}
}
