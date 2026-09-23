package authn_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const testKeyID = "k-000001"

// idp is a stand-in authorization server: it serves the discovery document and
// the JWKS the verifier fetches, and signs tokens with the key it published.
type idp struct {
	issuer string
	key    *rsa.PrivateKey
}

func newIdP(t *testing.T) *idp {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating the signing key: %v", err)
	}
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)

	provider := &idp{issuer: server.URL, key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                provider.issuer,
			"jwks_uri":                              provider.issuer + "/jwks",
			"authorization_endpoint":                provider.issuer + "/auth",
			"token_endpoint":                        provider.issuer + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"keys": []any{map[string]any{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": testKeyID,
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	server.Config.Handler = mux

	return provider
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// sign mints an RS256 JWT: base64url(header).base64url(payload).base64url(sig).
func (p *idp) sign(t *testing.T, claims map[string]any) string {
	t.Helper()

	header, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": testKeyID})
	if err != nil {
		t.Fatalf("marshalling the header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshalling the claims: %v", err)
	}

	signing := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(signature)
}

// keycloakToken is the shape Keycloak issues: aud carries the audience, scope
// is space-separated and realm roles are nested.
func (p *idp) keycloakToken(t *testing.T, overrides map[string]any) string {
	t.Helper()

	claims := map[string]any{
		"iss":          p.issuer,
		"aud":          "dmpf-bff",
		"azp":          "dmpf-bff",
		"sub":          "f:0e1a:alice",
		"exp":          time.Now().Add(time.Hour).Unix(),
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"scope":        "openid orders:write",
		"realm_access": map[string]any{"roles": []any{"orders:read"}},
		"tenant_id":    "acme",
	}
	for key, value := range overrides {
		if value == nil {
			delete(claims, key)
			continue
		}
		claims[key] = value
	}
	return p.sign(t, claims)
}

func verifierFor(t *testing.T, provider *idp) *authn.Verifier {
	t.Helper()

	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"

	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewVerifier() err = %v", err)
	}
	return verifier
}

func bearer(token string) ports.Credential {
	return ports.Credential{Scheme: "Bearer", Value: token}
}

func TestVerifierResolvesIdentityFromAKeycloakToken(t *testing.T) {
	provider := newIdP(t)
	verifier := verifierFor(t, provider)

	got, err := verifier.Authenticate(t.Context(), bearer(provider.keycloakToken(t, nil)))
	if err != nil {
		t.Fatalf("Authenticate() err = %v", err)
	}

	if got.Subject != "f:0e1a:alice" {
		t.Fatalf("Subject = %q", got.Subject)
	}
	if got.Tenant == nil || *got.Tenant != "acme" {
		t.Fatalf("Tenant = %v, want %q", got.Tenant, "acme")
	}
	want := []ports.Permission{"openid", "orders:read", "orders:write"}
	if !slices.Equal(got.Permissions, want) {
		t.Fatalf("Permissions = %v, want %v: scope and realm_access.roles unite", got.Permissions, want)
	}
}

func TestVerifierRejectsWhatFailsVerification(t *testing.T) {
	provider := newIdP(t)
	other := newIdP(t)
	verifier := verifierFor(t, provider)

	tests := []struct {
		name  string
		token func() string
	}{
		{name: "expired", token: func() string {
			return provider.keycloakToken(t, map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})
		}},
		{name: "wrong audience", token: func() string {
			return provider.keycloakToken(t, map[string]any{"aud": "another-api"})
		}},
		{name: "wrong issuer", token: func() string {
			return provider.keycloakToken(t, map[string]any{"iss": other.issuer})
		}},
		{name: "signed by another key", token: func() string {
			return other.keycloakToken(t, map[string]any{"iss": provider.issuer})
		}},
		{name: "not a token at all", token: func() string { return "not-a-jwt" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verifier.Authenticate(t.Context(), bearer(tt.token()))

			if !errors.Is(err, ports.ErrCredentialRejected) {
				t.Fatalf("Authenticate() err = %v, want %v", err, ports.ErrCredentialRejected)
			}
		})
	}
}

func TestVerifierSeparatesTheThreeConditionsOfIDN01(t *testing.T) {
	provider := newIdP(t)
	verifier := verifierFor(t, provider)

	if _, err := verifier.Authenticate(t.Context(), ports.Credential{}); !errors.Is(err, ports.ErrCredentialAbsent) {
		t.Fatalf("nothing presented: err = %v, want %v", err, ports.ErrCredentialAbsent)
	}

	if _, err := verifier.Authenticate(t.Context(), ports.Credential{Scheme: "Basic", Value: "x"}); !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("unsupported scheme: err = %v, want %v", err, ports.ErrCredentialRejected)
	}

	subjectless := provider.keycloakToken(t, map[string]any{"sub": nil})
	if _, err := verifier.Authenticate(t.Context(), bearer(subjectless)); !errors.Is(err, ports.ErrSubjectUnresolved) {
		t.Fatalf("verified without a subject: err = %v, want %v", err, ports.ErrSubjectUnresolved)
	}
}

func TestVerifierLeavesAnAbsentTenantAbsent(t *testing.T) {
	provider := newIdP(t)
	verifier := verifierFor(t, provider)

	got, err := verifier.Authenticate(t.Context(), bearer(provider.keycloakToken(t, map[string]any{"tenant_id": nil})))
	if err != nil {
		t.Fatalf("Authenticate() err = %v", err)
	}

	if got.Tenant != nil {
		t.Fatalf("Tenant = %q, want nil: no block invents a tenant to satisfy a requirement (IDN-20)", *got.Tenant)
	}
	if got.Permissions == nil {
		t.Fatal("a resolved subject always carries a set, empty at worst")
	}
}

func TestVerifierReadsTheClaimsTheOperatorDeclared(t *testing.T) {
	provider := newIdP(t)

	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "https://app.example.com/tenant_id"
	cfg.PermissionClaims = []string{"resource_access.dmpf-bff.roles"}

	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewVerifier() err = %v", err)
	}

	token := provider.keycloakToken(t, map[string]any{
		"https://app.example.com/tenant_id": "globex",
		"resource_access":                   map[string]any{"dmpf-bff": map[string]any{"roles": []any{"orders:admin"}}},
	})

	got, err := verifier.Authenticate(t.Context(), bearer(token))
	if err != nil {
		t.Fatalf("Authenticate() err = %v", err)
	}

	if got.Tenant == nil || *got.Tenant != "globex" {
		t.Fatalf("Tenant = %v, want %q: a namespaced claim resolves by its literal name", got.Tenant, "globex")
	}
	if want := []ports.Permission{"orders:admin"}; !slices.Equal(got.Permissions, want) {
		t.Fatalf("Permissions = %v, want %v", got.Permissions, want)
	}
}

func TestNewVerifierFailsWhenDiscoveryDoesNotAnswer(t *testing.T) {
	unreachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(unreachable.Close)

	cfg := authn.Defaults()
	cfg.Issuer = unreachable.URL
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"

	if _, err := authn.NewVerifier(t.Context(), cfg); err == nil {
		t.Fatal("NewVerifier() err = nil: a start that cannot reach the authority must fail, not serve unauthenticated")
	}
}
