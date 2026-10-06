package boot_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

type relaySettings struct{ interval string }

func (r relaySettings) LogValue() slog.Value {
	return slog.GroupValue(slog.String("interval", r.interval), slog.String("password", redact.Placeholder))
}

func recordsNamed(records []exportedLog, message string) []exportedLog {
	var named []exportedLog
	for _, record := range records {
		if record.body == message {
			named = append(named, record)
		}
	}
	return named
}

func TestBootRecordsTheEffectiveConfigurationOnceUnderDmpfConfig(t *testing.T) {
	collector := exportLogsOnly(t)
	telemetry := sampledTelemetry()
	telemetry.Settings = []slog.Attr{
		slog.String("grpc_addr", ":9090"),
		slog.Int("metric_tenants", 2),
		slog.Any("relay", relaySettings{interval: "500ms"}),
	}

	err := boot.Boot(context.Background(), telemetry, func(ctx context.Context, rt *otelboot.Runtime) error {
		rt.LoggerFor(emitterScope).InfoContext(ctx, "work started")
		return nil
	})
	if err != nil {
		t.Fatalf("Boot() = %v, want nil", err)
	}

	exported := collector.logs.exported()
	configured := recordsNamed(exported, "process configured")
	if len(configured) != 1 {
		t.Fatalf("%d \"process configured\" records, want exactly one per process (RF-A6): %+v", len(configured), exported)
	}
	want := map[string]string{
		"dmpf.config.grpc_addr":      ":9090",
		"dmpf.config.relay.interval": "500ms",
		"dmpf.config.relay.password": redact.Placeholder,
	}
	for key, value := range want {
		if configured[0].attributes[key] != value {
			t.Errorf("%s = %v, want %v in %v", key, configured[0].attributes[key], value, configured[0].attributes)
		}
	}
	if got := configured[0].values["dmpf.config.metric_tenants"]; got.GetIntValue() != 2 {
		t.Errorf("dmpf.config.metric_tenants = %v, want the number 2 in %v", got, configured[0].attributes)
	}
	if configured[0].severity != "INFO" {
		t.Errorf("severity = %q, want INFO in %+v", configured[0].severity, configured[0])
	}
	if _, nested := configured[0].attributes["relay"]; nested {
		t.Errorf("record = %v, want every setting under dmpf.config.<field> and none at the root", configured[0].attributes)
	}
	started := recordsNamed(exported, "work started")
	if len(started) == 0 || configured[0].at > started[0].at {
		t.Fatalf("exported %+v, want the configuration recorded before the work starts", exported)
	}
}

func TestStartTelemetryAloneRecordsNoConfiguration(t *testing.T) {
	collector := exportLogsOnly(t)
	telemetry := sampledTelemetry()
	telemetry.Settings = []slog.Attr{slog.String("grpc_addr", ":9090")}

	runtime, err := boot.StartTelemetry(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	if configured := recordsNamed(collector.logs.exported(), "process configured"); len(configured) != 0 {
		t.Fatalf("exported %+v, want the record from Boot alone, so a process has one", configured)
	}
}

func TestTheProcessConfiguredRecordCarriesNoKnownSecret(t *testing.T) {
	secrets := []struct {
		name    string
		setting slog.Attr
		secret  string
	}{
		{"database password", slog.String("pg_password", "sentinel-pg-password"), "sentinel-pg-password"},
		{"sasl password", slog.Group("kafka", slog.String("sasl_password", "sentinel-sasl")), "sentinel-sasl"},
		{"dsn with password", slog.String("pg_dsn", "postgres://orders:sentinel-dsn@db:5432/orders"), "sentinel-dsn"},
		{"token", slog.Group("oidc", slog.String("token", "sentinel-token")), "sentinel-token"},
		{"private key path", slog.String("tls_private_key", "/run/secrets/sentinel-key.pem"), "sentinel-key"},
	}
	for _, tc := range secrets {
		t.Run(tc.name, func(t *testing.T) {
			cleanOTelEnv(t)
			collector := startReceiver(t)
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
			t.Setenv("OTEL_TRACES_EXPORTER", "none")
			t.Setenv("OTEL_METRICS_EXPORTER", "none")
			telemetry := telemetryUnderTest()
			telemetry.Settings = []slog.Attr{slog.String("grpc_addr", ":9090"), tc.setting}

			err := boot.Boot(context.Background(), telemetry, func(context.Context, *otelboot.Runtime) error { return nil })
			if err != nil {
				t.Fatalf("Boot() = %v, want nil", err)
			}

			var configured []exportedLog
			for _, record := range collector.logs.exported() {
				if record.body == "process configured" {
					configured = append(configured, record)
				}
			}
			if len(configured) != 1 {
				t.Fatalf("exported %+v, want one \"process configured\" record", collector.logs.exported())
			}
			if configured[0].attributes["dmpf.config.grpc_addr"] != ":9090" {
				t.Errorf("attributes = %v, want dmpf.config.grpc_addr = :9090", configured[0].attributes)
			}
			for key, value := range configured[0].attributes {
				if strings.Contains(value, tc.secret) {
					t.Errorf("%s = %q carries the secret (LOG-09)", key, value)
				}
			}
		})
	}
}

func TestTheExportedRecordRedactsASensitiveSettingByItsKey(t *testing.T) {
	collector := exportLogsOnly(t)
	telemetry := sampledTelemetry()
	telemetry.Settings = []slog.Attr{
		slog.String("grpc_addr", ":9090"),
		slog.Group("kafka", slog.String("password", "sentinel-kafka-password")),
	}

	err := boot.Boot(context.Background(), telemetry, func(context.Context, *otelboot.Runtime) error { return nil })
	if err != nil {
		t.Fatalf("Boot() = %v, want nil", err)
	}

	exported := collector.logs.exported()
	configured := recordsNamed(exported, "process configured")
	if len(configured) != 1 {
		t.Fatalf("%d \"process configured\" records, want one: %+v", len(configured), exported)
	}
	if got := configured[0].attributes["dmpf.config.kafka.password"]; got != redact.Placeholder {
		t.Errorf("dmpf.config.kafka.password = %v, want %q (LOG-09)", got, redact.Placeholder)
	}
	if got := configured[0].attributes["dmpf.config.grpc_addr"]; got != ":9090" {
		t.Errorf("dmpf.config.grpc_addr = %v, want :9090 intact", got)
	}
	for _, record := range exported {
		if strings.Contains(record.body, "sentinel-kafka-password") {
			t.Errorf("body = %q exported the secret, want no secret anywhere (LOG-09)", record.body)
		}
		for key, value := range record.attributes {
			if strings.Contains(value, "sentinel-kafka-password") {
				t.Errorf("%s = %q exported the secret, want no secret anywhere (LOG-09)", key, value)
			}
		}
	}
}
