package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
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
		Resource:      otelboot.Resource{ServiceName: "orders", ServiceVersion: "dev", ServiceInstanceID: "orders-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	pool, err := pgxpool.New(ctx, "postgres://orders@127.0.0.1:1/orders")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want nil", err)
	}
	defer pool.Close()

	service, err := app.NewOrdersService(pool, runtime, app.Defaults(app.RoleAPI))
	if err != nil {
		t.Fatalf("NewOrdersService() = %v, want nil", err)
	}
	service.Instrumentation.Audit(ctx, ports.AuditEvent{Object: "order-9", Action: "read", Outcome: ports.OutcomeAccepted})
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

func TestTheRelayPublishesThroughKafka(t *testing.T) {
	if system := app.Defaults(app.RoleRelay).Relay.System; system != "kafka" {
		t.Fatalf("Relay.System = %q, want %q", system, "kafka")
	}
}

func TestTheRelayAddressesEachChannelByItsPhysicalTopic(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group = "orders.v1", "orders.v1.dlq", "orders-relay"
	catalog, err := app.NewCatalog(cfg)
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}

	if topic := catalog.AddressOf(application.Destination); topic != cfg.OrdersTopic {
		t.Fatalf("AddressOf(%q) = %q, want the physical topic %q (RF-B7)", application.Destination, topic, cfg.OrdersTopic)
	}
	if topic := catalog.AddressOf("uncatalogued.events"); topic != "" {
		t.Fatalf("AddressOf(uncatalogued.events) = %q, want empty: the send never names a logical channel", topic)
	}
}
