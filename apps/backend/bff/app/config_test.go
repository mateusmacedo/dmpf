package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

var targets = []string{"ORDERS_GRPC_TARGET", "orders:9090", "RESERVATIONS_GRPC_TARGET", "reservations:9090", "BOOKINGS_GRPC_TARGET", "bookings:9090"}

// devMock declares how these starts resolve identity. Since the edge demands a
// subject on every route, a start that declares no verifier and no mock is
// refused, so the cases that expect a usable config have to say which one.
const devMock = "AUTH_DEV_MOCK"

func TestFromEnvRefusesAStartThatDeclaresNoIdentity(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true")...))

	if err == nil {
		t.Fatal("FromEnv() = nil: a start that could authenticate nobody must be refused")
	}
	if !strings.Contains(err.Error(), devMock) {
		t.Fatalf("FromEnv() = %v, want the error to name %s", err, devMock)
	}
}

func TestFromEnvAppliesTheDefaults(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.Service != "bff" || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :8080, bff and the opt-out", cfg)
	}
	if cfg.RouteBudget.Validate() != nil || cfg.RouteBudget.Limit <= 0 {
		t.Fatalf("RouteBudget = %+v, want a valid budget", cfg.RouteBudget)
	}
}

func TestFromEnvNamesEachMissingTarget(t *testing.T) {
	for _, variable := range []string{"ORDERS_GRPC_TARGET", "RESERVATIONS_GRPC_TARGET", "BOOKINGS_GRPC_TARGET"} {
		t.Run(variable, func(t *testing.T) {
			env := []string{"GRPC_INSECURE", "true"}
			for i := 0; i < len(targets); i += 2 {
				if targets[i] != variable {
					env = append(env, targets[i], targets[i+1])
				}
			}
			_, err := app.FromEnv(lookup(env...))
			if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestFromEnvRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(lookup(targets...))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_INSECURE") || !strings.Contains(err.Error(), "GRPC_CA_FILE") {
		t.Fatalf("FromEnv() = %v, want both transport variables named (GRP-15)", err)
	}
}

func TestFromEnvAcceptsATrustAuthority(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_CA_FILE", "/etc/ca.pem", "GRPC_SERVER_NAME", "orders.internal",
		"GRPC_CLIENT_CERT_FILE", "/etc/bff.crt", "GRPC_CLIENT_KEY_FILE", "/etc/bff.key", "CORS_ORIGINS", "http://a, http://b", devMock, "true")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.GRPCInsecure || cfg.CAFile != "/etc/ca.pem" || cfg.ServerName != "orders.internal" || len(cfg.CORSOrigins) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

// IDN-03: the contexts only trust a verified workload, so an edge that trusts
// their authority also has to present its own certificate.
func TestFromEnvRefusesATrustAuthorityWithoutAClientPair(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_CA_FILE", "/etc/ca.pem", devMock, "true")...))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_CLIENT_CERT_FILE") {
		t.Fatalf("FromEnv() = %v, want the client pair named", err)
	}
}

func TestFromEnvRefusesAnInvalidBoolean(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "maybe")...))

	if !errors.Is(err, app.ErrInvalidVariable) {
		t.Fatalf("FromEnv() = %v, want ErrInvalidVariable", err)
	}
}

func TestFromEnvReadsTheDrainDelay(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")

	cfg, err := app.FromEnv(lookup(base...))
	if err != nil || cfg.DrainDelay != 0 {
		t.Fatalf("FromEnv() = %v, %v; want no drain by default", cfg.DrainDelay, err)
	}
	cfg, err = app.FromEnv(lookup(append(base, "DRAIN_DELAY", "5s")...))
	if err != nil || cfg.DrainDelay != 5*time.Second {
		t.Fatalf("FromEnv() = %v, %v; want 5s", cfg.DrainDelay, err)
	}
	for _, invalid := range []string{"soon", "-1s"} {
		if _, err := app.FromEnv(lookup(append(base, "DRAIN_DELAY", invalid)...)); err == nil || !strings.Contains(err.Error(), "DRAIN_DELAY") {
			t.Fatalf("FromEnv(DRAIN_DELAY=%s) = %v, want the variable named", invalid, err)
		}
	}
}

func TestFromEnvReadsTheAdministrationAddress(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")

	cfg, err := app.FromEnv(lookup(base...))
	if err != nil || cfg.AdminAddr != ":8090" || cfg.AdminAddr == cfg.HTTPAddr {
		t.Fatalf("FromEnv() = %q, %v; want :8090, apart from the public %q", cfg.AdminAddr, err, cfg.HTTPAddr)
	}
	cfg, err = app.FromEnv(lookup(append(base, "ADMIN_ADDR", "127.0.0.1:9100")...))
	if err != nil || cfg.AdminAddr != "127.0.0.1:9100" {
		t.Fatalf("FromEnv() = %q, %v; want 127.0.0.1:9100", cfg.AdminAddr, err)
	}
	if _, err := app.FromEnv(lookup(append(base, "ADMIN_ADDR", "8090")...)); !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "ADMIN_ADDR") {
		t.Fatalf("FromEnv(ADMIN_ADDR=8090) = %v, want ErrInvalidVariable naming ADMIN_ADDR", err)
	}
}

func TestFromEnvReadsNoLegacyTelemetryVariable(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true",
		"SERVICE", "legacy", "SERVICE_VERSION", "9.9.9", "INSTANCE_ID", "legacy-1", "OTLP_ENDPOINT", "legacy:4317", "OTLP_INSECURE", "not-a-bool")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want the legacy telemetry variables ignored (RF-E1)", err)
	}
	telemetry := app.TelemetryOf(cfg)
	if telemetry.Service != "bff" || telemetry.Version == "9.9.9" || telemetry.Instance == "legacy-1" {
		t.Fatalf("TelemetryOf() = %+v, want the identity left to OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES (RF-E1, RF-E3)", telemetry)
	}
}

func TestTheIdentityOfTheProcessIsLeftToTheEnvironment(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true")...))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if telemetry := app.TelemetryOf(cfg); telemetry.Version != "" || telemetry.Instance != "" {
		t.Fatalf("TelemetryOf() declares version %q and instance %q, want neither: both come from OTEL_RESOURCE_ATTRIBUTES (RF-E3)", telemetry.Version, telemetry.Instance)
	}
}
