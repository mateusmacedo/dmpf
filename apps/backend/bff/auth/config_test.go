package auth_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/auth"
)

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func keycloakEnv() map[string]string {
	return map[string]string{
		"DMPF_OIDC_ISSUER":       "https://keycloak.example.com/realms/dmpf",
		"DMPF_OIDC_AUDIENCE":     "dmpf-bff",
		"DMPF_OIDC_TENANT_CLAIM": "tenant_id",
	}
}

func TestFromEnvReadsTheKeycloakMinimum(t *testing.T) {
	cfg, err := auth.FromEnv(env(keycloakEnv()))
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
	_, err := auth.FromEnv(env(nil))

	if !errors.Is(err, auth.ErrVerifierNotDeclared) {
		t.Fatalf("FromEnv() err = %v, want %v", err, auth.ErrVerifierNotDeclared)
	}
}

func TestFromEnvRefusesAPartiallyConfiguredVerifier(t *testing.T) {
	tests := []struct {
		name  string
		unset string
	}{
		{name: "no audience", unset: "DMPF_OIDC_AUDIENCE"},
		{name: "no tenant claim", unset: "DMPF_OIDC_TENANT_CLAIM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs := keycloakEnv()
			delete(pairs, tt.unset)

			_, err := auth.FromEnv(env(pairs))
			if !errors.Is(err, auth.ErrMissingVariable) {
				t.Fatalf("FromEnv() err = %v, want %v", err, auth.ErrMissingVariable)
			}
			if !strings.Contains(err.Error(), tt.unset) {
				t.Fatalf("the error must name the missing variable, got %q", err.Error())
			}
		})
	}
}

func TestFromEnvAcceptsTheDeclaredDevelopmentMockAlone(t *testing.T) {
	cfg, err := auth.FromEnv(env(map[string]string{"DMPF_AUTH_DEV_MOCK": "true"}))
	if err != nil {
		t.Fatalf("FromEnv() err = %v", err)
	}
	if !cfg.DevMock {
		t.Fatal("DevMock = false after the explicit opt-out")
	}
}

func TestFromEnvOverridesEveryClaimAndTheTimeout(t *testing.T) {
	pairs := keycloakEnv()
	pairs["DMPF_OIDC_PERMISSION_CLAIMS"] = "scope, resource_access.dmpf-bff.roles"
	pairs["DMPF_OIDC_DISCOVERY_TIMEOUT_SECONDS"] = "30"
	pairs["DMPF_OIDC_TENANT_CLAIM"] = "https://app.example.com/tenant_id"

	cfg, err := auth.FromEnv(env(pairs))
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
	pairs["DMPF_AUTH_DEV_MOCK"] = "true"

	if _, err := auth.FromEnv(env(pairs)); !errors.Is(err, auth.ErrMockWithVerifier) {
		t.Fatalf("FromEnv() err = %v, want %v: one start resolves identity one way", err, auth.ErrMockWithVerifier)
	}
}
