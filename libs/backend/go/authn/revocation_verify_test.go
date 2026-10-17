package authn_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	aliceSubject = "f:0e1a:alice"
	aliceSession = "5d1c-session-alice"
)

type recordingChecker struct {
	mu     sync.Mutex
	inner  authn.RevocationChecker
	fail   error
	block  bool
	refs   []authn.SessionRef
	bounds []bool
}

func (c *recordingChecker) Revoked(ctx context.Context, ref authn.SessionRef) (authn.RevocationKind, error) {
	c.mu.Lock()
	c.refs = append(c.refs, ref)
	_, bounded := ctx.Deadline()
	c.bounds = append(c.bounds, bounded)
	fail, block := c.fail, c.block
	c.mu.Unlock()

	if block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if fail != nil {
		return "", fail
	}
	return c.inner.Revoked(ctx, ref)
}

func (c *recordingChecker) failWith(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail = err
}

func (c *recordingChecker) calls() []authn.SessionRef {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]authn.SessionRef(nil), c.refs...)
}

type revocationFixture struct {
	provider *idp
	store    *authn.MemoryStore
	checker  *recordingChecker
	sink     *records
	raw      *records
	verifier *authn.Verifier
}

func newRevocationFixture(t *testing.T, timeout time.Duration, options ...func(*authn.Config)) *revocationFixture {
	t.Helper()

	f := &revocationFixture{provider: newIdP(t), store: authn.NewMemoryStore(nil), sink: &records{}, raw: &records{}}
	f.checker = &recordingChecker{inner: f.store}
	cfg := authn.Defaults()
	cfg.Issuer = f.provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"
	cfg.LoggerProvider = f.sink.apiProvider(f.raw)
	cfg.Revocations = f.checker
	if timeout > 0 {
		cfg.RevocationTimeout = timeout
	}
	for _, option := range options {
		option(&cfg)
	}
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewVerifier() err = %v", err)
	}
	f.verifier = verifier
	return f
}

func (f *revocationFixture) revoke(t *testing.T, r authn.Revocation) {
	t.Helper()
	if r.ExpiresAt.IsZero() {
		r.ExpiresAt = time.Now().Add(6 * time.Minute)
	}
	if _, err := f.store.Revoke(t.Context(), r); err != nil {
		t.Fatalf("Revoke() err = %v", err)
	}
}

func revokedSession(sid string) authn.Revocation {
	return authn.Revocation{EventID: "evt-1", Kind: authn.RevocationSession, Subject: aliceSubject, SessionID: sid,
		OccurredAt: time.Now()}
}

func revokedSubject(at time.Time) authn.Revocation {
	return authn.Revocation{EventID: "evt-2", Kind: authn.RevocationSubject, Subject: aliceSubject, OccurredAt: at}
}

func TestTheVerifierRefusesWhatTheDenyListRevokes(t *testing.T) {
	issued := time.Now().Add(-time.Minute).Truncate(time.Second)

	tests := []struct {
		name    string
		entries []authn.Revocation
		claims  map[string]any
		refused bool
		wantSID string
	}{
		{"nothing revoked", nil,
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, false, aliceSession},
		{"the session is revoked", []authn.Revocation{revokedSession(aliceSession)},
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, true, aliceSession},
		{"another session is revoked", []authn.Revocation{revokedSession("another-session")},
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, false, aliceSession},
		{"the subject is revoked after the token was issued", []authn.Revocation{revokedSubject(issued.Add(time.Second))},
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, true, aliceSession},
		{"the subject is revoked in the second the token was issued", []authn.Revocation{revokedSubject(issued)},
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, true, aliceSession},
		{"the token was issued after the subject was revoked", []authn.Revocation{revokedSubject(issued.Add(-time.Second))},
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, false, aliceSession},
		{"both entries deny the token",
			[]authn.Revocation{revokedSubject(issued), revokedSession(aliceSession)},
			map[string]any{"sid": aliceSession, "iat": issued.Unix()}, true, aliceSession},
		{"a token without sid only answers to the subject entry", []authn.Revocation{revokedSession(aliceSession)},
			map[string]any{"iat": issued.Unix()}, false, ""},
		{"a token without sid is refused by the subject entry", []authn.Revocation{revokedSubject(issued)},
			map[string]any{"iat": issued.Unix()}, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevocationFixture(t, 0)
			for _, entry := range tt.entries {
				f.revoke(t, entry)
			}
			token := f.provider.keycloakToken(t, tt.claims)

			_, err := f.verifier.Authenticate(t.Context(), bearer(token))

			if tt.refused != (err != nil) {
				t.Fatalf("Authenticate() err = %v, refused want %v", err, tt.refused)
			}
			if tt.refused {
				if !errors.Is(err, ports.ErrCredentialRejected) {
					t.Fatalf("Authenticate() err = %v, want ErrCredentialRejected", err)
				}
				logged := f.sink.all()
				if len(logged) != 1 || attributesOf(logged[0])["dmpf.auth.refusal_reason"] != "session_revoked" {
					t.Fatalf("records = %v, want one refusal with reason session_revoked", logged)
				}
				if logged[0].Severity() != log.SeverityWarn || logged[0].Body().AsString() != "auth: token rejected" {
					t.Errorf("record = %q at %v, want the WARN refusal", logged[0].Body().AsString(), logged[0].Severity())
				}
			} else if logged := f.sink.all(); len(logged) != 0 {
				t.Errorf("records = %v, want none for an accepted token", logged)
			}
			calls := f.checker.calls()
			if len(calls) != 1 {
				t.Fatalf("checker consulted %d times, want once", len(calls))
			}
			want := authn.SessionRef{Subject: aliceSubject, ID: tt.wantSID, IssuedAt: issued}
			if calls[0].Subject != want.Subject || calls[0].ID != want.ID || !calls[0].IssuedAt.Equal(want.IssuedAt) {
				t.Errorf("SessionRef = %+v, want %+v", calls[0], want)
			}
			assertNothingPersonalWasEmitted(t, f, token)
		})
	}
}

func TestAnUnavailableDenyListFailsClosedAndRecoversWithoutRestart(t *testing.T) {
	f := newRevocationFixture(t, 0)
	token := f.provider.keycloakToken(t, map[string]any{"sid": aliceSession})
	f.checker.failWith(errors.New("store: connection refused to " + aliceSubject))

	_, err := f.verifier.Authenticate(t.Context(), bearer(token))

	if !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("Authenticate() err = %v, want ErrCredentialRejected: the edge fails closed", err)
	}
	assertStoreUnavailableLogged(t, f)
	if strings.Contains(err.Error(), aliceSubject) {
		t.Errorf("Authenticate() err = %q carries the subject the store echoed", err)
	}
	assertNothingPersonalWasEmitted(t, f, token)

	f.checker.failWith(nil)
	if _, err := f.verifier.Authenticate(t.Context(), bearer(token)); err != nil {
		t.Fatalf("Authenticate() after the store came back err = %v, want the token accepted again", err)
	}
}

func TestADenyListSlowerThanTheTimeoutFailsClosed(t *testing.T) {
	f := newRevocationFixture(t, 20*time.Millisecond)
	f.checker.block = true

	began := time.Now()
	_, err := f.verifier.Authenticate(t.Context(), bearer(f.provider.keycloakToken(t, nil)))
	elapsed := time.Since(began)

	if !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("Authenticate() err = %v, want ErrCredentialRejected", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Authenticate() took %v, want it bounded by OIDC_REVOCATION_TIMEOUT_MS", elapsed)
	}
	if f.checker.bounds[0] != true {
		t.Error("the checker received a context without deadline")
	}
	assertStoreUnavailableLogged(t, f)
}

func TestATokenRefusedBeforeTheDenyListNeverReachesIt(t *testing.T) {
	f := newRevocationFixture(t, 0)
	other := newIdP(t)

	tests := []struct {
		name  string
		token func() string
	}{
		{"expired", func() string {
			return f.provider.keycloakToken(t, map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})
		}},
		{"forged", func() string { return other.keycloakToken(t, map[string]any{"iss": f.provider.issuer}) }},
		{"another audience", func() string { return f.provider.keycloakToken(t, map[string]any{"aud": "another-api"}) }},
		{"without subject", func() string { return f.provider.keycloakToken(t, map[string]any{"sub": nil}) }},
		{"malformed", func() string { return "not-a-jwt" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(f.checker.calls())
			if _, err := f.verifier.Authenticate(t.Context(), bearer(tt.token())); err == nil {
				t.Fatal("Authenticate() err = nil, want the token refused")
			}
			if after := len(f.checker.calls()); after != before {
				t.Fatalf("checker consulted %d times, want none for a token refused before", after-before)
			}
		})
	}
}

func assertStoreUnavailableLogged(t *testing.T, f *revocationFixture) {
	t.Helper()

	logged := f.sink.all()
	if len(logged) != 1 {
		t.Fatalf("%d records, want one", len(logged))
	}
	got := attributesOf(logged[0])
	if body := logged[0].Body().AsString(); body != "auth: revocation store unavailable" {
		t.Errorf("body = %q, want the store unavailable", body)
	}
	if reason, refused := got["dmpf.auth.refusal_reason"]; refused || got["error.type"] != "TransientDependency" {
		t.Errorf("error.type = %q refusal_reason = %q, want TransientDependency and no refusal", got["error.type"], reason)
	}
	if logged[0].Severity() != log.SeverityWarn {
		t.Errorf("severity = %v, want WARN", logged[0].Severity())
	}
}

func assertNothingPersonalWasEmitted(t *testing.T, f *revocationFixture, token string) {
	t.Helper()

	for _, record := range f.raw.all() {
		texts := []string{record.Body().AsString()}
		for _, value := range attributesOf(record) {
			texts = append(texts, value)
		}
		for _, text := range texts {
			for _, personal := range []string{aliceSubject, aliceSession, token} {
				if strings.Contains(text, personal) {
					t.Errorf("record %q carries %q", record.Body().AsString(), personal)
				}
			}
		}
	}
}

func TestCTX16TheSameTokenIsAcceptedThenRefusedWhenRevokedInBetween(t *testing.T) {
	f := newRevocationFixture(t, 0)
	token := f.provider.keycloakToken(t, map[string]any{"sid": aliceSession})

	if _, err := f.verifier.Authenticate(t.Context(), bearer(token)); err != nil {
		t.Fatalf("first Authenticate() err = %v, want the token accepted before the revocation (CTX-16)", err)
	}
	f.revoke(t, revokedSession(aliceSession))
	_, err := f.verifier.Authenticate(t.Context(), bearer(token))

	if !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("second Authenticate() err = %v, want ErrCredentialRejected: an answer outlived its request (CTX-16)", err)
	}
	if calls := len(f.checker.calls()); calls != 2 {
		t.Fatalf("checker consulted %d times, want once per request: nothing may cache the absence (CTX-16)", calls)
	}
}
