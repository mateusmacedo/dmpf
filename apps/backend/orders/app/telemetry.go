package app

import (
	"log/slog"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// TelemetryOf is what this process declares about itself to the telemetry.
func TelemetryOf(cfg Config) boot.Telemetry {
	return boot.Telemetry{
		Service:  cfg.Service,
		Role:     string(cfg.Role),
		Signals:  cfg.Signals,
		Class:    tracing.ClassWrite,
		Settings: settings(cfg),
	}
}

func settings(cfg Config) []slog.Attr {
	storage := []slog.Attr{slog.Any("postgres", postgres.DescribeDSN(cfg.DSN))}
	switch cfg.Role {
	case RoleAPI:
		return append(storage,
			slog.Bool("migrate", cfg.Migrate),
			slog.String("migrator", presence(cfg.MigrateDSN)),
			slog.String("grpc_addr", cfg.API.GRPCAddr),
			slog.Bool("grpc_insecure", cfg.API.GRPCInsecure),
			slog.String("grpc_tls_cert_file", cfg.API.GRPCCertFile),
			slog.String("grpc_tls_key_file", presence(cfg.API.GRPCKeyFile)),
			slog.String("grpc_client_ca_file", cfg.API.GRPCClientCAFile),
			slog.String("grpc_trusted_clients", strings.Join(cfg.API.GRPCTrustedClients, ",")),
			slog.Int("metric_tenants", len(cfg.MetricTenants)),
			slog.Int("item_limit", cfg.ItemLimit),
		)
	case RoleRelay:
		return append(storage,
			slog.String("kafka_brokers", strings.Join(cfg.Brokers, ",")),
			slog.Bool("kafka_insecure", cfg.KafkaInsecure),
			slog.Any("kafka", cfg.KafkaAuth),
			slog.String("kafka_orders_topic", cfg.OrdersTopic),
			slog.String("kafka_orders_dlq", cfg.OrdersDLQ),
			slog.String("kafka_group", cfg.Group),
		)
	default:
		return storage
	}
}

func presence(value string) string {
	if value == "" {
		return "unset"
	}
	return "set"
}
