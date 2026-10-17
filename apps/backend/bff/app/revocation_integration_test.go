//go:build integration

package app_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const signingKeyID = "bff-test"

type signingIssuer struct {
	url string
	key *rsa.PrivateKey
}

func newSigningIssuer(t *testing.T) signingIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() = %v", err)
	}
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)
	encode := base64.RawURLEncoding.EncodeToString
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": server.URL, "jwks_uri": server.URL + "/jwks",
			"authorization_endpoint": server.URL + "/auth", "token_endpoint": server.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": signingKeyID, "use": "sig", "alg": "RS256",
			"n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	server.Config.Handler = mux
	return signingIssuer{url: server.URL, key: key}
}

func (i signingIssuer) token(t *testing.T, subject, session string) string {
	t.Helper()
	now := time.Now()
	segment := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal() = %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	signed := segment(map[string]string{"alg": "RS256", "kid": signingKeyID, "typ": "JWT"}) + "." + segment(map[string]any{
		"iss": i.url, "aud": "dmpf-bff", "sub": subject, "sid": session, "tenant_id": "acme",
		"iat": now.Add(-10 * time.Second).Unix(), "exp": now.Add(5 * time.Minute).Unix(),
	})
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() = %v", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func TestARevokedSessionIsRefusedByTheEdge(t *testing.T) {
	redisURL := tb.Env(t, "REDIS_URL")
	issuer := newSigningIssuer(t)
	e := startEdge(t, "OIDC_ISSUER", issuer.url, "OIDC_AUDIENCE", "dmpf-bff", "OIDC_TENANT_CLAIM", "tenant_id", "REDIS_URL", redisURL)
	cfg, err := redis.FromEnv(func(name string) string { return map[string]string{"REDIS_URL": redisURL}[name] })
	if err != nil {
		t.Fatalf("redis.FromEnv() = %v", err)
	}
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Fatalf("redis.NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	store := redis.NewRevocationStore(client, clock.System())
	subject, session := fmt.Sprintf("user-%d", time.Now().UnixNano()), fmt.Sprintf("sid-%d", time.Now().UnixNano())
	bearer := "Bearer " + issuer.token(t, subject, session)

	if status := e.request(t, http.MethodGet, "/orders/o-1", "Authorization", bearer); status == http.StatusUnauthorized {
		t.Fatalf("GET /orders/o-1 before the logout = 401, want the token accepted")
	}
	revoked := authn.Revocation{EventID: "ev-" + session, Kind: authn.RevocationSession, Subject: ports.SubjectID(subject),
		SessionID: session, OccurredAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)}
	if _, err := store.Revoke(context.Background(), revoked); err != nil {
		t.Fatalf("Revoke() = %v", err)
	}

	if status := e.request(t, http.MethodGet, "/orders/o-1", "Authorization", bearer); status != http.StatusUnauthorized {
		t.Fatalf("GET /orders/o-1 after the logout = %d, want 401", status)
	}
	e.stop(t)

	refusal := recordNamed(t, e.records(t), "auth: token rejected")
	if got := attributesOf(refusal)["dmpf.auth.refusal_reason"].AsString(); got != "session_revoked" {
		t.Fatalf("dmpf.auth.refusal_reason = %q, want session_revoked", got)
	}
}
