//go:build integration

package authn_test

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	revocationTTL        = 300 * time.Second
	revocationSkew       = 60 * time.Second
	revocationBudget     = 60 * time.Second
	revocationPollPeriod = 2 * time.Second
	refusalProbe         = 250 * time.Millisecond
)

func TestKeycloakRealmKeepsTheEventsTheRevocationRereads(t *testing.T) {
	t.Parallel()
	var realm struct {
		AccessTokenLifespan int64    `json:"accessTokenLifespan"`
		EventsEnabled       bool     `json:"eventsEnabled"`
		EnabledEventTypes   []string `json:"enabledEventTypes"`
		EventsExpiration    int64    `json:"eventsExpiration"`
	}
	masterAdmin(t).decode("", &realm)

	if !realm.EventsEnabled || len(realm.EnabledEventTypes) != 1 || realm.EnabledEventTypes[0] != "LOGOUT" {
		t.Fatalf("eventsEnabled = %v enabledEventTypes = %v, want only LOGOUT kept", realm.EventsEnabled, realm.EnabledEventTypes)
	}
	if lifespan := time.Duration(realm.AccessTokenLifespan) * time.Second; lifespan > revocationTTL {
		t.Fatalf("accessTokenLifespan = %v, want at most OIDC_REVOCATION_TTL_SECONDS %v", lifespan, revocationTTL)
	}
	if expiration := time.Duration(realm.EventsExpiration) * time.Second; expiration < revocationTTL+revocationSkew {
		t.Fatalf("eventsExpiration = %v, want at least TTL + skew = %v: the polling rereads that window",
			expiration, revocationTTL+revocationSkew)
	}
}

type revocationEdge struct {
	store    *authn.MemoryStore
	feed     *authn.Feed
	verifier *authn.Verifier
}

func newRevocationEdge(t *testing.T) *revocationEdge {
	t.Helper()
	e := &revocationEdge{store: authn.NewMemoryStore(nil)}
	feed, err := authn.NewFeed(e.store, authn.FeedConfig{Issuer: keycloakIssuer(t), TTL: revocationTTL, Skew: revocationSkew})
	if err != nil {
		t.Fatalf("NewFeed() err = %v", err)
	}
	e.feed = feed
	cfg := authn.Defaults()
	cfg.Issuer = keycloakIssuer(t)
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "organization.*.tenant_id"
	cfg.Revocations = e.store
	if e.verifier, err = authn.NewVerifier(t.Context(), cfg); err != nil {
		t.Fatalf("NewVerifier() err = %v", err)
	}
	return e
}

func (e *revocationEdge) poll(t *testing.T) {
	t.Helper()
	polling, err := authn.NewSessionPolling(e.feed, authn.SessionPollingConfig{
		ClientID: "dmpf-session-events", ClientSecret: tb.Env(t, "KEYCLOAK_SESSION_EVENTS_SECRET"), Interval: revocationPollPeriod,
	})
	if err != nil {
		t.Fatalf("NewSessionPolling() err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var running sync.WaitGroup
	running.Go(func() { polling.Run(ctx) })
	t.Cleanup(func() {
		cancel()
		running.Wait()
	})
}

// receivingPortal registers a portal client whose back-channel URL reaches a
// BackchannelLogout served on the host, at KEYCLOAK_CALLBACK_HOST from inside
// the Keycloak container.
func (e *revocationEdge) receivingPortal(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("net.Listen err = %v", err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	client := disposablePortal(t, "http://"+net.JoinHostPort(tb.Env(t, "KEYCLOAK_CALLBACK_HOST"), port)+"/backchannel-logout")

	receiver, err := authn.NewBackchannelLogout(t.Context(), e.feed, authn.BackchannelLogoutConfig{ClientID: client})
	if err != nil {
		_ = listener.Close()
		t.Fatalf("NewBackchannelLogout() err = %v", err)
	}
	server := httptest.NewUnstartedServer(receiver)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return client
}

func (e *revocationEdge) accepts(t *testing.T, accessToken string) {
	t.Helper()
	if _, err := e.verifier.Authenticate(t.Context(), bearer(accessToken)); err != nil {
		t.Fatalf("Authenticate() before the revocation err = %v", err)
	}
}

func (e *revocationEdge) refusedWithin(t *testing.T, accessToken string, since time.Time) time.Duration {
	t.Helper()
	for {
		_, err := e.verifier.Authenticate(t.Context(), bearer(accessToken))
		elapsed := time.Since(since)
		switch {
		case errors.Is(err, ports.ErrCredentialRejected) && elapsed >= revocationBudget:
			t.Fatalf("the token was refused only %v after the session ended, want under %v", elapsed, revocationBudget)
		case errors.Is(err, ports.ErrCredentialRejected):
			return elapsed
		case err != nil:
			t.Fatalf("Authenticate() err = %v, want a rejection or an acceptance", err)
		case elapsed >= revocationBudget:
			t.Fatalf("the token was still accepted %v after the session ended, want a refusal within %v", elapsed, revocationBudget)
		}
		time.Sleep(refusalProbe)
	}
}

func (e *revocationEdge) requireEntry(t *testing.T, accessToken string, want authn.RevocationKind) {
	t.Helper()
	claims := tokenClaims(t, accessToken)
	ref := authn.SessionRef{Subject: ports.SubjectID(claimText(claims, "sub"))}
	if issued, ok := claims["iat"].(float64); ok {
		ref.IssuedAt = time.Unix(int64(issued), 0)
	}
	if want == authn.RevocationSession {
		ref.ID = claimText(claims, "sid")
	}
	kind, err := e.store.Revoked(t.Context(), ref)
	if err != nil || kind != want {
		t.Fatalf("deny-list entry = %q (%v), want %q", kind, err, want)
	}
}

func disposableUser(t *testing.T) (account, string) {
	t.Helper()
	user, id := undeletedUser(t)
	removeOnCleanup(t, "/users/"+id)
	return user, id
}

func undeletedUser(t *testing.T) (account, string) {
	t.Helper()
	user := account{username: "dmpf-it-revoked-" + strings.ToLower(rand.Text()), password: rand.Text()}
	id := masterAdmin(t).create("/users", map[string]any{
		"username":      user.username,
		"enabled":       true,
		"emailVerified": true,
		"email":         user.username + "@dmpf.test",
		"firstName":     "Revoked",
		"lastName":      "Disposable",
		"credentials":   []map[string]any{{"type": "password", "value": user.password, "temporary": false}},
	})
	return user, id
}

func TestKeycloakRevocationRefusesTheTokenWithinTheBudget(t *testing.T) {
	t.Parallel()

	t.Run("user logout through the back-channel", func(t *testing.T) {
		t.Parallel()
		edge := newRevocationEdge(t)
		b := browserFor(t, edge.receivingPortal(t))
		tokens, _ := b.signIn(seedAccount(t, "alice"), "organization:acme")
		edge.accepts(t, tokens.AccessToken)

		since := time.Now()
		b.logout(tokens.IDToken)

		t.Logf("refused %v after the logout", edge.refusedWithin(t, tokens.AccessToken, since))
		edge.requireEntry(t, tokens.AccessToken, authn.RevocationSession)
	})
	t.Run("user logout without a receiver, through the LOGOUT event", func(t *testing.T) {
		t.Parallel()
		edge := newRevocationEdge(t)
		edge.poll(t)
		b := browserFor(t, disposablePortal(t, ""))
		tokens, _ := b.signIn(seedAccount(t, "alice"), "organization:acme")
		edge.accepts(t, tokens.AccessToken)

		since := time.Now()
		b.logout(tokens.IDToken)

		t.Logf("refused %v after the logout", edge.refusedWithin(t, tokens.AccessToken, since))
		edge.requireEntry(t, tokens.AccessToken, authn.RevocationSession)
	})
	t.Run("administrative logout, through the admin event", func(t *testing.T) {
		t.Parallel()
		edge := newRevocationEdge(t)
		edge.poll(t)
		user, id := disposableUser(t)
		tokens, _ := browserFor(t, disposablePortal(t, "")).signIn(user, "")
		edge.accepts(t, tokens.AccessToken)

		since := time.Now()
		masterAdmin(t).call(http.MethodPost, "/users/"+id+"/logout", nil, http.StatusNoContent)

		t.Logf("refused %v after the administrative logout", edge.refusedWithin(t, tokens.AccessToken, since))
		edge.requireEntry(t, tokens.AccessToken, authn.RevocationSubject)
	})
	t.Run("administrative end of one session, through the admin event", func(t *testing.T) {
		t.Parallel()
		edge := newRevocationEdge(t)
		edge.poll(t)
		user, _ := disposableUser(t)
		tokens, _ := browserFor(t, disposablePortal(t, "")).signIn(user, "")
		edge.accepts(t, tokens.AccessToken)

		since := time.Now()
		masterAdmin(t).call(http.MethodDelete, "/sessions/"+claimText(tokenClaims(t, tokens.AccessToken), "sid"), nil,
			http.StatusNoContent)

		t.Logf("refused %v after ending the session", edge.refusedWithin(t, tokens.AccessToken, since))
		edge.requireEntry(t, tokens.AccessToken, authn.RevocationSession)
	})
	t.Run("user deleted, through the admin event", func(t *testing.T) {
		t.Parallel()
		edge := newRevocationEdge(t)
		edge.poll(t)
		user, id := undeletedUser(t)
		tokens, _ := browserFor(t, disposablePortal(t, "")).signIn(user, "")
		edge.accepts(t, tokens.AccessToken)

		since := time.Now()
		masterAdmin(t).call(http.MethodDelete, "/users/"+id, nil, http.StatusNoContent)

		t.Logf("refused %v after deleting the user", edge.refusedWithin(t, tokens.AccessToken, since))
		edge.requireEntry(t, tokens.AccessToken, authn.RevocationSubject)
	})
	t.Run("user disabled, through the admin event", func(t *testing.T) {
		t.Parallel()
		edge := newRevocationEdge(t)
		edge.poll(t)
		user, id := disposableUser(t)
		tokens, _ := browserFor(t, disposablePortal(t, "")).signIn(user, "")
		edge.accepts(t, tokens.AccessToken)

		since := time.Now()
		masterAdmin(t).call(http.MethodPut, "/users/"+id, map[string]any{"enabled": false}, http.StatusNoContent)

		t.Logf("refused %v after disabling the user", edge.refusedWithin(t, tokens.AccessToken, since))
		edge.requireEntry(t, tokens.AccessToken, authn.RevocationSubject)
	})
}
