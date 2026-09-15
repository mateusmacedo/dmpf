package dmpfreferenceorders_test

import (
	"errors"
	"strings"
	"testing"

	dmpfreferenceorders "github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-orders-go"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func TestTheAPIRunsWithItsDefaults(t *testing.T) {
	cfg, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleAPI, lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_INSECURE", "true"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.GRPCAddr != ":9090" || cfg.Service != "dmpf-reference-orders" || cfg.ItemLimit != 10 || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, dmpf-reference-orders, item limit 10, insecure", cfg)
	}
}

func TestTheAPIRequiresATransportPolicy(t *testing.T) {
	_, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleAPI, lookup("DMPF_PG_DSN", "postgres://x"))

	if !errors.Is(err, dmpfreferenceorders.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_INSECURE") {
		t.Fatalf("FromEnv() = %v, want the missing transport policy named", err)
	}
}

func TestTheAPIAcceptsATLSPair(t *testing.T) {
	_, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleAPI,
		lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_TLS_CERT_FILE", "/tls/cert.pem", "DMPF_GRPC_TLS_KEY_FILE", "/tls/key.pem"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
}

func TestHalfATLSPairNamesTheMissingFile(t *testing.T) {
	_, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleAPI,
		lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_TLS_CERT_FILE", "/tls/cert.pem"))

	if !errors.Is(err, dmpfreferenceorders.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_TLS_KEY_FILE") {
		t.Fatalf("FromEnv() = %v, want DMPF_GRPC_TLS_KEY_FILE named", err)
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"DMPF_PG_DSN", "postgres://x", "DMPF_KAFKA_BROKERS", "b:9092",
		"DMPF_KAFKA_ORDERS_TOPIC", "orders", "DMPF_KAFKA_ORDERS_DLQ", "orders.dlq", "DMPF_KAFKA_GROUP", "g",
	}
	if _, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleRelay, lookup(full...)); err != nil {
		t.Fatalf("FromEnv(full) = %v, want nil", err)
	}
	for i := 0; i < len(full); i += 2 {
		variable := full[i]
		t.Run("without "+variable, func(t *testing.T) {
			pairs := append(append([]string{}, full[:i]...), full[i+2:]...)

			_, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleRelay, lookup(pairs...))

			if !errors.Is(err, dmpfreferenceorders.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestOrdersRefusesTheConsumerRole(t *testing.T) {
	_, err := dmpfreferenceorders.FromEnv("consumer", lookup("DMPF_PG_DSN", "postgres://x"))

	if !errors.Is(err, dmpfreferenceorders.ErrUnknownRole) || !strings.Contains(err.Error(), "consumer") || !strings.Contains(err.Error(), "api|relay") {
		t.Fatalf("FromEnv(consumer) = %v, want a refusal naming the role and api|relay", err)
	}
}

func TestAnInvalidItemLimitIsRefused(t *testing.T) {
	_, err := dmpfreferenceorders.FromEnv(dmpfreferenceorders.RoleAPI,
		lookup("DMPF_PG_DSN", "postgres://x", "DMPF_GRPC_INSECURE", "true", "DMPF_ITEM_LIMIT", "0"))

	if !errors.Is(err, dmpfreferenceorders.ErrInvalidVariable) {
		t.Fatalf("FromEnv() = %v, want ErrInvalidVariable", err)
	}
}
