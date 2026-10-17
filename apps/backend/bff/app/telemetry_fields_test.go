package app_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

var sentinels = []string{"sentinel-client-key", "sentinel-tenant", "sentinel-redis-password"}

func configWithSecrets() app.Config {
	cfg := app.Defaults()
	cfg.OrdersTarget = "orders:9090"
	cfg.ReservationsTarget = "reservations:9090"
	cfg.BookingsTarget = "bookings:9090"
	cfg.CAFile = "/tls/ca.crt"
	cfg.ServerName = "contexts.internal"
	cfg.ClientCertFile = "/tls/client.crt"
	cfg.ClientKeyFile = "/tls/sentinel-client-key.pem"
	cfg.CORSOrigins = []string{"https://app.example"}
	cfg.MetricTenants = []string{"sentinel-tenant-a", "sentinel-tenant-b"}
	cfg.OrdersContractPath = "/contracts/orders.yaml"
	cfg.ReservationsContractPath = "/contracts/reservations.yaml"
	cfg.BookingsContractPath = "/contracts/bookings.yaml"
	cfg.DrainDelay = 5 * time.Second
	cfg.Auth = authn.Config{Issuer: "https://issuer.example", Audience: "dmpf", TenantClaim: "tenant",
		PermissionClaims: []string{"permissions"}, DiscoveryTimeout: 5 * time.Second}
	var err error
	if cfg.Redis, err = redis.FromEnv(lookup("REDIS_URL", "redis://revocation:sentinel-redis-password@redis:6379/0")); err != nil {
		panic(err)
	}
	return cfg
}

func TestTheProcessConfiguredRecordCarriesTheAllowlistOfTheEdge(t *testing.T) {
	record, _ := edgeTelemetry.ProcessConfigured(t, configWithSecrets())

	got := obstest.SettingKeys(record)
	want := []string{"admin_addr", "authn.audience", "authn.dev_mock", "authn.discovery_timeout", "authn.issuer", "authn.permission_claims",
		"authn.revocation", "authn.tenant_claim", "bookings_grpc_target", "cors_origins", "drain_delay", "grpc_ca_file", "grpc_client_cert_file",
		"grpc_client_key_file", "grpc_insecure", "grpc_server_name", "http_addr", "metric_tenants", "openapi_bookings_path",
		"openapi_orders_path", "openapi_reservations_path", "orders_grpc_target", "redis.addr", "redis.command_timeout",
		"redis.dial_timeout", "redis.tls", "redis.username", "reservations_grpc_target"}
	if !slices.Equal(got, want) {
		t.Fatalf("dmpf.config.* = %v, want exactly the settings of the edge %v", got, want)
	}
}

func TestTheProcessConfiguredRecordDeclaresTheDenyListTheEdgeServesWith(t *testing.T) {
	cfg, closeRevocations, err := app.WithRevocations(configWithSecrets())
	if err != nil {
		t.Fatalf("WithRevocations() = %v", err)
	}
	t.Cleanup(func() { _ = closeRevocations() })

	record, _ := edgeTelemetry.ProcessConfigured(t, cfg)

	if got := record["dmpf.config.authn.revocation"].GetStringValue(); got != "enabled" {
		t.Fatalf("dmpf.config.authn.revocation = %q, want enabled: the edge never serves without the deny-list", got)
	}
}

func TestTheProcessConfiguredRecordCarriesNoSecret(t *testing.T) {
	record, out := edgeTelemetry.ProcessConfigured(t, configWithSecrets())

	for _, sentinel := range sentinels {
		if strings.Contains(out, sentinel) {
			t.Errorf("the telemetry carries %q (LOG-09): %s", sentinel, out)
		}
	}
	if got := record["dmpf.config.metric_tenants"]; got.GetIntValue() != 2 {
		t.Errorf("dmpf.config.metric_tenants = %v, want the count 2 and not the names", got)
	}
	if got := record["dmpf.config.grpc_client_key_file"].GetStringValue(); got != "set" {
		t.Errorf("dmpf.config.grpc_client_key_file = %v, want set", got)
	}
}

func TestTheTelemetryDeclaresTheApiRole(t *testing.T) {
	edgeTelemetry.RunRolesDeclared(t)
}

func TestARecordNamesTheExecutionOfTheCallFromTheBaggage(t *testing.T) {
	edgeTelemetry.RunTenantBaggage(t)
}
