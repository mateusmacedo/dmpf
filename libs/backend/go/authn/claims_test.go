package authn

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// keycloakClaims is the shape a Keycloak access token carries: scope as a
// space-separated string (RFC 6749 §3.3) and roles nested under realm_access.
func keycloakClaims(t *testing.T) map[string]any {
	t.Helper()
	const payload = `{
		"sub": "f:0e1a:alice",
		"aud": "dmpf-bff",
		"azp": "dmpf-bff",
		"scope": "openid profile orders:write",
		"realm_access": {"roles": ["offline_access", "orders:read"]},
		"resource_access": {"dmpf-bff": {"roles": ["orders:admin"]}},
		"tenant_id": "acme",
		"token_use": "access"
	}`
	var claims map[string]any
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		t.Fatalf("unmarshalling the fixture: %v", err)
	}
	return claims
}

func TestStringClaimResolvesFlatAndNestedPaths(t *testing.T) {
	claims := keycloakClaims(t)

	tests := []struct {
		name string
		path string
		want string
		ok   bool
	}{
		{name: "flat key", path: "tenant_id", want: "acme", ok: true},
		{name: "subject", path: "sub", want: "f:0e1a:alice", ok: true},
		{name: "absent key", path: "missing", ok: false},
		{name: "object is not a string", path: "realm_access", ok: false},
		{name: "nested object is not a string", path: "resource_access.dmpf-bff", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := stringClaim(claims, tt.path)
			if ok != tt.ok {
				t.Fatalf("stringClaim(%q) ok = %v, want %v", tt.path, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("stringClaim(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestStringClaimPrefersTheLiteralKeyOverPathTraversal(t *testing.T) {
	claims := map[string]any{
		"https://app.example.com/tenant_id": "acme",
		"https":                             map[string]any{"//app": "wrong"},
	}

	got, ok := stringClaim(claims, "https://app.example.com/tenant_id")
	if !ok || got != "acme" {
		t.Fatalf("stringClaim() = %q, %v: a namespaced claim carries dots in its own name and must match literally", got, ok)
	}
}

func TestStringClaimRefusesNonStringValues(t *testing.T) {
	claims := map[string]any{"n": float64(1), "b": true, "null": nil, "empty": ""}

	for _, path := range []string{"n", "b", "null", "empty"} {
		t.Run(path, func(t *testing.T) {
			if got, ok := stringClaim(claims, path); ok {
				t.Fatalf("stringClaim(%q) = %q, true: only a non-empty string resolves a claim", path, got)
			}
		})
	}
}

func TestPermissionsFromUnitesEveryDeclaredClaim(t *testing.T) {
	claims := keycloakClaims(t)

	got := permissionsFrom(claims, []string{"scope", "realm_access.roles"})
	want := []ports.Permission{"offline_access", "openid", "orders:read", "orders:write", "profile"}

	if !slices.Equal(got, want) {
		t.Fatalf("permissionsFrom() = %v, want %v", got, want)
	}
}

func TestPermissionsFromReadsNestedClientRoles(t *testing.T) {
	claims := keycloakClaims(t)

	got := permissionsFrom(claims, []string{"resource_access.dmpf-bff.roles"})
	if want := []ports.Permission{"orders:admin"}; !slices.Equal(got, want) {
		t.Fatalf("permissionsFrom() = %v, want %v", got, want)
	}
}

func TestPermissionsFromDeduplicatesAcrossClaims(t *testing.T) {
	claims := map[string]any{
		"scope": "orders:read orders:read",
		"roles": []any{"orders:read", "orders:write"},
	}

	got := permissionsFrom(claims, []string{"scope", "roles"})
	if want := []ports.Permission{"orders:read", "orders:write"}; !slices.Equal(got, want) {
		t.Fatalf("permissionsFrom() = %v, want %v", got, want)
	}
}

func TestPermissionsFromIsNeverNilAndSkipsUnusableClaims(t *testing.T) {
	claims := map[string]any{"scope": float64(3), "roles": []any{true, "orders:read", nil}}

	got := permissionsFrom(claims, []string{"scope", "roles", "absent"})
	if got == nil {
		t.Fatal("a resolved subject always carries a set, empty at worst: nil would read as absence")
	}
	if want := []ports.Permission{"orders:read"}; !slices.Equal(got, want) {
		t.Fatalf("permissionsFrom() = %v, want %v", got, want)
	}
}

func TestPermissionsFromWithoutDeclaredClaimsIsEmptyNotNil(t *testing.T) {
	got := permissionsFrom(keycloakClaims(t), nil)

	if got == nil || len(got) != 0 {
		t.Fatalf("permissionsFrom() = %v, want an empty non-nil set", got)
	}
}
