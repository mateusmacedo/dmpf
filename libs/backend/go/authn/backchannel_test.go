package authn_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
)

const (
	portalClientID   = "dmpf-portal"
	logoutEventClaim = "http://schemas.openid.net/event/backchannel-logout"
)

type backchannelFixture struct {
	provider *idp
	store    *countingStore
	sink     *records
	raw      *records
	handler  *authn.BackchannelLogout
}

func newBackchannelFixture(t *testing.T) *backchannelFixture {
	t.Helper()

	f := &backchannelFixture{provider: newIdP(t), sink: &records{}, raw: &records{}}
	f.store = &countingStore{inner: authn.NewMemoryStore(nil)}
	feed, err := authn.NewFeed(f.store, authn.FeedConfig{
		Issuer: f.provider.issuer, TTL: 5 * time.Minute, Skew: time.Minute, LoggerProvider: f.sink.apiProvider(f.raw),
	})
	if err != nil {
		t.Fatalf("NewFeed() err = %v", err)
	}
	f.handler, err = authn.NewBackchannelLogout(t.Context(), feed, authn.BackchannelLogoutConfig{
		ClientID: portalClientID,
	})
	if err != nil {
		t.Fatalf("NewBackchannelLogout() err = %v", err)
	}
	return f
}

func (p *idp) logoutToken(t *testing.T, overrides map[string]any) string {
	t.Helper()

	claims := map[string]any{
		"iss":    p.issuer,
		"aud":    portalClientID,
		"sub":    aliceSubject,
		"sid":    aliceSession,
		"iat":    time.Now().Unix(),
		"exp":    time.Now().Add(2 * time.Minute).Unix(),
		"jti":    "0f9e-logout-1",
		"typ":    "Logout",
		"events": map[string]any{logoutEventClaim: map[string]any{}},
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

func postLogout(handler http.Handler, form url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/backchannel-logout", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func requireNoStore(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store on every answer (Back-Channel Logout 1.0 §2.8)", got)
	}
}

func requireInvalidRequest(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Error != "invalid_request" {
		t.Fatalf("body = %s (%v), want {\"error\":\"invalid_request\"}", response.Body.String(), err)
	}
}

func TestBackchannelLogoutRevokesTheSessionOfAValidToken(t *testing.T) {
	f := newBackchannelFixture(t)
	issued := time.Now().Add(-3 * time.Second).Truncate(time.Second)

	response := postLogout(f.handler, url.Values{"logout_token": {f.provider.logoutToken(t, map[string]any{"iat": issued.Unix()})}})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", response.Code, response.Body)
	}
	requireNoStore(t, response)
	calls := f.store.calls()
	if len(calls) != 1 {
		t.Fatalf("%d revocations recorded, want 1", len(calls))
	}
	got := calls[0]
	if got.Kind != authn.RevocationSession || got.SessionID != aliceSession || got.EventID != "0f9e-logout-1" ||
		!got.OccurredAt.Equal(issued) {
		t.Fatalf("revocation = %+v, want the session %q at iat %v keyed by the jti", got, aliceSession, issued)
	}
}

func TestBackchannelLogoutRevokesTheSubjectOfATokenWithoutSid(t *testing.T) {
	f := newBackchannelFixture(t)
	issued := time.Now().Add(-3 * time.Second).Truncate(time.Second)
	token := f.provider.logoutToken(t, map[string]any{"sid": nil, "iat": issued.Unix()})

	response := postLogout(f.handler, url.Values{"logout_token": {token}})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200: without sid every session of the sub ends (Back-Channel Logout 1.0 §2.7)",
			response.Code, response.Body)
	}
	calls := f.store.calls()
	if len(calls) != 1 {
		t.Fatalf("%d revocations recorded, want 1", len(calls))
	}
	if got := calls[0]; got.Kind != authn.RevocationSubject || got.Subject != aliceSubject || got.SessionID != "" ||
		!got.OccurredAt.Equal(issued) {
		t.Fatalf("revocation = %+v, want the subject %q at iat %v", got, aliceSubject, issued)
	}
}

func TestBackchannelLogoutAcceptsTheSameTokenTwice(t *testing.T) {
	f := newBackchannelFixture(t)
	token := f.provider.logoutToken(t, nil)

	for attempt := range 2 {
		if response := postLogout(f.handler, url.Values{"logout_token": {token}}); response.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200: without dedupe by jti the idempotent Apply absorbs a replay",
				attempt, response.Code)
		}
	}
}

func TestBackchannelLogoutRefusesWhatFailsVerification(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]any
		forge     bool
	}{
		{name: "another issuer", overrides: map[string]any{"iss": "https://keycloak.example/realms/another"}},
		{name: "forged", forge: true},
		{name: "with a nonce", overrides: map[string]any{"nonce": "n-1"}},
		{name: "another audience", overrides: map[string]any{"aud": "dmpf-bff"}},
		{name: "without events", overrides: map[string]any{"events": nil}},
		{name: "expired", overrides: map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}},
		{name: "without iat", overrides: map[string]any{"iat": nil}},
		{name: "iat past the skew", overrides: map[string]any{"iat": time.Now().Add(2 * time.Minute).Unix()}},
		{name: "without jti", overrides: map[string]any{"jti": nil}},
		{name: "without sub and sid", overrides: map[string]any{"sub": nil, "sid": nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newBackchannelFixture(t)
			token := f.provider.logoutToken(t, tt.overrides)
			if tt.forge {
				token = newIdP(t).logoutToken(t, map[string]any{"iss": f.provider.issuer})
			}

			response := postLogout(f.handler, url.Values{"logout_token": {token}})

			requireInvalidRequest(t, response)
			requireNoStore(t, response)
			if calls := f.store.calls(); len(calls) != 0 {
				t.Fatalf("%d revocations recorded from a refused token, want none", len(calls))
			}
		})
	}
}

func TestBackchannelLogoutAcceptsAnIatAheadWithinTheSkew(t *testing.T) {
	f := newBackchannelFixture(t)
	token := f.provider.logoutToken(t, map[string]any{"iat": time.Now().Add(30 * time.Second).Unix()})

	if response := postLogout(f.handler, url.Values{"logout_token": {token}}); response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a clock up to OIDC_REVOCATION_SKEW_SECONDS ahead is tolerated", response.Code)
	}
}

func TestBackchannelLogoutRefusesARequestThatCarriesNoToken(t *testing.T) {
	f := newBackchannelFixture(t)

	t.Run("no logout_token", func(t *testing.T) {
		requireInvalidRequest(t, postLogout(f.handler, url.Values{"other": {"x"}}))
	})
	t.Run("not a token", func(t *testing.T) {
		requireInvalidRequest(t, postLogout(f.handler, url.Values{"logout_token": {"not-a-jwt"}}))
	})
	t.Run("body past the limit", func(t *testing.T) {
		requireInvalidRequest(t, postLogout(f.handler, url.Values{"logout_token": {strings.Repeat("a", 1<<20)}}))
	})
	t.Run("not a POST", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		f.handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/backchannel-logout", nil))
		if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodPost {
			t.Fatalf("status = %d Allow = %q, want 405 allowing POST", recorder.Code, recorder.Header().Get("Allow"))
		}
		requireNoStore(t, recorder)
	})
}

func TestBackchannelLogoutAnswers503WhileTheStoreIsDown(t *testing.T) {
	f := newBackchannelFixture(t)
	f.store.fail = errors.New("redis: dial tcp: refused for dmpf:revocation:sid:" + aliceSession)
	token := f.provider.logoutToken(t, nil)

	response := postLogout(f.handler, url.Values{"logout_token": {token}})

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 so the IdP sees the failure", response.Code)
	}
	requireNoStore(t, response)
	logged := f.sink.all()
	if len(logged) != 1 || logged[0].Body().AsString() != "auth: revocation store unavailable" {
		t.Fatalf("records = %d, want one store unavailable", len(logged))
	}
	if got := attributesOf(logged[0]); got["error.type"] != "TransientDependency" || logged[0].Severity() != log.SeverityWarn {
		t.Fatalf("attributes = %v severity = %v, want a WARN TransientDependency", got, logged[0].Severity())
	}
	requireNothingPersonal(t, f.raw, token, response.Body.String())
}

func TestBackchannelLogoutAnswers503WhileTheSigningKeysAreUnavailable(t *testing.T) {
	f := newBackchannelFixture(t)
	f.provider.keysDown.Store(true)

	response := postLogout(f.handler, url.Values{"logout_token": {f.provider.logoutToken(t, nil)}})

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: keys that cannot be fetched say nothing about the token", response.Code)
	}
}

func TestBackchannelLogoutLogsARefusalWithoutWhoItConcerns(t *testing.T) {
	f := newBackchannelFixture(t)
	token := f.provider.logoutToken(t, map[string]any{"nonce": "n-1"})

	response := postLogout(f.handler, url.Values{"logout_token": {token}})

	requireInvalidRequest(t, response)
	logged := f.sink.all()
	if len(logged) != 1 || attributesOf(logged[0])["dmpf.auth.revocation.outcome"] != "invalid" {
		t.Fatalf("records = %d, want one invalid session event", len(logged))
	}
	requireNothingPersonal(t, f.raw, token, response.Body.String())
}

func requireNothingPersonal(t *testing.T, raw *records, token, body string) {
	t.Helper()

	texts := []string{body}
	for _, record := range raw.all() {
		texts = append(texts, record.Body().AsString())
		for _, value := range attributesOf(record) {
			texts = append(texts, value)
		}
	}
	for _, text := range texts {
		for _, personal := range []string{aliceSubject, aliceSession, token} {
			if strings.Contains(text, personal) {
				t.Errorf("%q carries %q", text, personal)
			}
		}
	}
}

func TestNewBackchannelLogoutRefusesAStartItCouldNotHonour(t *testing.T) {
	provider := newIdP(t)
	feed, err := authn.NewFeed(authn.NewMemoryStore(nil), authn.FeedConfig{Issuer: provider.issuer, TTL: time.Minute})
	if err != nil {
		t.Fatalf("NewFeed() err = %v", err)
	}

	if _, err := authn.NewBackchannelLogout(t.Context(), nil, authn.BackchannelLogoutConfig{ClientID: portalClientID}); !errors.Is(err, authn.ErrNoRevocationFeed) {
		t.Errorf("without a feed: err = %v, want %v", err, authn.ErrNoRevocationFeed)
	}
	if _, err := authn.NewBackchannelLogout(t.Context(), feed, authn.BackchannelLogoutConfig{}); !errors.Is(err, authn.ErrMissingVariable) {
		t.Errorf("without a client id: err = %v, want %v", err, authn.ErrMissingVariable)
	}
}
