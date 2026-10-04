package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
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
	if cfg.GRPCAddr != ":9090" || cfg.Service != "orders" || cfg.ItemLimit != 10 || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, orders, item limit 10, insecure", cfg)
	}
}

func TestTheAPIRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_INSECURE") {
		t.Fatalf("FromEnv() = %v, want the missing transport policy named", err)
	}
}

func TestTheAPIAcceptsATLSPair(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI,
		lookup("PG_DSN", "postgres://x", "GRPC_TLS_CERT_FILE", "/tls/cert.pem", "GRPC_TLS_KEY_FILE", "/tls/key.pem",
			"GRPC_CLIENT_CA_FILE", "/tls/clients.pem", "GRPC_TRUSTED_CLIENTS", "spiffe://dmpf/bff"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
}

// IDN-03: a TLS server that does not authenticate its caller would read the
// tenant any process on the network chose to send.
func TestATLSPairWithoutClientAuthenticationNamesWhatIsMissing(t *testing.T) {
	pair := []string{"PG_DSN", "postgres://x", "GRPC_TLS_CERT_FILE", "/tls/cert.pem", "GRPC_TLS_KEY_FILE", "/tls/key.pem"}

	if _, err := app.FromEnv(app.RoleAPI, lookup(pair...)); !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_CLIENT_CA_FILE") {
		t.Fatalf("FromEnv() = %v, want GRPC_CLIENT_CA_FILE named", err)
	}
	withCA := append(pair, "GRPC_CLIENT_CA_FILE", "/tls/clients.pem")
	if _, err := app.FromEnv(app.RoleAPI, lookup(withCA...)); !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_TRUSTED_CLIENTS") {
		t.Fatalf("FromEnv() = %v, want GRPC_TRUSTED_CLIENTS named", err)
	}
}

func TestHalfATLSPairNamesTheMissingFile(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI,
		lookup("PG_DSN", "postgres://x", "GRPC_TLS_CERT_FILE", "/tls/cert.pem"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_TLS_KEY_FILE") {
		t.Fatalf("FromEnv() = %v, want GRPC_TLS_KEY_FILE named", err)
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_ORDERS_TOPIC", "orders", "KAFKA_ORDERS_DLQ", "orders.dlq", "KAFKA_GROUP", "g",
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

func TestOrdersRefusesTheConsumerRole(t *testing.T) {
	_, err := app.FromEnv("consumer", lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrUnknownRole) || !strings.Contains(err.Error(), "consumer") || !strings.Contains(err.Error(), "api|relay") {
		t.Fatalf("FromEnv(consumer) = %v, want a refusal naming the role and api|relay", err)
	}
}

func TestAnInvalidItemLimitIsRefused(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI,
		lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true", "ITEM_LIMIT", "0"))

	if !errors.Is(err, app.ErrInvalidVariable) {
		t.Fatalf("FromEnv() = %v, want ErrInvalidVariable", err)
	}
}

func TestTheDefaultsKeepCommandsAndThePurgeWithinTheirRetention(t *testing.T) {
	cfg := app.Defaults(app.RoleAPI)

	if cfg.IdempotencyWait != time.Second || cfg.IdempotencyRetention != 24*time.Hour || cfg.OutboxRetention != 168*time.Hour {
		t.Fatalf("wait %v, retention %v, outbox %v; want 1s, 24h and 168h", cfg.IdempotencyWait, cfg.IdempotencyRetention, cfg.OutboxRetention)
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

func TestFromEnvReadsNoLegacyTelemetryVariable(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true",
		"SERVICE", "legacy", "SERVICE_VERSION", "9.9.9", "INSTANCE_ID", "legacy-1", "OTLP_ENDPOINT", "legacy:4317", "OTLP_INSECURE", "not-a-bool"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want the legacy telemetry variables ignored (RF-E1)", err)
	}
	telemetry := app.TelemetryOf(cfg)
	if telemetry.Service != "orders" || telemetry.Version == "9.9.9" || telemetry.Instance == "legacy-1" {
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
