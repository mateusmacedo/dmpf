package authn_test

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func keycloakEnv() map[string]string {
	return map[string]string{
		"OIDC_ISSUER":       "https://keycloak.example.com/realms/dmpf",
		"OIDC_AUDIENCE":     "dmpf-bff",
		"OIDC_TENANT_CLAIM": "tenant_id",
	}
}

func TestFromEnvReadsTheKeycloakMinimum(t *testing.T) {
	cfg, err := authn.FromEnv(env(keycloakEnv()))
	if err != nil {
		t.Fatalf("FromEnv() err = %v", err)
	}

	if cfg.Issuer != "https://keycloak.example.com/realms/dmpf" {
		t.Fatalf("Issuer = %q", cfg.Issuer)
	}
	if cfg.Audience != "dmpf-bff" {
		t.Fatalf("Audience = %q", cfg.Audience)
	}
	if cfg.TenantClaim != "tenant_id" {
		t.Fatalf("TenantClaim = %q", cfg.TenantClaim)
	}
	if want := []string{"scope", "realm_access.roles"}; !slices.Equal(cfg.PermissionClaims, want) {
		t.Fatalf("PermissionClaims = %v, want %v: the default reads a Keycloak token", cfg.PermissionClaims, want)
	}
	if cfg.DiscoveryTimeout != 10*time.Second {
		t.Fatalf("DiscoveryTimeout = %v, want 10s", cfg.DiscoveryTimeout)
	}
	if cfg.DevMock {
		t.Fatal("the development mock must stay off unless declared")
	}
}

func TestFromEnvRefusesAStartWithNeitherVerifierNorDeclaredMock(t *testing.T) {
	_, err := authn.FromEnv(env(nil))

	if !errors.Is(err, authn.ErrVerifierNotDeclared) {
		t.Fatalf("FromEnv() err = %v, want %v", err, authn.ErrVerifierNotDeclared)
	}
}

func TestFromEnvRefusesAPartiallyConfiguredVerifier(t *testing.T) {
	tests := []struct {
		name  string
		unset string
	}{
		{name: "no audience", unset: "OIDC_AUDIENCE"},
		{name: "no tenant claim", unset: "OIDC_TENANT_CLAIM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs := keycloakEnv()
			delete(pairs, tt.unset)

			_, err := authn.FromEnv(env(pairs))
			if !errors.Is(err, authn.ErrMissingVariable) {
				t.Fatalf("FromEnv() err = %v, want %v", err, authn.ErrMissingVariable)
			}
			if !strings.Contains(err.Error(), tt.unset) {
				t.Fatalf("the error must name the missing variable, got %q", err.Error())
			}
		})
	}
}

func TestFromEnvAcceptsTheDeclaredDevelopmentMockAlone(t *testing.T) {
	cfg, err := authn.FromEnv(env(map[string]string{"AUTH_DEV_MOCK": "true"}))
	if err != nil {
		t.Fatalf("FromEnv() err = %v", err)
	}
	if !cfg.DevMock {
		t.Fatal("DevMock = false after the explicit opt-out")
	}
}

func TestFromEnvOverridesEveryClaimAndTheTimeout(t *testing.T) {
	pairs := keycloakEnv()
	pairs["OIDC_PERMISSION_CLAIMS"] = "scope, resource_access.dmpf-bff.roles"
	pairs["OIDC_DISCOVERY_TIMEOUT_SECONDS"] = "30"
	pairs["OIDC_TENANT_CLAIM"] = "https://app.example.com/tenant_id"

	cfg, err := authn.FromEnv(env(pairs))
	if err != nil {
		t.Fatalf("FromEnv() err = %v", err)
	}

	if want := []string{"scope", "resource_access.dmpf-bff.roles"}; !slices.Equal(cfg.PermissionClaims, want) {
		t.Fatalf("PermissionClaims = %v, want %v", cfg.PermissionClaims, want)
	}
	if cfg.DiscoveryTimeout != 30*time.Second {
		t.Fatalf("DiscoveryTimeout = %v, want 30s", cfg.DiscoveryTimeout)
	}
	if cfg.TenantClaim != "https://app.example.com/tenant_id" {
		t.Fatalf("TenantClaim = %q: a namespaced claim must survive configuration", cfg.TenantClaim)
	}
}

func TestFromEnvRefusesTheMockAlongsideAConfiguredVerifier(t *testing.T) {
	pairs := keycloakEnv()
	pairs["AUTH_DEV_MOCK"] = "true"

	if _, err := authn.FromEnv(env(pairs)); !errors.Is(err, authn.ErrMockWithVerifier) {
		t.Fatalf("FromEnv() err = %v, want %v: one start resolves identity one way", err, authn.ErrMockWithVerifier)
	}
}

func TestReadEnvReadsTheRevocationTimeout(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"unset answers the default", "", 250 * time.Millisecond},
		{"declared in milliseconds", "500", 500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs := keycloakEnv()
			pairs["OIDC_REVOCATION_TIMEOUT_MS"] = tt.value

			cfg, err := authn.ReadEnv(env(pairs))
			if err != nil {
				t.Fatalf("ReadEnv() err = %v", err)
			}
			if cfg.RevocationTimeout != tt.want {
				t.Fatalf("RevocationTimeout = %v, want %v", cfg.RevocationTimeout, tt.want)
			}
		})
	}
}

func TestReadEnvRefusesARevocationTimeoutThatIsNotPositive(t *testing.T) {
	for _, value := range []string{"0", "-1", "fast"} {
		t.Run(value, func(t *testing.T) {
			pairs := keycloakEnv()
			pairs["OIDC_REVOCATION_TIMEOUT_MS"] = value

			_, err := authn.ReadEnv(env(pairs))
			if !errors.Is(err, envconfig.ErrInvalidVariable) {
				t.Fatalf("ReadEnv() err = %v, want %v", err, envconfig.ErrInvalidVariable)
			}
			if !strings.Contains(err.Error(), "OIDC_REVOCATION_TIMEOUT_MS") {
				t.Fatalf("the error must name the variable, got %q", err.Error())
			}
		})
	}
}

func TestReadEnvBoundsTheTimeoutsByTheLargestDuration(t *testing.T) {
	tests := []struct {
		variable string
		value    string
		want     error
	}{
		{"OIDC_DISCOVERY_TIMEOUT_SECONDS", strconv.FormatInt(math.MaxInt64/int64(time.Second), 10), nil},
		{"OIDC_DISCOVERY_TIMEOUT_SECONDS", strconv.FormatInt(math.MaxInt64/int64(time.Second)+1, 10), envconfig.ErrInvalidVariable},
		{"OIDC_DISCOVERY_TIMEOUT_SECONDS", "0", envconfig.ErrInvalidVariable},
		{"OIDC_REVOCATION_TIMEOUT_MS", strconv.FormatInt(math.MaxInt64/int64(time.Millisecond), 10), nil},
		{"OIDC_REVOCATION_TIMEOUT_MS", strconv.FormatInt(math.MaxInt64/int64(time.Millisecond)+1, 10), envconfig.ErrInvalidVariable},
	}
	for _, tt := range tests {
		t.Run(tt.variable+"="+tt.value, func(t *testing.T) {
			pairs := keycloakEnv()
			pairs[tt.variable] = tt.value

			cfg, err := authn.ReadEnv(env(pairs))
			if !errors.Is(err, tt.want) {
				t.Fatalf("ReadEnv() err = %v, want %v", err, tt.want)
			}
			if err != nil && !strings.Contains(err.Error(), tt.variable) {
				t.Fatalf("the error must name the variable, got %q", err.Error())
			}
			if err == nil && (cfg.DiscoveryTimeout <= 0 || cfg.RevocationTimeout <= 0) {
				t.Fatalf("timeouts = %v %v, want positive durations", cfg.DiscoveryTimeout, cfg.RevocationTimeout)
			}
		})
	}
}

func TestValidateRefusesACheckerWithoutAPositiveTimeout(t *testing.T) {
	tests := []struct {
		name    string
		checker authn.RevocationChecker
		timeout time.Duration
		wantErr bool
	}{
		{"checker with the default timeout", authn.NewMemoryStore(nil), 250 * time.Millisecond, false},
		{"checker with zero timeout", authn.NewMemoryStore(nil), 0, true},
		{"checker with negative timeout", authn.NewMemoryStore(nil), -time.Millisecond, true},
		{"no checker ignores the timeout", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := authn.ReadEnv(env(keycloakEnv()))
			if err != nil {
				t.Fatal(err)
			}
			cfg.Revocations = tt.checker
			cfg.RevocationTimeout = tt.timeout

			err = cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, envconfig.ErrInvalidVariable) {
				t.Fatalf("Validate() err = %v, want %v", err, envconfig.ErrInvalidVariable)
			}
		})
	}
}

func TestTheConfigLogsWhetherRevocationIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		checker authn.RevocationChecker
		want    string
	}{
		{"without a checker", nil, "disabled"},
		{"with a checker", authn.NewMemoryStore(nil), "enabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := authn.Defaults()
			cfg.Revocations = tt.checker

			var got string
			for _, attr := range cfg.LogValue().Group() {
				if attr.Key == "revocation" {
					got = attr.Value.String()
				}
			}
			if got != tt.want {
				t.Fatalf("revocation = %q, want %q", got, tt.want)
			}
		})
	}
}
