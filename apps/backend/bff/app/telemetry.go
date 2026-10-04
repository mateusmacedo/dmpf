package app

import (
	"log/slog"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const processRole = "api"

// TelemetryOf is what this process declares about itself to the telemetry.
func TelemetryOf(cfg Config) boot.Telemetry {
	return boot.Telemetry{
		Service:  cfg.Service,
		Role:     processRole,
		Signals:  cfg.Signals,
		Class:    tracing.ClassWrite,
		Settings: settings(cfg),
	}
}

func settings(cfg Config) []slog.Attr {
	return []slog.Attr{
		slog.String("http_addr", cfg.HTTPAddr),
		slog.String("admin_addr", cfg.AdminAddr),
		slog.String("orders_grpc_target", cfg.OrdersTarget),
		slog.String("reservations_grpc_target", cfg.ReservationsTarget),
		slog.String("bookings_grpc_target", cfg.BookingsTarget),
		slog.Bool("grpc_insecure", cfg.GRPCInsecure),
		slog.String("grpc_ca_file", cfg.CAFile),
		slog.String("grpc_server_name", cfg.ServerName),
		slog.String("grpc_client_cert_file", cfg.ClientCertFile),
		slog.String("grpc_client_key_file", presence(cfg.ClientKeyFile)),
		slog.String("cors_origins", strings.Join(cfg.CORSOrigins, ",")),
		slog.Int("metric_tenants", len(cfg.MetricTenants)),
		slog.String("openapi_orders_path", cfg.OrdersContractPath),
		slog.String("openapi_reservations_path", cfg.ReservationsContractPath),
		slog.String("openapi_bookings_path", cfg.BookingsContractPath),
		slog.String("drain_delay", cfg.DrainDelay.String()),
		slog.Any("authn", cfg.Auth),
	}
}

func presence(value string) string {
	if value == "" {
		return "unset"
	}
	return "set"
}
