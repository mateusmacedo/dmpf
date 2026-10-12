package app_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

var sentinels = []string{"sentinel-pg-password", "sentinel-sasl-user", "sentinel-sasl-password", "sentinel-grpc-key", "sentinel-kafka-key", "sentinel-tenant"}

func configWithSecrets(role app.Role) app.Config {
	cfg := app.Defaults(role)
	cfg.DSN = "postgres://orders:sentinel-pg-password@db:5432/orders?sslmode=require"
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
	cfg.OrdersTopic = "orders.v1"
	cfg.OrdersDLQ = "orders.v1.dlq"
	cfg.Group = "orders-relay"
	return cfg
}

func TestTheProcessConfiguredRecordCarriesTheAllowlistOfTheRole(t *testing.T) {
	telemetrySubject.RunAllowlist(t, configWithSecrets, map[app.Role][]string{
		app.RoleAPI: {"grpc_addr", "grpc_client_ca_file", "grpc_insecure", "grpc_tls_cert_file", "grpc_tls_key_file",
			"grpc_trusted_clients", "item_limit", "metric_tenants", "migrate"},
		app.RoleRelay: {"kafka.ca_file", "kafka.cert_file", "kafka.key_file", "kafka.sasl_mechanism", "kafka.sasl_password",
			"kafka.sasl_username", "kafka_brokers", "kafka_group", "kafka_insecure", "kafka_orders_dlq", "kafka_orders_topic"},
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
