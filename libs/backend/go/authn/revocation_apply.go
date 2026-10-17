package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type SessionEventKind string

const (
	SessionEnded   SessionEventKind = "session_ended"
	SubjectRevoked SessionEventKind = "subject_revoked"
)

// SessionEvent is an IdP session event already normalized by its adapter. The
// adapter applies only events whose origin it verified: a logout token with
// signature, iss, aud and events checked, or the issuer's own Admin API.
type SessionEvent struct {
	EventID    string
	Kind       SessionEventKind
	Issuer     string
	Subject    ports.SubjectID
	SessionID  string
	OccurredAt time.Time
}

type EventOutcome string

const (
	EventApplied       EventOutcome = "applied"
	EventDuplicate     EventOutcome = "duplicate"
	EventExpired       EventOutcome = "expired"
	EventInvalid       EventOutcome = "invalid"
	EventForeignIssuer EventOutcome = "foreign_issuer"
)

var ErrNoRevocationStore = errors.New("auth: the revocation feed has no store")

const (
	envRevocationTTL  = "OIDC_REVOCATION_TTL_SECONDS"
	envRevocationSkew = "OIDC_REVOCATION_SKEW_SECONDS"
)

const defaultRevocationSkewSeconds = 60

const (
	keyEventOutcomeLog = "dmpf.auth.revocation.outcome"
	categoryValidation = "Validation"
)

// FeedConfig sizes the entries: each one lives until OccurredAt + TTL + Skew,
// past the expiry of the longest-lived token issued before the event. TTL is
// the longest token lifetime the IdP issues and has no default.
type FeedConfig struct {
	Issuer         string
	TTL            time.Duration
	Skew           time.Duration
	Clock          clock.Clock
	LoggerProvider log.LoggerProvider
	MeterProvider  metric.MeterProvider
}

type Feed struct {
	store   RevocationStore
	issuer  string
	keep    time.Duration
	skew    time.Duration
	clock   clock.Clock
	logger  *slog.Logger
	metrics feedMetrics
}

func ReadRevocationFeedEnv(lookup func(string) string) (FeedConfig, error) {
	cfg := FeedConfig{Issuer: lookup(envIssuer)}

	if declared := lookup(envRevocationTTL); declared != "" {
		ttl, err := parseSeconds(envRevocationTTL, declared, 1)
		if err != nil {
			return FeedConfig{}, err
		}
		cfg.TTL = ttl
	}

	cfg.Skew = defaultRevocationSkewSeconds * time.Second
	if declared := lookup(envRevocationSkew); declared != "" {
		skew, err := parseSeconds(envRevocationSkew, declared, 0)
		if err != nil {
			return FeedConfig{}, err
		}
		cfg.Skew = skew
	}
	return cfg, nil
}

func parseSeconds(variable, value string, least int64) (time.Duration, error) {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < least || seconds > math.MaxInt64/int64(time.Second) {
		return 0, fmt.Errorf("%w: %s=%q is not a number of seconds from %d up to the largest duration",
			envconfig.ErrInvalidVariable, variable, value, least)
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseMilliseconds(variable, value string) (time.Duration, error) {
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds < 1 || milliseconds > math.MaxInt64/int64(time.Millisecond) {
		return 0, fmt.Errorf("%w: %s=%q is not a number of milliseconds from 1 up to the largest duration",
			envconfig.ErrInvalidVariable, variable, value)
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

func NewFeed(store RevocationStore, cfg FeedConfig) (*Feed, error) {
	switch {
	case store == nil:
		return nil, ErrNoRevocationStore
	case cfg.Issuer == "":
		return nil, fmt.Errorf("%w: %s", ErrMissingVariable, envIssuer)
	case cfg.TTL <= 0:
		return nil, fmt.Errorf("%w: %s", ErrMissingVariable, envRevocationTTL)
	case cfg.Skew < 0:
		return nil, fmt.Errorf("%w: %s must not be negative, got %v",
			envconfig.ErrInvalidVariable, envRevocationSkew, cfg.Skew)
	case cfg.TTL > math.MaxInt64-cfg.Skew:
		return nil, fmt.Errorf("%w: %s added to %s exceeds the largest duration",
			envconfig.ErrInvalidVariable, envRevocationSkew, envRevocationTTL)
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System()
	}
	return &Feed{
		store:   store,
		issuer:  cfg.Issuer,
		keep:    cfg.TTL + cfg.Skew,
		skew:    cfg.Skew,
		clock:   cfg.Clock,
		logger:  logging.NewLogger(cfg.LoggerProvider, reflect.TypeFor[Feed]().PkgPath()),
		metrics: newFeedMetrics(cfg.MeterProvider),
	}, nil
}

func (f *Feed) Apply(ctx context.Context, e SessionEvent) (EventOutcome, error) {
	revocation, outcome := f.revocationOf(e)
	if outcome != "" {
		return outcome, nil
	}
	if !revocation.ExpiresAt.After(f.clock.Now()) {
		return EventExpired, nil
	}
	changed, err := f.store.Revoke(ctx, revocation)
	if err != nil {
		return "", fmt.Errorf("auth: recording the revocation: %w", err)
	}
	if !changed {
		return EventDuplicate, nil
	}
	return EventApplied, nil
}

// Record counts and logs the outcome Apply returned. It is separate so that an
// adapter that reapplies the same events counts each one once. The empty outcome
// of a store failure is the caller's error to report, not a refused event.
func (f *Feed) Record(ctx context.Context, e SessionEvent, outcome EventOutcome) {
	switch outcome {
	case "":
		return
	case EventApplied:
		f.metrics.delayed(ctx, f.clock.Now().Sub(e.OccurredAt))
	case EventDuplicate, EventExpired, EventForeignIssuer:
	default:
		outcome = EventInvalid
	}
	f.metrics.observed(ctx, outcome)
	if outcome == EventInvalid || outcome == EventForeignIssuer {
		f.logger.WarnContext(ctx, "auth: session event refused",
			slog.String(string(semconv.ErrorTypeKey), categoryValidation), slog.String(keyEventOutcomeLog, string(outcome)))
	}
}

// unavailable logs what an adapter could not reach, never the error itself: its
// text may echo the key the store was asked for.
func (f *Feed) unavailable(ctx context.Context, message string, attrs ...slog.Attr) {
	attrs = append(attrs, slog.String(string(semconv.ErrorTypeKey), categoryTransientDependency))
	f.logger.LogAttrs(ctx, logging.Severity(logging.Client, ports.OutcomeFailed), message, attrs...)
}

func (f *Feed) revocationOf(e SessionEvent) (Revocation, EventOutcome) {
	r := Revocation{
		EventID:    e.EventID,
		Subject:    e.Subject,
		SessionID:  e.SessionID,
		OccurredAt: e.OccurredAt,
		ExpiresAt:  e.OccurredAt.Add(f.keep),
	}
	switch e.Kind {
	case SessionEnded:
		r.Kind = RevocationSession
	case SubjectRevoked:
		r.Kind, r.SessionID = RevocationSubject, ""
	}
	switch {
	case e.EventID == "" || e.Issuer == "" || e.OccurredAt.IsZero() || r.keyed() != nil:
		return Revocation{}, EventInvalid
	case e.Issuer != f.issuer:
		return Revocation{}, EventForeignIssuer
	}
	return r, ""
}
