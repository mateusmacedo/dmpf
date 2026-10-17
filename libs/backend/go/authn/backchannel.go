package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var ErrNoRevocationFeed = errors.New("auth: the session event adapter has no revocation feed")

const (
	envBackchannelClientID  = "OIDC_BACKCHANNEL_CLIENT_ID"
	maxLogoutRequestBytes   = 64 << 10
	formLogoutToken         = "logout_token"
	invalidLogoutRequestDoc = `{"error":"invalid_request"}`
)

// BackchannelLogoutConfig names the client the IdP addresses its logout tokens
// to: their aud is that client, not the audience of the access token.
type BackchannelLogoutConfig struct {
	ClientID         string
	DiscoveryTimeout time.Duration
}

// BackchannelLogout receives OpenID Connect Back-Channel Logout 1.0 requests and
// applies each verified logout token as a session_ended event, or subject_revoked
// without sid (§2.7). A replayed token is applied again: the feed is idempotent.
type BackchannelLogout struct {
	feed   *Feed
	tokens *oidc.IDTokenVerifier
}

func NewBackchannelLogout(ctx context.Context, feed *Feed, cfg BackchannelLogoutConfig) (*BackchannelLogout, error) {
	switch {
	case feed == nil:
		return nil, ErrNoRevocationFeed
	case cfg.ClientID == "":
		return nil, fmt.Errorf("%w: %s", ErrMissingVariable, envBackchannelClientID)
	}
	if cfg.DiscoveryTimeout <= 0 {
		cfg.DiscoveryTimeout = defaultDiscoveryTimeoutSeconds * time.Second
	}

	discovery, cancel := context.WithTimeout(ctx, cfg.DiscoveryTimeout)
	defer cancel()
	client := &http.Client{Timeout: cfg.DiscoveryTimeout, Transport: otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method }))}
	provider, err := oidc.NewProvider(oidc.ClientContext(discovery, client), feed.issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovering %s: %w", feed.issuer, err)
	}

	return &BackchannelLogout{
		feed:   feed,
		tokens: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID, Now: feed.clock.Now}),
	}, nil
}

func (b *BackchannelLogout) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLogoutRequestBytes)
	if err := r.ParseForm(); err != nil || r.PostForm.Get(formLogoutToken) == "" {
		b.refuse(r.Context(), w)
		return
	}

	event, keysRead := b.verify(r.Context(), r.PostForm.Get(formLogoutToken))
	switch {
	case !keysRead:
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	case event == nil:
		b.refuse(r.Context(), w)
		return
	}

	outcome, err := b.feed.Apply(r.Context(), *event)
	if err != nil {
		b.feed.unavailable(r.Context(), "auth: revocation store unavailable")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	b.feed.Record(r.Context(), *event, outcome)
	if outcome == EventInvalid || outcome == EventForeignIssuer {
		writeInvalidRequest(w)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *BackchannelLogout) verify(ctx context.Context, raw string) (event *SessionEvent, keysRead bool) {
	token, err := b.tokens.VerifyLogout(ctx, raw)
	if err != nil {
		if _, refused := refusalOf(err); !refused {
			b.feed.unavailable(ctx, "auth: signing keys unavailable")
			return nil, false
		}
		return nil, true
	}
	if token.IssuedAt.IsZero() || token.IssuedAt.After(b.feed.clock.Now().Add(b.feed.skew)) {
		return nil, true
	}
	kind := SessionEnded
	if token.SessionID == "" {
		kind = SubjectRevoked
	}
	return &SessionEvent{
		EventID:    token.TokenID,
		Kind:       kind,
		Issuer:     token.Issuer,
		Subject:    ports.SubjectID(token.Subject),
		SessionID:  token.SessionID,
		OccurredAt: token.IssuedAt,
	}, true
}

func (b *BackchannelLogout) refuse(ctx context.Context, w http.ResponseWriter) {
	b.feed.Record(ctx, SessionEvent{}, EventInvalid)
	writeInvalidRequest(w)
}

func writeInvalidRequest(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(invalidLogoutRequestDoc))
}
