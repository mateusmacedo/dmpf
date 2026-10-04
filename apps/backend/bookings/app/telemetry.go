package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
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

func classify(err error) string {
	if failure, ok := errors.AsType[*application.Failure](err); ok {
		return string(failure.Category())
	}
	switch {
	case errors.Is(err, ports.ErrIdempotencyKeyAbsent), errors.Is(err, ports.ErrIdempotencyKeyInvalid),
		errors.Is(err, ports.ErrIdempotencyMismatch):
		return string(application.Validation)
	case errors.Is(err, ports.ErrIdempotencyInFlight), errors.Is(err, ports.ErrAlreadyExists),
		errors.Is(err, ports.ErrVersionConflict):
		return string(application.Conflict)
	case errors.Is(err, ports.ErrNotFound):
		return string(application.NotFound)
	default:
		return obsusecase.CategoryUnclassified
	}
}

// subject is read from the carrier, the one source of identity (CTX-03). Over
// gRPC it is usually absent, because the fan-out never carries it (IDN-02), and
// absent stays absent (IDN-20).
func subject(ctx context.Context) string {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return ""
	}
	who, _ := execution.Subject()
	return string(who)
}

func settings(cfg Config) []slog.Attr {
	storage := []slog.Attr{slog.Any("postgres", postgres.DescribeDSN(cfg.DSN))}
	switch cfg.Role {
	case RoleAPI:
		return append(storage,
			slog.Bool("migrate", cfg.Migrate),
			slog.String("grpc_addr", cfg.GRPCAddr),
			slog.Bool("grpc_insecure", cfg.GRPCInsecure),
			slog.String("grpc_tls_cert_file", cfg.GRPCCertFile),
			slog.String("grpc_tls_key_file", presence(cfg.GRPCKeyFile)),
			slog.String("grpc_client_ca_file", cfg.GRPCClientCAFile),
			slog.String("grpc_trusted_clients", strings.Join(cfg.GRPCTrustedClients, ",")),
			slog.Int("metric_tenants", len(cfg.MetricTenants)),
		)
	case RoleRelay:
		return append(storage, append(kafkaSettings(cfg),
			slog.String("kafka_bookings_topic", cfg.BookingsTopic),
			slog.String("kafka_bookings_dlq", cfg.BookingsDLQ),
		)...)
	default:
		return storage
	}
}

func kafkaSettings(cfg Config) []slog.Attr {
	return []slog.Attr{
		slog.String("kafka_brokers", strings.Join(cfg.Brokers, ",")),
		slog.Bool("kafka_insecure", cfg.KafkaInsecure),
		slog.Any("kafka", cfg.KafkaAuth),
		slog.String("kafka_group", cfg.Group),
	}
}

func presence(value string) string {
	if value == "" {
		return "unset"
	}
	return "set"
}
