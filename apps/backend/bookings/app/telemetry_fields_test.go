package app_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

var sentinels = []string{"sentinel-pg-password", "sentinel-sasl-user", "sentinel-sasl-password", "sentinel-grpc-key", "sentinel-kafka-key", "sentinel-tenant", "sentinel-migrator-password"}

func configWithSecrets(role app.Role) app.Config {
	cfg := app.Defaults(role)
	cfg.DSN = "postgres://bookings:sentinel-pg-password@db:5432/bookings?sslmode=require"
	cfg.Migrate = true
	cfg.MigrateDSN = "postgres://bookings_migrator:sentinel-migrator-password@db:5432/bookings?sslmode=require"
	cfg.API.GRPCCertFile = "/tls/tls.crt"
	cfg.API.GRPCKeyFile = "/tls/sentinel-grpc-key.pem"
	cfg.API.GRPCClientCAFile = "/tls/ca.crt"
	cfg.API.GRPCTrustedClients = []string{"bff"}
	cfg.MetricTenants = []string{"sentinel-tenant-a", "sentinel-tenant-b"}
	cfg.Brokers = []string{"kafka-1:9093", "kafka-2:9093"}
	cfg.KafkaAuth = kafka.ClientAuth{
		SASL:    &kafka.SASL{Mechanism: kafka.ScramSHA512, Username: "sentinel-sasl-user", Password: "sentinel-sasl-password"},
		KeyFile: "/kafka/sentinel-kafka-key.pem",
	}
	cfg.BookingsTopic = "bookings.v1"
	cfg.BookingsDLQ = "bookings.v1.dlq"
	cfg.Group = "bookings-relay"
	return cfg
}

func TestTheProcessConfiguredRecordCarriesTheAllowlistOfTheRole(t *testing.T) {
	telemetrySubject.RunAllowlist(t, configWithSecrets, map[app.Role][]string{
		app.RoleAPI: {"grpc_addr", "grpc_client_ca_file", "grpc_insecure", "grpc_tls_cert_file", "grpc_tls_key_file",
			"grpc_trusted_clients", "metric_tenants", "migrate", "migrator"},
		app.RoleRelay: {"kafka.ca_file", "kafka.cert_file", "kafka.key_file", "kafka.sasl_mechanism", "kafka.sasl_password",
			"kafka.sasl_username", "kafka_bookings_dlq", "kafka_bookings_topic", "kafka_brokers", "kafka_group", "kafka_insecure"},
	})
}

func TestTheProcessConfiguredRecordCarriesNoSecret(t *testing.T) {
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			record, out := telemetrySubject.ProcessConfigured(t, configWithSecrets(role))

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
				if got := record["dmpf.config.migrator"].GetStringValue(); got != "set" {
					t.Errorf("dmpf.config.migrator = %v, want set", got)
				}
			case app.RoleRelay:
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
	telemetrySubject.RunRolesDeclared(t)
}
