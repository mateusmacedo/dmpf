package obstest

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

// Subject is the telemetry of an app: its configuration C, built per role R,
// as the binary reads and boots it.
type Subject[C any, R ~string] struct {
	Service     string
	Roles       []R
	API         R
	Defaults    func(R) C
	FromEnv     func(R, func(string) string) (C, error)
	TelemetryOf func(C) boot.Telemetry
	Scope       string
}

var PostgresSettings = []string{"postgres.database", "postgres.host", "postgres.port", "postgres.sslmode", "postgres.user"}

// ProcessConfigured boots cfg against a collector and answers the attributes of
// the one "process configured" record (RF-A6) and everything the logs carried.
func (s Subject[C, R]) ProcessConfigured(t *testing.T, cfg C) (map[string]*commonpb.AnyValue, string) {
	t.Helper()
	addr, collector := StartCollector(t)
	UseOnlyTheOTelEnvironmentOf(t, map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + addr,
		"OTEL_TRACES_EXPORTER":        "none",
		"OTEL_METRICS_EXPORTER":       "none",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_RESOURCE_ATTRIBUTES":    "service.version=test,service.instance.id=" + s.Service + "-1",
	})
	err := boot.Boot(context.Background(), s.TelemetryOf(cfg), func(context.Context, *otelboot.Runtime) error { return nil })
	if err != nil {
		t.Fatalf("Boot() = %v, want nil", err)
	}
	var emitted strings.Builder
	var configured []map[string]*commonpb.AnyValue
	for _, record := range collector.LogRecords() {
		emitted.WriteString(record.String() + "\n")
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
		t.Fatalf("%d \"process configured\" records, want exactly one (RF-A6): %s", len(configured), emitted.String())
	}
	return configured[0], emitted.String()
}

func SettingKeys(record map[string]*commonpb.AnyValue) []string {
	var keys []string
	for key := range record {
		if setting, ok := strings.CutPrefix(key, "dmpf.config."); ok {
			keys = append(keys, setting)
		}
	}
	slices.Sort(keys)
	return keys
}

// RunAllowlist holds the process configured record of each role to exactly its
// own settings plus the Postgres ones, never a code constant.
func (s Subject[C, R]) RunAllowlist(t *testing.T, configOf func(R) C, own map[R][]string) {
	t.Helper()
	for role, settings := range own {
		t.Run(string(role), func(t *testing.T) {
			record, _ := s.ProcessConfigured(t, configOf(role))

			if keys := SettingKeys(record); slices.ContainsFunc(keys, func(key string) bool { return strings.HasPrefix(key, "relay.") }) {
				t.Fatalf("dmpf.config.* = %v, want no relay.*: code constants are not settings (RF-A6)", keys)
			}
			want := slices.Sorted(slices.Values(append(slices.Clone(settings), PostgresSettings...)))
			if got := SettingKeys(record); !slices.Equal(got, want) {
				t.Fatalf("dmpf.config.* = %v, want exactly the settings of the %s role %v", got, role, want)
			}
			if got := record["dmpf.config.postgres.user"].GetStringValue(); got != "set" {
				t.Errorf("dmpf.config.postgres.user = %v, want set", got)
			}
		})
	}
}

func (s Subject[C, R]) RunRolesDeclared(t *testing.T) {
	t.Helper()
	for _, role := range s.Roles {
		if telemetry := s.TelemetryOf(s.Defaults(role)); telemetry.Role != string(role) {
			t.Errorf("Role = %q, want %q (dmpf.process.role, RF-E3)", telemetry.Role, role)
		}
	}
}

func (s Subject[C, R]) recordsOfTheProcess(t *testing.T, emit func(ctx context.Context, logger *slog.Logger)) []map[string]string {
	t.Helper()
	telemetry := s.TelemetryOf(s.Defaults(s.API))
	logs := &MemoryLogs{}
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: telemetry.Service, ServiceVersion: "test",
			ServiceInstanceID: s.Service + "-api-1", Role: telemetry.Role},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	emit(context.Background(), runtime.LoggerFor(s.Scope))
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	var all []map[string]string
	for _, record := range logs.Records() {
		attributes := map[string]string{"body": record.Body().AsString()}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.AsString()
			return true
		})
		all = append(all, attributes)
	}
	return all
}

// RunTenantBaggage is IDN-20: the line names the tenant of the call, off the
// carrier, and no tenant when the call resolved none.
func (s Subject[C, R]) RunTenantBaggage(t *testing.T) {
	t.Helper()
	execution := execctx.Execution(t, execctx.WithoutSubject())

	records := s.recordsOfTheProcess(t, func(ctx context.Context, logger *slog.Logger) {
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
	for key, want := range map[string]string{tracing.KeyTenantID: "acme", tracing.KeyCorrelationID: "c-test", tracing.KeyRequestID: "r-test"} {
		if got := scoped[key]; got != want {
			t.Errorf("%s = %q, want %q of the call", key, got, want)
		}
		if got, present := unscoped[key]; present {
			t.Errorf("%s = %q outside a call, want absent: no value is invented for it", key, got)
		}
	}
}

// AuditTrail proves the audit of a service goes through the logger provider of
// the runtime that built it and never through the global one.
func AuditTrail(t *testing.T, service string, emit func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error) {
	t.Helper()
	ctx := context.Background()
	global := &MemoryLogs{}
	previous := otel.GetLoggerProvider()
	otel.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(global))))
	t.Cleanup(func() { otel.SetLoggerProvider(previous) })

	exporter := &MemoryLogs{}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: service, ServiceVersion: "dev", ServiceInstanceID: service + "-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	pool, err := pgxpool.New(ctx, "postgres://"+service+"@127.0.0.1:1/"+service)
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want nil", err)
	}
	defer pool.Close()

	if err := emit(ctx, runtime, pool); err != nil {
		t.Fatalf("emit the audit = %v, want nil", err)
	}
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	if names := exporter.EventNames(); len(names) != 1 || names[0] != audit.EventName {
		t.Fatalf("runtime exported %v, want exactly [%s]", names, audit.EventName)
	}
	if names := global.EventNames(); len(names) != 0 {
		t.Fatalf("global provider exported %v, want nothing", names)
	}
}
