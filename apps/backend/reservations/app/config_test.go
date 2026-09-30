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
