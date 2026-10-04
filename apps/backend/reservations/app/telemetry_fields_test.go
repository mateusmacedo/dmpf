package app_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

var sentinels = []string{"sentinel-pg-password", "sentinel-sasl-user", "sentinel-sasl-password", "sentinel-grpc-key", "sentinel-kafka-key", "sentinel-tenant"}

func configWithSecrets(role app.Role) app.Config {
	cfg := app.Defaults(role)
	cfg.DSN = "postgres://reservations:sentinel-pg-password@db:5432/reservations?sslmode=require"
	cfg.GRPCCertFile = "/tls/tls.crt"
	cfg.GRPCKeyFile = "/tls/sentinel-grpc-key.pem"
	cfg.GRPCClientCAFile = "/tls/ca.crt"
	cfg.GRPCTrustedClients = []string{"bff"}
	cfg.MetricTenants = []string{"sentinel-tenant-a", "sentinel-tenant-b"}
	cfg.Brokers = []string{"kafka-1:9093", "kafka-2:9093"}
	cfg.KafkaAuth = kafka.ClientAuth{
		SASL:    &kafka.SASL{Mechanism: kafka.ScramSHA512, Username: "sentinel-sasl-user", Password: "sentinel-sasl-password"},
		KeyFile: "/kafka/sentinel-kafka-key.pem",
	}
	cfg.OrdersTopic = "orders.v1"
	cfg.OrdersDLQ = "orders.v1.dlq"
	cfg.ReservationsTopic = "reservations.v1"
	cfg.ReservationsDLQ = "reservations.v1.dlq"
	cfg.Group = "reservations"
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
		"OTEL_RESOURCE_ATTRIBUTES":    "service.version=test,service.instance.id=reservations-1",
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

func settingKeys(record map[string]*commonpb.AnyValue) []string {
	var keys []string
	for key := range maps.Keys(record) {
		if setting, ok := strings.CutPrefix(key, "dmpf.config."); ok {
			keys = append(keys, setting)
		}
	}
	slices.Sort(keys)
	return keys
}

var postgresSettings = []string{"postgres.database", "postgres.host", "postgres.port", "postgres.sslmode", "postgres.user"}

func TestTheProcessConfiguredRecordCarriesTheAllowlistOfTheRole(t *testing.T) {
	for role, own := range map[app.Role][]string{
		app.RoleAPI: {"grpc_addr", "grpc_client_ca_file", "grpc_insecure", "grpc_tls_cert_file", "grpc_tls_key_file",
			"grpc_trusted_clients", "metric_tenants", "migrate"},
		app.RoleRelay: {"kafka.ca_file", "kafka.cert_file", "kafka.key_file", "kafka.sasl_mechanism", "kafka.sasl_password",
			"kafka.sasl_username", "kafka_brokers", "kafka_group", "kafka_insecure", "kafka_reservations_dlq",
			"kafka_reservations_topic"},
		app.RoleConsumer: {"kafka.ca_file", "kafka.cert_file", "kafka.key_file", "kafka.sasl_mechanism", "kafka.sasl_password",
			"kafka.sasl_username", "kafka_brokers", "kafka_group", "kafka_insecure", "kafka_orders_dlq", "kafka_orders_topic",
			"orders_source"},
	} {
		t.Run(string(role), func(t *testing.T) {
			record, _ := processConfigured(t, configWithSecrets(role))

			if keys := settingKeys(record); slices.ContainsFunc(keys, func(key string) bool { return strings.HasPrefix(key, "relay.") }) {
				t.Fatalf("dmpf.config.* = %v, want no relay.*: code constants are not settings (RF-A6)", keys)
			}
			want := slices.Sorted(slices.Values(append(own, postgresSettings...)))
			if got := settingKeys(record); !slices.Equal(got, want) {
				t.Fatalf("dmpf.config.* = %v, want exactly the settings of the %s role %v", got, role, want)
			}
			if got := record["dmpf.config.postgres.user"].GetStringValue(); got != "set" {
				t.Errorf("dmpf.config.postgres.user = %v, want set", got)
			}
		})
	}
}

func TestTheProcessConfiguredRecordCarriesNoSecret(t *testing.T) {
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			record, out := processConfigured(t, configWithSecrets(role))

			for _, sentinel := range sentinels {
				if strings.Contains(out, sentinel) {
					t.Errorf("the telemetry carries %q (LOG-09): %s", sentinel, out)
				}
			}
			switch role {
			case app.RoleAPI:
				if got := record["dmpf.config.metric_tenants"]; got.GetIntValue() != 2 {
					t.Errorf("dmpf.config.metric_tenants = %v, want the count 2 and not the names", got)
				}
				if got := record["dmpf.config.grpc_tls_key_file"].GetStringValue(); got != "set" {
					t.Errorf("dmpf.config.grpc_tls_key_file = %v, want set", got)
				}
			case app.RoleRelay, app.RoleConsumer:
				if got := record["dmpf.config.kafka.sasl_password"].GetStringValue(); got != redact.Placeholder {
					t.Errorf("dmpf.config.kafka.sasl_password = %v, want %q", got, redact.Placeholder)
				}
				if got := record["dmpf.config.kafka.key_file"].GetStringValue(); got != "set" {
					t.Errorf("dmpf.config.kafka.key_file = %v, want set", got)
				}
			}
		})
	}
}

func TestTheTelemetryDeclaresTheRole(t *testing.T) {
	for _, role := range app.Roles {
		telemetry := app.TelemetryOf(app.Defaults(role))

		if telemetry.Role != string(role) {
			t.Errorf("Role = %q, want %q (dmpf.process.role, RF-E3)", telemetry.Role, role)
		}
	}
}
