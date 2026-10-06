package app_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func recordsOfTheProcess(t *testing.T, emit func(ctx context.Context, logger *slog.Logger)) []map[string]string {
	t.Helper()
	cfg := app.Defaults(app.RoleAPI)
	telemetry := app.TelemetryOf(cfg)
	logs := &recordingExporter{}
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: telemetry.Service, ServiceVersion: "test",
			ServiceInstanceID: "bookings-api-1", Role: telemetry.Role},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}

	emit(context.Background(), runtime.LoggerFor(reflect.TypeFor[app.Config]().PkgPath()))
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	logs.mu.Lock()
	defer logs.mu.Unlock()
	all := make([]map[string]string, 0, len(logs.records))
	for _, record := range logs.records {
		attributes := map[string]string{"body": record.Body().AsString()}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.AsString()
			return true
		})
		all = append(all, attributes)
	}
	return all
}

// IDN-20: the line names the tenant of the call, off the carrier, and no tenant
// when the call resolved none; a fixed value would put one tenancy's name on
// another tenant's lines.
func TestARecordNamesTheExecutionOfTheCallFromTheBaggage(t *testing.T) {
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Tenant: &tenant,
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}

	records := recordsOfTheProcess(t, func(ctx context.Context, logger *slog.Logger) {
		logger.InfoContext(tracing.WithExecutionBaggage(ctx, execution), "scoped")
		logger.InfoContext(ctx, "unscoped")
	})

	byBody := map[string]map[string]string{}
	for _, record := range records {
		byBody[record["body"]] = record
	}
	scoped, unscoped := byBody["scoped"], byBody["unscoped"]
	if scoped == nil || unscoped == nil {
		t.Fatalf("records = %v, want the scoped and the unscoped line", records)
	}
	for key, want := range map[string]string{tracing.KeyTenantID: "acme", tracing.KeyCorrelationID: "c-1", tracing.KeyRequestID: "r-1"} {
		if got := scoped[key]; got != want {
			t.Errorf("%s = %q, want %q of the call", key, got, want)
		}
		if got, present := unscoped[key]; present {
			t.Errorf("%s = %q outside a call, want absent: no value is invented for it", key, got)
		}
	}
}
