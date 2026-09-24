package app_test

import (
	"errors"
	"strings"
	"testing"

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
	cfg, err := app.FromEnv(app.RoleAPI, lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_INSECURE", "true"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.GRPCAddr != ":9090" || cfg.Service != "orders" || cfg.ItemLimit != 10 || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, orders, item limit 10, insecure", cfg)
	}
}

func TestTheAPIRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI, lookup("DMPF_PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_INSECURE") {
		t.Fatalf("FromEnv() = %v, want the missing transport policy named", err)
	}
}

func TestTheAPIAcceptsATLSPair(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI,
		lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_TLS_CERT_FILE", "/tls/cert.pem", "DMPF_GRPC_TLS_KEY_FILE", "/tls/key.pem",
			"DMPF_GRPC_CLIENT_CA_FILE", "/tls/clients.pem", "DMPF_GRPC_TRUSTED_CLIENTS", "spiffe://dmpf/bff"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
}

// IDN-03: a TLS server that does not authenticate its caller would read the
// tenant any process on the network chose to send.
func TestATLSPairWithoutClientAuthenticationNamesWhatIsMissing(t *testing.T) {
	pair := []string{"DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_TLS_CERT_FILE", "/tls/cert.pem", "DMPF_GRPC_TLS_KEY_FILE", "/tls/key.pem"}

	if _, err := app.FromEnv(app.RoleAPI, lookup(pair...)); !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_CLIENT_CA_FILE") {
		t.Fatalf("FromEnv() = %v, want DMPF_GRPC_CLIENT_CA_FILE named", err)
	}
	withCA := append(pair, "DMPF_GRPC_CLIENT_CA_FILE", "/tls/clients.pem")
	if _, err := app.FromEnv(app.RoleAPI, lookup(withCA...)); !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_TRUSTED_CLIENTS") {
		t.Fatalf("FromEnv() = %v, want DMPF_GRPC_TRUSTED_CLIENTS named", err)
	}
}

func TestHalfATLSPairNamesTheMissingFile(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI,
		lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_TLS_CERT_FILE", "/tls/cert.pem"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_TLS_KEY_FILE") {
		t.Fatalf("FromEnv() = %v, want DMPF_GRPC_TLS_KEY_FILE named", err)
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"DMPF_PG_DSN", "postgres://x", "DMPF_KAFKA_BROKERS", "b:9092", "DMPF_KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"DMPF_KAFKA_ORDERS_TOPIC", "orders", "DMPF_KAFKA_ORDERS_DLQ", "orders.dlq", "DMPF_KAFKA_GROUP", "g",
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
	_, err := app.FromEnv("consumer", lookup("DMPF_PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrUnknownRole) || !strings.Contains(err.Error(), "consumer") || !strings.Contains(err.Error(), "api|relay") {
		t.Fatalf("FromEnv(consumer) = %v, want a refusal naming the role and api|relay", err)
	}
}

func TestAnInvalidItemLimitIsRefused(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI,
		lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_INSECURE", "true", "DMPF_ITEM_LIMIT", "0"))

	if !errors.Is(err, app.ErrInvalidVariable) {
		t.Fatalf("FromEnv() = %v, want ErrInvalidVariable", err)
	}
}
