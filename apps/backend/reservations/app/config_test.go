package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
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
	if cfg.GRPCAddr != ":9090" || cfg.Service != "reservations" || cfg.Wait != 2*time.Second || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, reservations, wait 2s, insecure", cfg)
	}
}

func TestTheAPIRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_INSECURE") {
		t.Fatalf("FromEnv() = %v, want the missing transport policy named", err)
	}
}

func requireEachVariable(t *testing.T, role app.Role, full []string) {
	t.Helper()
	if _, err := app.FromEnv(role, lookup(full...)); err != nil {
		t.Fatalf("FromEnv(%s, full) = %v, want nil", role, err)
	}
	for i := 0; i < len(full); i += 2 {
		variable := full[i]
		t.Run(string(role)+" without "+variable, func(t *testing.T) {
			pairs := append(append([]string{}, full[:i]...), full[i+2:]...)

			_, err := app.FromEnv(role, lookup(pairs...))

			if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	requireEachVariable(t, app.RoleRelay, []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_RESERVATIONS_TOPIC", "reservations", "KAFKA_RESERVATIONS_DLQ", "reservations.dlq", "KAFKA_GROUP", "g",
	})
}

func TestTheConsumerNamesEachMissingVariable(t *testing.T) {
	requireEachVariable(t, app.RoleConsumer, []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_ORDERS_TOPIC", "orders", "KAFKA_ORDERS_DLQ", "orders.dlq", "KAFKA_GROUP", "g",
	})
}

func TestAnUnknownRoleIsRefused(t *testing.T) {
	_, err := app.FromEnv("worker", lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrUnknownRole) || !strings.Contains(err.Error(), "api|relay|consumer") {
		t.Fatalf("FromEnv(worker) = %v, want a refusal listing api|relay|consumer", err)
	}
}

func TestTheDefaultsKeepCommandsMessagesAndThePurgeWithinTheirRetention(t *testing.T) {
	cfg := app.Defaults(app.RoleConsumer)

	if cfg.IdempotencyWait != time.Second || cfg.IdempotencyRetention != 24*time.Hour {
		t.Fatalf("wait %v, retention %v; want 1s and 24h", cfg.IdempotencyWait, cfg.IdempotencyRetention)
	}
	if cfg.OutboxRetention != 168*time.Hour || cfg.InboxRetention != 192*time.Hour {
		t.Fatalf("outbox %v, inbox %v; want 168h and 192h", cfg.OutboxRetention, cfg.InboxRetention)
	}
	if cfg.PurgeInterval != 15*time.Minute || cfg.PurgeBatch != 1000 {
		t.Fatalf("purge every %v in batches of %d, want 15m and 1000", cfg.PurgeInterval, cfg.PurgeBatch)
	}
}

func TestANonPositivePolicyIsRefused(t *testing.T) {
	for name, spoil := range map[string]func(*app.Config){
		"wait":             func(c *app.Config) { c.IdempotencyWait = 0 },
		"retention":        func(c *app.Config) { c.IdempotencyRetention = -time.Hour },
		"outbox retention": func(c *app.Config) { c.OutboxRetention = 0 },
		"inbox retention":  func(c *app.Config) { c.InboxRetention = 0 },
		"purge interval":   func(c *app.Config) { c.PurgeInterval = 0 },
		"purge batch":      func(c *app.Config) { c.PurgeBatch = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := app.Defaults(app.RoleAPI)
			cfg.DSN, cfg.GRPCInsecure = "postgres://x", true
			spoil(&cfg)

			if err := cfg.Validate(); !errors.Is(err, app.ErrInvalidPolicy) {
				t.Fatalf("Validate() = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestTheConsumerRefusesAnInboxRetentionShorterThanTheRedeliveryWindow(t *testing.T) {
	env := lookup("PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_ORDERS_TOPIC", "orders", "KAFKA_ORDERS_DLQ", "orders.dlq", "KAFKA_GROUP", "g")
	cfg, err := app.FromEnv(app.RoleConsumer, env)
	if err != nil {
		t.Fatalf("FromEnv(consumer) = %v, want nil with the default retention", err)
	}

	cfg.InboxRetention = 6 * 24 * time.Hour
	if err := cfg.Validate(); !errors.Is(err, app.ErrInvalidPolicy) || !strings.Contains(err.Error(), "INB-14") {
		t.Fatalf("Validate() = %v, want ErrInvalidPolicy naming INB-14", err)
	}
	cfg.Role, cfg.GRPCInsecure = app.RoleAPI, true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(api) = %v, want nil: only the consumer keeps message entries", err)
	}
}

func TestFromEnvReadsNoLegacyTelemetryVariable(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true",
		"SERVICE", "legacy", "SERVICE_VERSION", "9.9.9", "INSTANCE_ID", "legacy-1", "OTLP_ENDPOINT", "legacy:4317", "OTLP_INSECURE", "not-a-bool"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want the legacy telemetry variables ignored (RF-E1)", err)
	}
	telemetry := app.TelemetryOf(cfg)
	if telemetry.Service != "reservations" || telemetry.Version == "9.9.9" || telemetry.Instance == "legacy-1" {
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
