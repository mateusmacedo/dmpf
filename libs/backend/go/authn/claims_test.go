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
		"organization": {"acme": {"tenant_id": ["t-acme"], "id": "0b6d1f5e-7c9a-4e55-9d3a-6f1c2a8b4e10"}},
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
		{name: "list of one string resolves", path: "resource_access.dmpf-bff.roles", want: "orders:admin", ok: true},
		{name: "list of two strings is ambiguous", path: "realm_access.roles", ok: false},
		{name: "wildcard picks the only organization", path: "organization.*.id", want: "0b6d1f5e-7c9a-4e55-9d3a-6f1c2a8b4e10", ok: true},
		{name: "wildcard then list of one string", path: "organization.*.tenant_id", want: "t-acme", ok: true},
		{name: "wildcard leaf resolves the only value", path: "resource_access.*.roles", want: "orders:admin", ok: true},
		{name: "wildcard leaf over an object is not a string", path: "organization.*", ok: false},
		{name: "wildcard over a missing parent", path: "missing.*.tenant_id", ok: false},
		{name: "wildcard over a missing leaf", path: "organization.*.missing", ok: false},
		{name: "wildcard over a non-object parent", path: "tenant_id.*.x", ok: false},
		{name: "bare wildcard never picks a top-level claim", path: "*", ok: false},
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

func TestStringClaimAcceptsAListOnlyWithOneNonEmptyString(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
		ok    bool
	}{
		{name: "one string", value: []any{"t-acme"}, want: "t-acme", ok: true},
		{name: "two strings", value: []any{"t-acme", "t-globex"}, ok: false},
		{name: "empty list", value: []any{}, ok: false},
		{name: "one empty string", value: []any{""}, ok: false},
		{name: "one number", value: []any{float64(1)}, ok: false},
		{name: "one null", value: []any{nil}, ok: false},
		{name: "one nested list", value: []any{[]any{"t-acme"}}, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := stringClaim(map[string]any{"tenant": tt.value}, "tenant")
			if ok != tt.ok {
				t.Fatalf("stringClaim(%v) ok = %v, want %v", tt.value, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("stringClaim(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestStringClaimWildcardNeedsExactlyOneKey(t *testing.T) {
	const path = "organization.*.tenant_id"
	acme := map[string]any{"tenant_id": []any{"t-acme"}}
	globex := map[string]any{"tenant_id": []any{"t-globex"}}

	tests := []struct {
		name   string
		claims map[string]any
		want   string
		ok     bool
	}{
		{name: "one organization", claims: map[string]any{"organization": map[string]any{"acme": acme}}, want: "t-acme", ok: true},
		{name: "no organization claim", claims: map[string]any{"sub": "alice"}, ok: false},
		{name: "empty organization object", claims: map[string]any{"organization": map[string]any{}}, ok: false},
		{name: "two organizations", claims: map[string]any{"organization": map[string]any{"acme": acme, "globex": globex}}, ok: false},
		{name: "one organization without the attribute", claims: map[string]any{"organization": map[string]any{"acme": map[string]any{"id": "x"}}}, ok: false},
		{name: "organization is a list", claims: map[string]any{"organization": []any{"acme"}}, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := stringClaim(tt.claims, path)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("stringClaim(%q) = %q, %v, want %q, %v: the wildcard resolves only when exactly one key sits at that level, so a tenant is never picked among several (IDN-20)", path, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestClaimValuePrefersTheLiteralKeyOverTheWildcard(t *testing.T) {
	claims := map[string]any{
		"organization.*.tenant_id": "literal",
		"organization":             map[string]any{"acme": map[string]any{"tenant_id": "wildcard"}},
	}

	got, ok := stringClaim(claims, "organization.*.tenant_id")
	if !ok || got != "literal" {
		t.Fatalf("stringClaim() = %q, %v: the whole path is tried as a literal key before any traversal", got, ok)
	}
}

func TestClaimValueKeepsPlainPathsWorkingNextToTheWildcard(t *testing.T) {
	claims := keycloakClaims(t)

	got, ok := claimValue(claims, "realm_access.roles")
	roles, isList := got.([]any)
	if !ok || !isList || len(roles) != 2 {
		t.Fatalf("claimValue(realm_access.roles) = %v, %v: a path without the wildcard must still walk the object and keep the full list", got, ok)
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

func TestPermissionsFromWildcardReadsTheOnlyOrganization(t *testing.T) {
	claims := map[string]any{
		"organization": map[string]any{"acme": map[string]any{"roles": []any{"orders:read", "orders:write"}}},
	}

	got := permissionsFrom(claims, []string{"organization.*.roles"})
	if want := []ports.Permission{"orders:read", "orders:write"}; !slices.Equal(got, want) {
		t.Fatalf("permissionsFrom() = %v, want %v", got, want)
	}
}

func TestPermissionsFromWildcardNeverUnitesDifferentOrganizations(t *testing.T) {
	claims := map[string]any{
		"organization": map[string]any{
			"acme":   map[string]any{"roles": []any{"orders:read"}},
			"globex": map[string]any{"roles": []any{"orders:admin"}},
		},
		"scope": "openid",
	}

	got := permissionsFrom(claims, []string{"organization.*.roles", "scope"})
	if want := []ports.Permission{"openid"}; !slices.Equal(got, want) {
		t.Fatalf("permissionsFrom() = %v, want %v: a wildcard that matches two organizations must grant nothing from either, never the union", got, want)
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
