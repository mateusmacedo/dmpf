package authn_test

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

const feedIssuer = "https://keycloak.example.com/realms/dmpf"

var feedEpoch = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type countingStore struct {
	mu      sync.Mutex
	inner   authn.RevocationStore
	fail    error
	revoked []authn.Revocation
}

func (s *countingStore) Revoked(ctx context.Context, ref authn.SessionRef) (authn.RevocationKind, error) {
	return s.inner.Revoked(ctx, ref)
}

func (s *countingStore) Revoke(ctx context.Context, r authn.Revocation) (bool, error) {
	s.mu.Lock()
	s.revoked = append(s.revoked, r)
	fail := s.fail
	s.mu.Unlock()
	if fail != nil {
		return false, fail
	}
	return s.inner.Revoke(ctx, r)
}

func (s *countingStore) calls() []authn.Revocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]authn.Revocation(nil), s.revoked...)
}

type feedFixture struct {
	clock *clock.Fake
	store *countingStore
	feed  *authn.Feed
}

func feedConfig(c clock.Clock) authn.FeedConfig {
	return authn.FeedConfig{Issuer: feedIssuer, TTL: 5 * time.Minute, Skew: time.Minute, Clock: c}
}

func newFeedFixture(t *testing.T, options ...func(*authn.FeedConfig)) *feedFixture {
	t.Helper()

	fake := clock.NewFake(feedEpoch)
	f := &feedFixture{clock: fake, store: &countingStore{inner: authn.NewMemoryStore(fake)}}
	cfg := feedConfig(fake)
	for _, option := range options {
		option(&cfg)
	}
	feed, err := authn.NewFeed(f.store, cfg)
	if err != nil {
		t.Fatalf("NewFeed() err = %v", err)
	}
	f.feed = feed
	return f
}

func sessionEnded(id, sid string, occurred time.Time) authn.SessionEvent {
	return authn.SessionEvent{
		EventID:    id,
		Kind:       authn.SessionEnded,
		Issuer:     feedIssuer,
		Subject:    aliceSubject,
		SessionID:  sid,
		OccurredAt: occurred,
	}
}

func subjectRevoked(id string, occurred time.Time) authn.SessionEvent {
	return authn.SessionEvent{
		EventID:    id,
		Kind:       authn.SubjectRevoked,
		Issuer:     feedIssuer,
		Subject:    aliceSubject,
		OccurredAt: occurred,
	}
}

func TestApplyRefusesAnEventItCannotTrust(t *testing.T) {
	occurred := feedEpoch.Add(-10 * time.Second)

	tests := []struct {
		name  string
		event func() authn.SessionEvent
		want  authn.EventOutcome
	}{
		{"session_ended without session_id", func() authn.SessionEvent {
			e := sessionEnded("evt-1", aliceSession, occurred)
			e.SessionID = ""
			return e
		}, authn.EventInvalid},
		{"subject_revoked without subject", func() authn.SessionEvent {
			e := subjectRevoked("evt-1", occurred)
			e.Subject = ""
			return e
		}, authn.EventInvalid},
		{"unknown kind", func() authn.SessionEvent {
			e := sessionEnded("evt-1", aliceSession, occurred)
			e.Kind = "token_revoked"
			return e
		}, authn.EventInvalid},
		{"no event_id", func() authn.SessionEvent { return sessionEnded("", aliceSession, occurred) }, authn.EventInvalid},
		{"no occurred_at", func() authn.SessionEvent { return sessionEnded("evt-1", aliceSession, time.Time{}) }, authn.EventInvalid},
		{"no issuer", func() authn.SessionEvent {
			e := sessionEnded("evt-1", aliceSession, occurred)
			e.Issuer = ""
			return e
		}, authn.EventInvalid},
		{"malformed and from another issuer", func() authn.SessionEvent {
			e := subjectRevoked("evt-1", occurred)
			e.Subject, e.Issuer = "", "https://evil.example.com/realms/dmpf"
			return e
		}, authn.EventInvalid},
		{"another issuer", func() authn.SessionEvent {
			e := sessionEnded("evt-1", aliceSession, occurred)
			e.Issuer = "https://evil.example.com/realms/dmpf"
			return e
		}, authn.EventForeignIssuer},
		{"the issuer with a trailing slash", func() authn.SessionEvent {
			e := subjectRevoked("evt-1", occurred)
			e.Issuer = feedIssuer + "/"
			return e
		}, authn.EventForeignIssuer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedFixture(t)

			got, err := f.feed.Apply(t.Context(), tt.event())

			if err != nil {
				t.Fatalf("Apply() err = %v, want an outcome: a refused event is not retried", err)
			}
			if got != tt.want {
				t.Fatalf("Apply() = %q, want %q", got, tt.want)
			}
			if calls := f.store.calls(); len(calls) != 0 {
				t.Fatalf("store received %d revocations, want none for a refused event", len(calls))
			}
		})
	}
}

func TestApplyAcceptsTheKeyEachKindRequires(t *testing.T) {
	occurred := feedEpoch.Add(-10 * time.Second)

	tests := []struct {
		name  string
		event authn.SessionEvent
		want  authn.Revocation
	}{
		{"session_ended", sessionEnded("evt-1", aliceSession, occurred), authn.Revocation{
			EventID: "evt-1", Kind: authn.RevocationSession, Subject: aliceSubject, SessionID: aliceSession,
			OccurredAt: occurred, ExpiresAt: occurred.Add(6 * time.Minute),
		}},
		{"session_ended without subject", func() authn.SessionEvent {
			e := sessionEnded("evt-1", aliceSession, occurred)
			e.Subject = ""
			return e
		}(), authn.Revocation{
			EventID: "evt-1", Kind: authn.RevocationSession, SessionID: aliceSession,
			OccurredAt: occurred, ExpiresAt: occurred.Add(6 * time.Minute),
		}},
		{"subject_revoked", subjectRevoked("evt-2", occurred), authn.Revocation{
			EventID: "evt-2", Kind: authn.RevocationSubject, Subject: aliceSubject,
			OccurredAt: occurred, ExpiresAt: occurred.Add(6 * time.Minute),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedFixture(t)

			got, err := f.feed.Apply(t.Context(), tt.event)

			if err != nil || got != authn.EventApplied {
				t.Fatalf("Apply() = %q, %v; want applied", got, err)
			}
			calls := f.store.calls()
			if len(calls) != 1 || calls[0] != tt.want {
				t.Fatalf("store received %+v, want exactly %+v", calls, tt.want)
			}
		})
	}
}

func TestReadRevocationFeedEnvReadsTheTTLAndTheSkew(t *testing.T) {
	tests := []struct {
		name     string
		pairs    map[string]string
		wantTTL  time.Duration
		wantSkew time.Duration
	}{
		{"the skew defaults to a minute", map[string]string{
			"OIDC_ISSUER": feedIssuer, "OIDC_REVOCATION_TTL_SECONDS": "300",
		}, 5 * time.Minute, time.Minute},
		{"the skew declared", map[string]string{
			"OIDC_ISSUER": feedIssuer, "OIDC_REVOCATION_TTL_SECONDS": "900", "OIDC_REVOCATION_SKEW_SECONDS": "30",
		}, 15 * time.Minute, 30 * time.Second},
		{"a zero skew declared", map[string]string{
			"OIDC_ISSUER": feedIssuer, "OIDC_REVOCATION_TTL_SECONDS": "300", "OIDC_REVOCATION_SKEW_SECONDS": "0",
		}, 5 * time.Minute, 0},
		{"no TTL is read as zero, refused by NewFeed", map[string]string{
			"OIDC_ISSUER": feedIssuer,
		}, 0, time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := authn.ReadRevocationFeedEnv(env(tt.pairs))
			if err != nil {
				t.Fatalf("ReadRevocationFeedEnv() err = %v", err)
			}

			if cfg.Issuer != feedIssuer {
				t.Errorf("Issuer = %q, want %q", cfg.Issuer, feedIssuer)
			}
			if cfg.TTL != tt.wantTTL {
				t.Errorf("TTL = %v, want %v", cfg.TTL, tt.wantTTL)
			}
			if cfg.Skew != tt.wantSkew {
				t.Errorf("Skew = %v, want %v", cfg.Skew, tt.wantSkew)
			}
		})
	}
}

func TestReadRevocationFeedEnvRefusesAnInvalidValue(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    string
	}{
		{"TTL not a number", "OIDC_REVOCATION_TTL_SECONDS", "five minutes"},
		{"TTL zero", "OIDC_REVOCATION_TTL_SECONDS", "0"},
		{"TTL negative", "OIDC_REVOCATION_TTL_SECONDS", "-300"},
		{"skew not a number", "OIDC_REVOCATION_SKEW_SECONDS", "1m"},
		{"skew negative", "OIDC_REVOCATION_SKEW_SECONDS", "-60"},
		{"TTL wrapping into a short duration", "OIDC_REVOCATION_TTL_SECONDS", "18446744074"},
		{"TTL past the largest duration", "OIDC_REVOCATION_TTL_SECONDS", "9300000000"},
		{"skew past the largest duration", "OIDC_REVOCATION_SKEW_SECONDS", "9300000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs := map[string]string{"OIDC_ISSUER": feedIssuer, "OIDC_REVOCATION_TTL_SECONDS": "300"}
			pairs[tt.variable] = tt.value

			_, err := authn.ReadRevocationFeedEnv(env(pairs))

			if !errors.Is(err, envconfig.ErrInvalidVariable) {
				t.Fatalf("ReadRevocationFeedEnv() err = %v, want %v", err, envconfig.ErrInvalidVariable)
			}
			if !strings.Contains(err.Error(), tt.variable) {
				t.Fatalf("the error must name %s, got %q", tt.variable, err.Error())
			}
		})
	}
}

func TestNewFeedRefusesAStartItCouldNotHonour(t *testing.T) {
	tests := []struct {
		name    string
		store   authn.RevocationStore
		mutate  func(*authn.FeedConfig)
		wantErr error
		naming  string
	}{
		{"no TTL", authn.NewMemoryStore(nil), func(cfg *authn.FeedConfig) { cfg.TTL = 0 },
			authn.ErrMissingVariable, "OIDC_REVOCATION_TTL_SECONDS"},
		{"negative TTL", authn.NewMemoryStore(nil), func(cfg *authn.FeedConfig) { cfg.TTL = -time.Minute },
			authn.ErrMissingVariable, "OIDC_REVOCATION_TTL_SECONDS"},
		{"negative skew", authn.NewMemoryStore(nil), func(cfg *authn.FeedConfig) { cfg.Skew = -time.Second },
			envconfig.ErrInvalidVariable, "OIDC_REVOCATION_SKEW_SECONDS"},
		{"TTL plus skew past the largest duration", authn.NewMemoryStore(nil),
			func(cfg *authn.FeedConfig) { cfg.TTL, cfg.Skew = time.Duration(math.MaxInt64), time.Second },
			envconfig.ErrInvalidVariable, "OIDC_REVOCATION_SKEW_SECONDS"},
		{"no issuer", authn.NewMemoryStore(nil), func(cfg *authn.FeedConfig) { cfg.Issuer = "" },
			authn.ErrMissingVariable, "OIDC_ISSUER"},
		{"no store", nil, func(*authn.FeedConfig) {}, authn.ErrNoRevocationStore, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := feedConfig(nil)
			tt.mutate(&cfg)

			feed, err := authn.NewFeed(tt.store, cfg)

			if !errors.Is(err, tt.wantErr) || feed != nil {
				t.Fatalf("NewFeed() = %v, %v; want nil and %v", feed, err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.naming) {
				t.Fatalf("the error must name %s, got %q", tt.naming, err.Error())
			}
		})
	}
}

func TestNewFeedAcceptsAZeroSkewAndTheSystemClock(t *testing.T) {
	cfg := authn.FeedConfig{Issuer: feedIssuer, TTL: time.Minute}

	if _, err := authn.NewFeed(authn.NewMemoryStore(nil), cfg); err != nil {
		t.Fatalf("NewFeed() err = %v, want a feed without skew, clock, logger or meter declared", err)
	}
}

func TestApplyIsIdempotentAndMonotonic(t *testing.T) {
	older, newer := feedEpoch.Add(-2*time.Minute), feedEpoch.Add(-time.Minute)

	tests := []struct {
		name   string
		events []authn.SessionEvent
		want   []authn.EventOutcome
	}{
		{"redelivery", []authn.SessionEvent{subjectRevoked("evt-1", newer), subjectRevoked("evt-1", newer)},
			[]authn.EventOutcome{authn.EventApplied, authn.EventDuplicate}},
		{"an older event after a newer one", []authn.SessionEvent{subjectRevoked("evt-2", newer), subjectRevoked("evt-1", older)},
			[]authn.EventOutcome{authn.EventApplied, authn.EventDuplicate}},
		{"a newer event after an older one", []authn.SessionEvent{subjectRevoked("evt-1", older), subjectRevoked("evt-2", newer)},
			[]authn.EventOutcome{authn.EventApplied, authn.EventApplied}},
		{"the same session redelivered", []authn.SessionEvent{
			sessionEnded("jti-1", aliceSession, newer), sessionEnded("jti-1", aliceSession, newer),
		}, []authn.EventOutcome{authn.EventApplied, authn.EventDuplicate}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedFixture(t)

			var got []authn.EventOutcome
			for _, event := range tt.events {
				outcome, err := f.feed.Apply(t.Context(), event)
				if err != nil {
					t.Fatalf("Apply(%s) err = %v", event.EventID, err)
				}
				got = append(got, outcome)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("outcomes = %v, want %v", got, tt.want)
			}
			if tt.events[0].Kind != authn.SubjectRevoked {
				return
			}
			between := authn.SessionRef{Subject: aliceSubject, IssuedAt: newer.Add(-time.Second)}
			if kind, _ := f.store.Revoked(t.Context(), between); kind != authn.RevocationSubject {
				t.Fatalf("Revoked(iat before the newest event) = %q, want subject: revoked_at went backwards", kind)
			}
		})
	}
}

func TestARedeliveryDoesNotExtendTheEntry(t *testing.T) {
	f := newFeedFixture(t)
	occurred := feedEpoch.Add(-time.Minute)
	ref := authn.SessionRef{Subject: aliceSubject, ID: aliceSession, IssuedAt: occurred}
	if _, err := f.feed.Apply(t.Context(), sessionEnded("jti-1", aliceSession, occurred)); err != nil {
		t.Fatal(err)
	}

	f.clock.Advance(3 * time.Minute)
	if outcome, err := f.feed.Apply(t.Context(), sessionEnded("jti-1", aliceSession, occurred)); err != nil || outcome != authn.EventDuplicate {
		t.Fatalf("Apply(redelivery) = %q, %v; want duplicate", outcome, err)
	}
	f.clock.Advance(3 * time.Minute)

	if kind, _ := f.store.Revoked(t.Context(), ref); kind != "" {
		t.Fatalf("Revoked() at occurred_at + TTL + skew = %q, want the entry gone: the redelivery extended it", kind)
	}
}

func TestApplyIgnoresAnEventPastTheLifeOfAnyToken(t *testing.T) {
	keep := 6 * time.Minute

	tests := []struct {
		name     string
		occurred time.Time
		want     authn.EventOutcome
	}{
		{"long expired", feedEpoch.Add(-time.Hour), authn.EventExpired},
		{"expiring right now", feedEpoch.Add(-keep), authn.EventExpired},
		{"a second left", feedEpoch.Add(-keep + time.Second), authn.EventApplied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedFixture(t)

			got, err := f.feed.Apply(t.Context(), subjectRevoked("evt-1", tt.occurred))

			if err != nil || got != tt.want {
				t.Fatalf("Apply() = %q, %v; want %q", got, err, tt.want)
			}
			if calls := len(f.store.calls()); tt.want == authn.EventExpired && calls != 0 {
				t.Fatalf("store received %d revocations, want none for an expired event", calls)
			}
		})
	}
}

func TestApplyReturnsTheStoreErrorToTheCaller(t *testing.T) {
	f := newFeedFixture(t)
	down := errors.New("store: down")
	f.store.fail = down

	got, err := f.feed.Apply(t.Context(), sessionEnded("jti-1", aliceSession, feedEpoch))

	if !errors.Is(err, down) {
		t.Fatalf("Apply() err = %v, want the store error so the adapter can retry", err)
	}
	if got != "" {
		t.Fatalf("Apply() = %q, want no outcome: nothing was decided about the event", got)
	}
}
