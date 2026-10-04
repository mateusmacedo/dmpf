package app_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func (e *recordingExporter) eventNames() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make([]string, 0, len(e.records))
	for _, record := range e.records {
		names = append(names, record.EventName())
	}
	return names
}

func TestTheAuditTrailOfTheServiceReachesTheLoggerProviderOfTheRuntime(t *testing.T) {
	ctx := context.Background()
	global := &recordingExporter{}
	previous := otel.GetLoggerProvider()
	otel.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(global))))
	t.Cleanup(func() { otel.SetLoggerProvider(previous) })

	exporter := &recordingExporter{}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "bookings", ServiceVersion: "dev", ServiceInstanceID: "bookings-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	pool, err := pgxpool.New(ctx, "postgres://bookings@127.0.0.1:1/bookings")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want nil", err)
	}
	defer pool.Close()

	service, err := app.NewBookingsService(pool, runtime, app.Defaults(app.RoleAPI))
	if err != nil {
		t.Fatalf("NewBookingsService() = %v, want nil", err)
	}
	service.Instrumentation.Audit(ctx, ports.AuditEvent{Object: "booking-9", Action: "read", Outcome: ports.OutcomeAccepted})
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	if names := exporter.eventNames(); len(names) != 1 || names[0] != audit.EventName {
		t.Fatalf("runtime exported %v, want exactly [%s]", names, audit.EventName)
	}
	if names := global.eventNames(); len(names) != 0 {
		t.Fatalf("global provider exported %v, want nothing", names)
	}
}

func (e *recordingExporter) scopes() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	scopes := make([]string, 0, len(e.records))
	for _, record := range e.records {
		scopes = append(scopes, record.InstrumentationScope().Name)
	}
	return scopes
}

func TestTheRelayRunsOverTheTelemetryOfTheRuntime(t *testing.T) {
	ctx := context.Background()
	logs := &recordingExporter{}
	spans := tracetest.NewInMemoryExporter()
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "bookings", ServiceVersion: "dev", ServiceInstanceID: "bookings-relay-1"},
		TraceExporter: spans,
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	cfg := app.Defaults(app.RoleRelay)

	got := app.RelayConfig(cfg, runtime, nil)

	if got.LogValue().String() != cfg.Relay.LogValue().String() {
		t.Fatalf("RelayConfig() = %v, want the operational values of %v", got.LogValue(), cfg.Relay.LogValue())
	}
	if got.System != "kafka" {
		t.Fatalf("System = %q, want %q", got.System, "kafka")
	}
	if got.MeterProvider == nil || got.MeterProvider != runtime.MeterProvider() {
		t.Fatalf("MeterProvider = %v, want the runtime's %v", got.MeterProvider, runtime.MeterProvider())
	}
	if got.Tracer == nil || got.LoggerProvider != runtime.LoggerProvider() {
		t.Fatalf("Tracer = %v, LoggerProvider = %v, want both from the runtime", got.Tracer, got.LoggerProvider)
	}
	_, span := got.Tracer.Start(ctx, "outbox drain probe")
	span.End()
	logging.NewLogger(got.LoggerProvider, reflect.TypeFor[relay.Relay]().PkgPath()).WarnContext(ctx, "outbox publish failed")
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
	if scopes := logs.scopes(); len(scopes) != 1 || scopes[0] != "github.com/mateusmacedo/dmpf/libs/backend/go/app/relay" {
		t.Fatalf("relay log scopes = %v, want [github.com/mateusmacedo/dmpf/libs/backend/go/app/relay]", scopes)
	}
}

func TestTheRelayAddressesEachChannelByItsPhysicalTopic(t *testing.T) {
	ctx := context.Background()
	runtime, err := otelboot.Start(ctx, otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "bookings", ServiceVersion: "dev", ServiceInstanceID: "bookings-relay-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	})
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(ctx) })
	cfg := app.Defaults(app.RoleRelay)
	cfg.BookingsTopic, cfg.BookingsDLQ, cfg.Group = "bookings.v1", "bookings.v1.dlq", "bookings-relay"
	catalog, err := app.NewCatalog(cfg)
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}

	got := app.RelayConfig(cfg, runtime, catalog)

	if got.Address == nil {
		t.Fatal("Address = nil, want the topic the catalog binds each channel to")
	}
	if topic := got.Address(application.Destination); topic != cfg.BookingsTopic {
		t.Fatalf("Address(%q) = %q, want the physical topic %q (RF-B7)", application.Destination, topic, cfg.BookingsTopic)
	}
	if topic := got.Address("uncatalogued.events"); topic != "" {
		t.Fatalf("Address(uncatalogued.events) = %q, want empty: the send never names a logical channel", topic)
	}
}
