package dmpfreference_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	dmpfreference "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/apps/backend/dmpf-reference"
)

const testDSN = "postgres://app:app@localhost:5432/app?sslmode=disable"

func env(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func requireMissing(t *testing.T, err error, variable string) {
	t.Helper()
	if !errors.Is(err, dmpfreference.ErrMissingVariable) {
		t.Fatalf("err = %v, want ErrMissingVariable naming %s", err, variable)
	}
	if !strings.Contains(err.Error(), variable) {
		t.Fatalf("err = %q does not name %s", err, variable)
	}
}

func TestConfigAPIDefaults(t *testing.T) {
	cfg, err := dmpfreference.FromEnv(dmpfreference.RoleAPI, env("DMPF_PG_DSN", testDSN))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if cfg.Role != dmpfreference.RoleAPI || cfg.DSN != testDSN {
		t.Fatalf("cfg = %+v, want role api over the DSN", cfg)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want \":8080\"", cfg.HTTPAddr)
	}
	if cfg.Service != "dmpf-reference" || cfg.Version != "dev" || cfg.Instance == "" {
		t.Fatalf("resource = (%q, %q, %q), want (dmpf-reference, dev, non-empty instance)", cfg.Service, cfg.Version, cfg.Instance)
	}
	if cfg.ItemLimit != 10 {
		t.Fatalf("ItemLimit = %d, want 10", cfg.ItemLimit)
	}
	if cfg.Migrate || cfg.KafkaInsecure || cfg.OTLPInsecure {
		t.Fatalf("booleans default to false, got migrate=%v kafkaInsecure=%v otlpInsecure=%v", cfg.Migrate, cfg.KafkaInsecure, cfg.OTLPInsecure)
	}
	if cfg.OTLPEndpoint != "" {
		t.Fatalf("OTLPEndpoint = %q, want empty (telemetry in memory)", cfg.OTLPEndpoint)
	}
	if cfg.OpenAPIPath != "" || len(cfg.CORSOrigins) != 0 {
		t.Fatalf("OpenAPIPath = %q, CORSOrigins = %v, want both off by default", cfg.OpenAPIPath, cfg.CORSOrigins)
	}
	if cfg.Wait != 2*time.Second {
		t.Fatalf("Wait = %v, want 2s", cfg.Wait)
	}
	if cfg.Admission.PerSecond != 50 || cfg.Admission.Burst != 100 || cfg.Admission.Concurrency != 32 {
		t.Fatalf("Admission = %+v, want {50 100 32}", cfg.Admission)
	}
	if cfg.Budget.Limit != 2*time.Second || cfg.Budget.Slack != 200*time.Millisecond || cfg.Budget.EstimatedDuration != 100*time.Millisecond {
		t.Fatalf("Budget = %+v, want {2s 200ms 100ms}", cfg.Budget)
	}
	if err := cfg.Relay.Validate(); err != nil {
		t.Fatalf("Relay defaults do not validate: %v", err)
	}
	if cfg.Relay.Source != "urn:dmpf:reference" || cfg.Relay.BatchSize != 50 || cfg.Relay.MaxAttempts != 10 {
		t.Fatalf("Relay = %+v, want source urn:dmpf:reference, batch 50, attempts 10", cfg.Relay)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestConfigOverridesFromTheEnvironment(t *testing.T) {
	cfg, err := dmpfreference.FromEnv(dmpfreference.RoleConsumer, env(
		"DMPF_PG_DSN", testDSN,
		"DMPF_HTTP_ADDR", "127.0.0.1:9090",
		"DMPF_KAFKA_BROKERS", "a:9092, b:9092,,c:9092",
		"DMPF_KAFKA_INSECURE", "true",
		"DMPF_KAFKA_TOPIC", "orders.events",
		"DMPF_KAFKA_GROUP", "reservations",
		"DMPF_KAFKA_DLQ", "orders.events.dlq",
		"DMPF_KAFKA_RESERVATIONS_TOPIC", "reservations.events",
		"DMPF_KAFKA_RESERVATIONS_DLQ", "reservations.events.dlq",
		"DMPF_OPENAPI_PATH", "/contracts/openapi/orders/v1/openapi.yaml",
		"DMPF_CORS_ORIGINS", "http://localhost:8082, http://127.0.0.1:8082",
		"DMPF_OTLP_ENDPOINT", "collector:4317",
		"DMPF_OTLP_INSECURE", "1",
		"DMPF_MIGRATE", "true",
		"DMPF_SERVICE", "orders-edge",
		"DMPF_SERVICE_VERSION", "1.2.3",
		"DMPF_INSTANCE_ID", "pod-7",
		"DMPF_ITEM_LIMIT", "3",
	))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if want := []string{"a:9092", "b:9092", "c:9092"}; !slices.Equal(cfg.Brokers, want) {
		t.Fatalf("Brokers = %v, want %v (trimmed, empties dropped)", cfg.Brokers, want)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.Topic != "orders.events" || cfg.Group != "reservations" || cfg.DLQ != "orders.events.dlq" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.ReservationsTopic != "reservations.events" || cfg.ReservationsDLQ != "reservations.events.dlq" {
		t.Fatalf("reservations channel = (%q, %q), want the two variables", cfg.ReservationsTopic, cfg.ReservationsDLQ)
	}
	if cfg.OpenAPIPath != "/contracts/openapi/orders/v1/openapi.yaml" {
		t.Fatalf("OpenAPIPath = %q", cfg.OpenAPIPath)
	}
	if want := []string{"http://localhost:8082", "http://127.0.0.1:8082"}; !slices.Equal(cfg.CORSOrigins, want) {
		t.Fatalf("CORSOrigins = %v, want %v (trimmed)", cfg.CORSOrigins, want)
	}
	if !cfg.KafkaInsecure || !cfg.OTLPInsecure || !cfg.Migrate {
		t.Fatalf("booleans = kafkaInsecure=%v otlpInsecure=%v migrate=%v, want all true", cfg.KafkaInsecure, cfg.OTLPInsecure, cfg.Migrate)
	}
	if cfg.OTLPEndpoint != "collector:4317" || cfg.Service != "orders-edge" || cfg.Version != "1.2.3" || cfg.Instance != "pod-7" || cfg.ItemLimit != 3 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestConfigRequiresTheVariablesOfEachRole(t *testing.T) {
	full := []string{
		"DMPF_PG_DSN", testDSN,
		"DMPF_KAFKA_BROKERS", "localhost:9092",
		"DMPF_KAFKA_TOPIC", "orders.events",
		"DMPF_KAFKA_GROUP", "reservations",
		"DMPF_KAFKA_DLQ", "orders.events.dlq",
		"DMPF_KAFKA_RESERVATIONS_TOPIC", "reservations.events",
		"DMPF_KAFKA_RESERVATIONS_DLQ", "reservations.events.dlq",
	}
	without := func(name string) func(string) string {
		lookup := env(full...)
		return func(key string) string {
			if key == name {
				return ""
			}
			return lookup(key)
		}
	}

	cases := []struct {
		role     dmpfreference.Role
		variable string
		required bool
	}{
		{dmpfreference.RoleAPI, "DMPF_PG_DSN", true},
		{dmpfreference.RoleAPI, "DMPF_KAFKA_BROKERS", false},
		{dmpfreference.RoleAPI, "DMPF_KAFKA_TOPIC", false},
		{dmpfreference.RoleAPI, "DMPF_KAFKA_RESERVATIONS_TOPIC", false},
		{dmpfreference.RoleRelay, "DMPF_PG_DSN", true},
		{dmpfreference.RoleRelay, "DMPF_KAFKA_BROKERS", true},
		{dmpfreference.RoleRelay, "DMPF_KAFKA_TOPIC", true},
		{dmpfreference.RoleRelay, "DMPF_KAFKA_DLQ", true},
		{dmpfreference.RoleRelay, "DMPF_KAFKA_RESERVATIONS_TOPIC", true},
		{dmpfreference.RoleRelay, "DMPF_KAFKA_RESERVATIONS_DLQ", true},
		// Every Kafka channel declares a group (KFK-07), so the relay needs it too.
		{dmpfreference.RoleRelay, "DMPF_KAFKA_GROUP", true},
		{dmpfreference.RoleConsumer, "DMPF_PG_DSN", true},
		{dmpfreference.RoleConsumer, "DMPF_KAFKA_BROKERS", true},
		{dmpfreference.RoleConsumer, "DMPF_KAFKA_TOPIC", true},
		{dmpfreference.RoleConsumer, "DMPF_KAFKA_GROUP", true},
		{dmpfreference.RoleConsumer, "DMPF_KAFKA_DLQ", true},
		{dmpfreference.RoleConsumer, "DMPF_KAFKA_RESERVATIONS_TOPIC", true},
		{dmpfreference.RoleConsumer, "DMPF_KAFKA_RESERVATIONS_DLQ", true},
	}
	for _, tc := range cases {
		t.Run(string(tc.role)+" without "+tc.variable, func(t *testing.T) {
			_, err := dmpfreference.FromEnv(tc.role, without(tc.variable))
			if !tc.required {
				if err != nil {
					t.Fatalf("FromEnv() = %v, want nil — %s is optional for %s", err, tc.variable, tc.role)
				}
				return
			}
			requireMissing(t, err, tc.variable)
		})
	}
}

func TestConfigRefusesAnUnknownRole(t *testing.T) {
	_, err := dmpfreference.FromEnv("worker", env("DMPF_PG_DSN", testDSN))
	if !errors.Is(err, dmpfreference.ErrUnknownRole) {
		t.Fatalf("FromEnv(worker) = %v, want ErrUnknownRole", err)
	}
}

func TestConfigNamesTheVariableItCannotParse(t *testing.T) {
	cases := []struct {
		variable string
		value    string
	}{
		{"DMPF_MIGRATE", "yes-please"},
		{"DMPF_KAFKA_INSECURE", "maybe"},
		{"DMPF_OTLP_INSECURE", "2"},
		{"DMPF_ITEM_LIMIT", "many"},
		{"DMPF_ITEM_LIMIT", "0"},
		{"DMPF_ITEM_LIMIT", "-1"},
	}
	for _, tc := range cases {
		t.Run(tc.variable+"="+tc.value, func(t *testing.T) {
			_, err := dmpfreference.FromEnv(dmpfreference.RoleAPI, env("DMPF_PG_DSN", testDSN, tc.variable, tc.value))
			if !errors.Is(err, dmpfreference.ErrInvalidVariable) {
				t.Fatalf("err = %v, want ErrInvalidVariable", err)
			}
			if !strings.Contains(err.Error(), tc.variable) {
				t.Fatalf("err = %q does not name %s", err, tc.variable)
			}
		})
	}
}

func TestValidateRefusesAConfigAssembledByHand(t *testing.T) {
	cfg := dmpfreference.Config{Role: dmpfreference.RoleRelay, DSN: testDSN, Brokers: []string{"localhost:9092"}, Topic: "orders.events"}

	requireMissing(t, cfg.Validate(), "DMPF_KAFKA_DLQ")
}
