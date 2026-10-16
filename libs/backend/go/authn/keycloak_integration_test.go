//go:build integration

package authn_test

import (
	"crypto/rand"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"
	eventDeadline          = 15 * time.Second
)

func tenantVerifier(t *testing.T) *authn.Verifier {
	t.Helper()
	cfg := authn.Defaults()
	cfg.Issuer = keycloakIssuer(t)
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "organization.*.tenant_id"
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewVerifier(%s) err = %v: the edge must discover the reference realm", cfg.Issuer, err)
	}
	return verifier
}

func authenticate(t *testing.T, verifier *authn.Verifier, accessToken string) ports.Identity {
	t.Helper()
	identity, err := verifier.Authenticate(t.Context(), bearer(accessToken))
	if err != nil {
		t.Fatalf("Authenticate() err = %v: a token of the reference realm must pass the edge", err)
	}
	return identity
}

func TestKeycloakDiscoveryIssuerIsTheRealmURL(t *testing.T) {
	t.Parallel()
	issuer := keycloakIssuer(t)

	provider, err := oidc.NewProvider(t.Context(), issuer)
	if err != nil {
		t.Fatalf("discovery of %s err = %v: OIDC_ISSUER is KEYCLOAK_URL/realms/dmpf", issuer, err)
	}
	var discovery struct {
		Issuer string `json:"issuer"`
	}
	if err := provider.Claims(&discovery); err != nil || discovery.Issuer != issuer {
		t.Fatalf("discovery issuer = %q (%v), want %q", discovery.Issuer, err, issuer)
	}
	tenantVerifier(t)
}

func TestKeycloakAliceResolvesTheAcmeTenant(t *testing.T) {
	t.Parallel()
	verifier := tenantVerifier(t)

	tokens, _ := newBrowser(t).signIn(seedAccount(t, "alice"), "organization:acme")
	identity := authenticate(t, verifier, tokens.AccessToken)

	if want := claimText(tokenClaims(t, tokens.AccessToken), "sub"); want == "" || string(identity.Subject) != want {
		t.Fatalf("Identity.Subject = %q, want the token sub %q", identity.Subject, want)
	}
	requireTenant(t, identity, "t-acme")
}

func TestKeycloakBobSwitchesTenantWithinOneSSOSession(t *testing.T) {
	t.Parallel()
	verifier := tenantVerifier(t)
	b := newBrowser(t)
	bob := seedAccount(t, "bob")

	acme, _ := b.signIn(bob, "organization:acme")
	globex, answered := b.signIn(bob, "organization:globex")

	if len(answered) != 0 {
		t.Fatalf("second authorization answered %v, want none: the SSO session carries bob to the other organization", answered)
	}
	requireTenant(t, authenticate(t, verifier, acme.AccessToken), "t-acme")
	requireTenant(t, authenticate(t, verifier, globex.AccessToken), "t-globex")
}

func TestKeycloakWithoutExactlyOneOrganizationTheTenantIsAbsent(t *testing.T) {
	t.Parallel()
	verifier := tenantVerifier(t)
	cases := []struct {
		name, username, scope string
		organizations         int
	}{
		{name: "bob with every organization", username: "bob", scope: "organization:*", organizations: 2},
		{name: "carol without organization", username: "carol", scope: "organization", organizations: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokens, _ := newBrowser(t).signIn(seedAccount(t, tc.username), tc.scope)

			organizations, _ := tokenClaims(t, tokens.AccessToken)["organization"].(map[string]any)
			if len(organizations) != tc.organizations {
				t.Fatalf("organization claim has %d entries, want %d", len(organizations), tc.organizations)
			}
			requireTenant(t, authenticate(t, verifier, tokens.AccessToken), "")
		})
	}
}

func TestKeycloakRefusesDirectGrantForEveryClient(t *testing.T) {
	t.Parallel()
	issuer := keycloakIssuer(t)
	alice := seedAccount(t, "alice")
	var clients []clientFlows
	masterAdmin(t).decode("/clients", &clients)
	if !slices.ContainsFunc(clients, func(c clientFlows) bool { return c.ClientID == portalClient }) {
		t.Fatalf("GET /clients listed %d clients without %s: the Admin REST must list the whole realm", len(clients), portalClient)
	}
	for _, c := range clients {
		if c.DirectGrant || c.Implicit {
			t.Errorf("client %s has directAccessGrantsEnabled=%t implicitFlowEnabled=%t, want both false",
				c.ClientID, c.DirectGrant, c.Implicit)
		}
	}
	cases := map[string]string{portalClient: "unauthorized_client", "admin-cli": "invalid_client"}
	for client, wantError := range cases {
		t.Run(client, func(t *testing.T) {
			t.Parallel()
			status, tokens := requestToken(t, issuer, url.Values{
				"grant_type": {"password"},
				"client_id":  {client},
				"username":   {alice.username},
				"password":   {alice.password},
				"scope":      {"openid"},
			})

			if tokens.AccessToken != "" || tokens.Error != wantError {
				t.Fatalf("password grant = HTTP %d %q (%s), want %s and no token: no client of the realm takes direct grants",
					status, tokens.Error, tokens.ErrorDescription, wantError)
			}
		})
	}
}

func TestKeycloakPortalAuthorizesOnlyCodeWithS256(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, wantError string
		alter           func(b *browser, params url.Values)
	}{
		{name: "implicit", wantError: "unauthorized_client", alter: func(_ *browser, params url.Values) {
			params.Set("response_type", "token")
		}},
		{name: "without PKCE", wantError: "invalid_request", alter: func(_ *browser, params url.Values) {
			params.Del("code_challenge")
			params.Del("code_challenge_method")
		}},
		{name: "plain PKCE", wantError: "invalid_request", alter: func(b *browser, params url.Values) {
			params.Set("code_challenge", b.verifier)
			params.Set("code_challenge_method", "plain")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newBrowser(t)
			params := b.codeRequest("")
			tc.alter(b, params)

			s := b.authorize(params)

			if s.callback.Get("code") != "" || s.callback.Get("access_token") != "" || s.callback.Get("error") != tc.wantError {
				t.Fatalf("authorization ended in %s, want a %s redirect without code or token", s, tc.wantError)
			}
		})
	}
}

func TestKeycloakSecondFactorGatesOnlyMFARequired(t *testing.T) {
	t.Parallel()
	verifier := tenantVerifier(t)

	t.Run("mfa-required user", func(t *testing.T) {
		t.Parallel()
		user, key := disposableMFAUser(t)
		b := newBrowser(t)

		s, _ := b.login(user, b.authorize(b.codeRequest("organization:acme")))
		if s.form == nil || s.form.kind != formOTP || s.callback.Get("code") != "" {
			t.Fatalf("after the password %s reached %s, want the OTP form and no code", user.username, s)
		}
		s = b.submit(s.form, url.Values{"otp": {totp(key, time.Now())}})

		tokens := b.exchange(s.code(t))
		requireTenant(t, authenticate(t, verifier, tokens.AccessToken), "t-acme")
	})
	t.Run("alice", func(t *testing.T) {
		t.Parallel()
		_, answered := newBrowser(t).signIn(seedAccount(t, "alice"), "organization:acme")

		if want := []formKind{formUsername, formPassword}; !slices.Equal(answered, want) {
			t.Fatalf("alice answered %v, want %v: without mfa-required there is no second factor", answered, want)
		}
	})
}

func TestKeycloakLogoutDeliversAVerifiableLogoutToken(t *testing.T) {
	t.Parallel()
	issuer := keycloakIssuer(t)
	callbackHost := tb.Env(t, "KEYCLOAK_CALLBACK_HOST")
	alice := seedAccount(t, "alice")
	received := make(chan string, 1)
	port := serveBackchannelLogout(t, received)
	client := disposablePortal(t, "http://"+net.JoinHostPort(callbackHost, port)+"/backchannel-logout")

	b := browserFor(t, client)
	tokens, _ := b.signIn(alice, "organization:acme")
	b.logout(tokens.IDToken)

	var raw string
	select {
	case raw = <-received:
	case <-time.After(eventDeadline):
		t.Fatalf("no logout_token within %s: Keycloak must POST to the back-channel URL of %s", eventDeadline, client)
	}
	provider, err := oidc.NewProvider(t.Context(), issuer)
	if err != nil {
		t.Fatalf("oidc.NewProvider(%s) err = %v", issuer, err)
	}
	logout, err := provider.Verifier(&oidc.Config{ClientID: client}).Verify(t.Context(), raw)
	if err != nil {
		t.Fatalf("logout token verification err = %v: signature, iss and aud=%s must hold", err, client)
	}
	var claims struct {
		SessionID string                     `json:"sid"`
		Events    map[string]json.RawMessage `json:"events"`
	}
	if err := logout.Claims(&claims); err != nil {
		t.Fatalf("logout token claims err = %v", err)
	}
	if _, ok := claims.Events[backchannelLogoutEvent]; !ok {
		t.Fatalf("logout token events = %v, want the back-channel logout event", claims.Events)
	}
	access := tokenClaims(t, tokens.AccessToken)
	if logout.Subject != claimText(access, "sub") || claims.SessionID == "" || claims.SessionID != claimText(access, "sid") {
		t.Fatalf("logout token sub/sid = %q/%q, want the access token's %q/%q",
			logout.Subject, claims.SessionID, claimText(access, "sub"), claimText(access, "sid"))
	}
}

func serveBackchannelLogout(t *testing.T, received chan<- string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("net.Listen err = %v: the receiver must be reachable from the Keycloak container", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/backchannel-logout" || r.ParseForm() != nil {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		select {
		case received <- r.PostForm.Get("logout_token"):
		default:
		}
	}))
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func TestKeycloakDisablingAUserIsAnAdminEventForSessionEvents(t *testing.T) {
	t.Parallel()
	admin := masterAdmin(t)
	reader := sessionEventsReader(t)
	id := admin.create("/users", map[string]any{
		"username": "dmpf-it-" + strings.ToLower(rand.Text()),
		"enabled":  true,
	})
	removeOnCleanup(t, "/users/"+id)

	admin.call(http.MethodPut, "/users/"+id, map[string]any{"enabled": false}, http.StatusNoContent)

	query := url.Values{"resourceTypes": {"USER"}, "operationTypes": {"UPDATE"}, "resourcePath": {"users/" + id}}
	for deadline := time.Now().Add(eventDeadline); ; time.Sleep(250 * time.Millisecond) {
		body, _ := reader.call(http.MethodGet, "/admin-events?"+query.Encode(), nil, http.StatusOK)
		var events []struct {
			ResourcePath string            `json:"resourcePath"`
			Details      map[string]string `json:"details"`
		}
		if err := json.Unmarshal(body, &events); err != nil {
			t.Fatalf("admin-events is not a JSON list: %v", err)
		}
		for _, e := range events {
			if e.ResourcePath == "users/"+id && e.Details["previous_enabled"] == "true" && e.Details["updated_enabled"] == "false" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no UPDATE USER event disabling users/%s within %s: got %s", id, eventDeadline, body)
		}
	}
}
