package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
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
	if cfg.GRPCAddr != ":9090" || cfg.Service != "bookings" || cfg.Version != "dev" || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, bookings, dev, insecure", cfg)
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

func TestDefaultsAreTheValuesOfARoleWithoutEnvironment(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)

	if cfg.Role != app.RoleRelay || cfg.GRPCAddr != ":9090" || cfg.Relay.BatchSize == 0 {
		t.Fatalf("Defaults(relay) = %+v, want the role, :9090 and a relay configuration", cfg)
	}
}
