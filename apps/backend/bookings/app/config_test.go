package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func TestTheAPIRunsWithItsDefaults(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.API.GRPCAddr != ":9090" || cfg.Service != "bookings" || !cfg.API.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, bookings, insecure", cfg)
	}
	if cfg.Relay.ShutdownGrace != observability.ShutdownGrace {
		t.Fatalf("Relay.ShutdownGrace = %v, want the kernel's %v", cfg.Relay.ShutdownGrace, observability.ShutdownGrace)
	}
}

func TestTheAPIRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_INSECURE") {
		t.Fatalf("FromEnv() = %v, want the missing transport policy named", err)
	}
}

func TestTheAPIRefusesANonPositivePurgeBatchUnderItsOwnSentinel(t *testing.T) {
	cfg := app.Defaults(app.RoleAPI)
	cfg.DSN, cfg.API.GRPCInsecure = "postgres://x", true
	cfg.Policies.PurgeBatch = 0

	err := cfg.Validate()

	if want := "bookings: invalid idempotency or purge policy: PurgeBatch 0"; !errors.Is(err, app.ErrInvalidPolicy) || err.Error() != want {
		t.Fatalf("Validate() = %v, want %q", err, want)
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_BOOKINGS_TOPIC", "bookings", "KAFKA_BOOKINGS_DLQ", "bookings.dlq", "KAFKA_GROUP", "g",
	}
	if _, err := app.FromEnv(app.RoleRelay, lookup(full...)); err != nil {
		t.Fatalf("FromEnv(full) = %v, want nil", err)
	}
	for i := 0; i < len(full); i += 2 {
		variable := full[i]
		t.Run("without "+variable, func(t *testing.T) {
			pairs := append(append([]string{}, full[:i]...), full[i+2:]...)

			_, err := app.FromEnv(app.RoleRelay, lookup(pairs...))

			if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestBookingsRefusesTheConsumerRole(t *testing.T) {
	_, err := app.FromEnv("consumer", lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrUnknownRole) || !strings.Contains(err.Error(), "consumer") || !strings.Contains(err.Error(), "api|relay") {
		t.Fatalf("FromEnv(consumer) = %v, want a refusal naming the role and api|relay", err)
	}
}

func TestTheDefaultsKeepCommandsAndThePurgeWithinTheirRetention(t *testing.T) {
	want := kernelapp.Policies{
		IdempotencyWait:      time.Second,
		IdempotencyRetention: 24 * time.Hour,
		OutboxRetention:      168 * time.Hour,
		PurgeInterval:        15 * time.Minute,
		PurgeBatch:           1000,
	}
	if got := app.Defaults(app.RoleAPI).Policies; got != want {
		t.Fatalf("Policies = %+v, want %+v", got, want)
	}
}

func TestDefaultsAreTheValuesOfARoleWithoutEnvironment(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)

	if cfg.Role != app.RoleRelay || cfg.API.GRPCAddr != ":9090" || cfg.Relay.BatchSize == 0 {
		t.Fatalf("Defaults(relay) = %+v, want the role, :9090 and a relay configuration", cfg)
	}
}

func TestFromEnvReadsNoLegacyTelemetryVariable(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true",
		"SERVICE", "legacy", "SERVICE_VERSION", "9.9.9", "INSTANCE_ID", "legacy-1", "OTLP_ENDPOINT", "legacy:4317", "OTLP_INSECURE", "not-a-bool"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want the legacy telemetry variables ignored (RF-E1)", err)
	}
	telemetry := app.TelemetryOf(cfg)
	if telemetry.Service != "bookings" || telemetry.Version == "9.9.9" || telemetry.Instance == "legacy-1" {
		t.Fatalf("TelemetryOf() = %+v, want the identity left to OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES (RF-E1, RF-E3)", telemetry)
	}
}

func TestTheIdentityOfTheProcessIsLeftToTheEnvironment(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true"))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if telemetry := app.TelemetryOf(cfg); telemetry.Version != "" || telemetry.Instance != "" {
		t.Fatalf("TelemetryOf() declares version %q and instance %q, want neither: both come from OTEL_RESOURCE_ATTRIBUTES (RF-E3)", telemetry.Version, telemetry.Instance)
	}
}
