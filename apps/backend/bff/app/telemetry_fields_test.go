package app_test

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type memoryLogs struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memoryLogs) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *memoryLogs) Shutdown(context.Context) error   { return nil }
func (e *memoryLogs) ForceFlush(context.Context) error { return nil }

var sentinels = []string{"sentinel-client-key", "sentinel-tenant"}

func configWithSecrets() app.Config {
	cfg := app.Defaults()
	cfg.OrdersTarget = "orders:9090"
	cfg.ReservationsTarget = "reservations:9090"
	cfg.BookingsTarget = "bookings:9090"
	cfg.CAFile = "/tls/ca.crt"
	cfg.ServerName = "contexts.internal"
	cfg.ClientCertFile = "/tls/client.crt"
	cfg.ClientKeyFile = "/tls/sentinel-client-key.pem"
	cfg.CORSOrigins = []string{"https://app.example"}
	cfg.MetricTenants = []string{"sentinel-tenant-a", "sentinel-tenant-b"}
	cfg.OrdersContractPath = "/contracts/orders.yaml"
	cfg.ReservationsContractPath = "/contracts/reservations.yaml"
	cfg.BookingsContractPath = "/contracts/bookings.yaml"
	cfg.DrainDelay = 5 * time.Second
	cfg.Auth = authn.Config{Issuer: "https://issuer.example", Audience: "dmpf", TenantClaim: "tenant",
		PermissionClaims: []string{"permissions"}, DiscoveryTimeout: 5 * time.Second}
	return cfg
}

func processConfigured(t *testing.T, cfg app.Config) (map[string]*commonpb.AnyValue, string) {
	t.Helper()
	addr, collector := startCollector(t)
	useOnlyTheOTelEnvironmentOf(t, map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + addr,
		"OTEL_TRACES_EXPORTER":        "none",
		"OTEL_METRICS_EXPORTER":       "none",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_RESOURCE_ATTRIBUTES":    "service.version=test,service.instance.id=bff-1",
	})
	err := boot.Boot(context.Background(), app.TelemetryOf(cfg), func(context.Context, *otelboot.Runtime) error { return nil })
	if err != nil {
		t.Fatalf("Boot() = %v, want nil", err)
	}
	var emitted string
	var configured []map[string]*commonpb.AnyValue
	for _, record := range collector.logRecords() {
		emitted += record.String() + "\n"
		if record.GetBody().GetStringValue() != "process configured" {
			continue
		}
		attributes := map[string]*commonpb.AnyValue{}
		for _, attribute := range record.GetAttributes() {
			attributes[attribute.GetKey()] = attribute.GetValue()
		}
		configured = append(configured, attributes)
	}
	if len(configured) != 1 {
		t.Fatalf("%d \"process configured\" records, want exactly one (RF-A6): %s", len(configured), emitted)
	}
	return configured[0], emitted
}

func TestTheProcessConfiguredRecordCarriesTheAllowlistOfTheEdge(t *testing.T) {
	record, _ := processConfigured(t, configWithSecrets())

	var got []string
	for key := range maps.Keys(record) {
		if setting, ok := strings.CutPrefix(key, "dmpf.config."); ok {
			got = append(got, setting)
		}
	}
	slices.Sort(got)
	want := []string{"admin_addr", "authn.audience", "authn.dev_mock", "authn.discovery_timeout", "authn.issuer", "authn.permission_claims",
		"authn.tenant_claim", "bookings_grpc_target", "cors_origins", "drain_delay", "grpc_ca_file", "grpc_client_cert_file",
		"grpc_client_key_file", "grpc_insecure", "grpc_server_name", "http_addr", "metric_tenants", "openapi_bookings_path",
		"openapi_orders_path", "openapi_reservations_path", "orders_grpc_target", "reservations_grpc_target"}
	if !slices.Equal(got, want) {
		t.Fatalf("dmpf.config.* = %v, want exactly the settings of the edge %v", got, want)
	}
}

func TestTheProcessConfiguredRecordCarriesNoSecret(t *testing.T) {
	record, out := processConfigured(t, configWithSecrets())

	for _, sentinel := range sentinels {
		if strings.Contains(out, sentinel) {
			t.Errorf("the telemetry carries %q (LOG-09): %s", sentinel, out)
		}
	}
	if got := record["dmpf.config.metric_tenants"]; got.GetIntValue() != 2 {
		t.Errorf("dmpf.config.metric_tenants = %v, want the count 2 and not the names", got)
	}
	if got := record["dmpf.config.grpc_client_key_file"].GetStringValue(); got != "set" {
		t.Errorf("dmpf.config.grpc_client_key_file = %v, want set", got)
	}
}

func TestTheTelemetryDeclaresTheApiRole(t *testing.T) {
	telemetry := app.TelemetryOf(app.Defaults())

	if telemetry.Role != "api" {
		t.Errorf("Role = %q, want api (dmpf.process.role, RF-E3)", telemetry.Role)
	}
}

// IDN-20: the line names the tenant the edge authenticated, off the carrier,
// and no tenant when the call resolved none.
func TestARecordNamesTheExecutionOfTheCallFromTheBaggage(t *testing.T) {
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Tenant: &tenant,
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	cfg := app.Defaults()
	telemetry := app.TelemetryOf(cfg)
	logs := &memoryLogs{}
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: telemetry.Service, ServiceVersion: "test",
			ServiceInstanceID: "bff-1", Role: telemetry.Role},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}

	runtime.LoggerFor(reflect.TypeFor[app.Config]().PkgPath()).InfoContext(tracing.WithExecutionBaggage(context.Background(), execution), "scoped")
	runtime.LoggerFor(reflect.TypeFor[app.Config]().PkgPath()).InfoContext(context.Background(), "unscoped")
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	byBody := map[string]map[string]string{}
	logs.mu.Lock()
	for _, record := range logs.records {
		attributes := map[string]string{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.AsString()
			return true
		})
		byBody[record.Body().AsString()] = attributes
	}
	logs.mu.Unlock()
	scoped, unscoped := byBody["scoped"], byBody["unscoped"]
	if scoped == nil || unscoped == nil {
		t.Fatalf("records = %v, want the scoped and the unscoped line", byBody)
	}
	for key, want := range map[string]string{tracing.KeyTenantID: "acme", tracing.KeyCorrelationID: "c-1", tracing.KeyRequestID: "r-1"} {
		if got := scoped[key]; got != want {
			t.Errorf("%s = %q, want %q of the call", key, got, want)
		}
		if got, present := unscoped[key]; present {
			t.Errorf("%s = %q outside a call, want absent", key, got)
		}
	}
}
