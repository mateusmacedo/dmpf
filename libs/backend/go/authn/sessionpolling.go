package authn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var (
	ErrNotAKeycloakRealm = errors.New("auth: the issuer is not a Keycloak realm URL")

	errEventsUnavailable = errors.New("session events unavailable")
	errStoreUnavailable  = errors.New("revocation store unavailable")
	errTooManyPages      = errors.New("the window did not end within the page limit")
)

const (
	envSessionEventsClientID     = "OIDC_SESSION_EVENTS_CLIENT_ID"
	envSessionEventsClientSecret = "OIDC_SESSION_EVENTS_CLIENT_SECRET"

	defaultPollingInterval = 5 * time.Second
	defaultPollingPageSize = 100
	defaultPollingTimeout  = 10 * time.Second
	maxPollingPages        = 1000
	maxPollingPageBytes    = 8 << 20

	keyEventSource = "dmpf.auth.revocation.source"
)

// SessionPollingConfig is the service account that reads the realm events; it
// needs the realm-management role view-events and nothing else.
type SessionPollingConfig struct {
	ClientID       string
	ClientSecret   string
	Interval       time.Duration
	PageSize       int
	RequestTimeout time.Duration
}

// SessionPolling rereads, every cycle and without a cursor, the admin and LOGOUT
// events of [now − (TTL + skew), now]: Keycloak orders them by time alone, and the
// reapply rewrites what the store lost. Seen ids only keep each event counted once.
type SessionPolling struct {
	feed        *Feed
	credentials clientcredentials.Config
	tokenHTTP   *http.Client
	client      *http.Client
	admin       string
	interval    time.Duration
	pageSize    int
	sources     []pollSource

	mu   sync.Mutex
	seen map[string]time.Time
}

type pollSource struct {
	name   string
	path   string
	filter url.Values
	decode func(body []byte) ([]*SessionEvent, error)
}

func NewSessionPolling(feed *Feed, cfg SessionPollingConfig) (*SessionPolling, error) {
	switch {
	case feed == nil:
		return nil, ErrNoRevocationFeed
	case cfg.ClientID == "":
		return nil, fmt.Errorf("%w: %s", ErrMissingVariable, envSessionEventsClientID)
	case cfg.ClientSecret == "":
		return nil, fmt.Errorf("%w: %s", ErrMissingVariable, envSessionEventsClientSecret)
	}
	realm := strings.LastIndex(feed.issuer, "/realms/")
	if realm < 0 || strings.Contains(feed.issuer[realm+len("/realms/"):], "/") {
		return nil, fmt.Errorf("%w: %s", ErrNotAKeycloakRealm, feed.issuer)
	}
	cfg = withPollingDefaults(cfg)

	p := &SessionPolling{
		feed: feed,
		credentials: clientcredentials.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TokenURL:     feed.issuer + "/protocol/openid-connect/token",
		},
		tokenHTTP: &http.Client{Timeout: cfg.RequestTimeout},
		admin:     feed.issuer[:realm] + "/admin" + feed.issuer[realm:],
		interval:  cfg.Interval,
		pageSize:  cfg.PageSize,
		seen:      make(map[string]time.Time),
	}
	p.authorize()
	p.sources = []pollSource{
		{name: "admin", path: "/admin-events", decode: p.adminEvents,
			filter: url.Values{"resourceTypes": {"USER", "USER_SESSION"}, "operationTypes": {"UPDATE", "ACTION", "DELETE"}}},
		{name: "user", path: "/events", decode: p.userEvents, filter: url.Values{"type": {"LOGOUT"}}},
	}
	return p, nil
}

// authorize builds a client with an empty token cache: the oauth2 ReuseTokenSource
// keeps sending a cached token until its expiry, even after the Admin REST refuses it.
func (p *SessionPolling) authorize() {
	p.client = p.credentials.Client(context.WithValue(context.Background(), oauth2.HTTPClient, p.tokenHTTP))
	p.client.Timeout = p.tokenHTTP.Timeout
}

func withPollingDefaults(cfg SessionPollingConfig) SessionPollingConfig {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultPollingInterval
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPollingPageSize
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaultPollingTimeout
	}
	return cfg
}

func (p *SessionPolling) Run(ctx context.Context) {
	for ctx.Err() == nil {
		_ = p.Poll(ctx)
		timer := p.feed.clock.NewTimer(p.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C():
		}
	}
}

// Poll's error names the failing source and never the store's error text, which
// may echo the key it was asked for.
func (p *SessionPolling) Poll(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	to := p.feed.clock.Now()
	from := to.Add(-p.feed.keep)
	p.forgetBefore(from)

	var failures []error
	for _, source := range p.sources {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := p.pollSource(ctx, source, from, to); err != nil {
			failures = append(failures, fmt.Errorf("auth: polling the %s events: %w", source.name, err))
		}
	}
	return errors.Join(failures...)
}

// Tracked is how many event ids the window still holds.
func (p *SessionPolling) Tracked() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.seen)
}

func (p *SessionPolling) pollSource(ctx context.Context, source pollSource, from, to time.Time) error {
	for page := range maxPollingPages {
		events, err := p.fetch(ctx, source, from, to, page*p.pageSize)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			attrs := []slog.Attr{slog.String(keyEventSource, source.name)}
			if refused, ok := errors.AsType[*eventsStatusError](err); ok {
				attrs = append(attrs, slog.Int(string(semconv.HTTPResponseStatusCodeKey), refused.code))
			}
			p.feed.unavailable(ctx, "auth: session events unavailable", attrs...)
			return err
		}
		for _, event := range events {
			if event == nil {
				continue
			}
			if err := p.apply(ctx, source, *event); err != nil {
				p.feed.unavailable(ctx, "auth: revocation store unavailable", slog.String(keyEventSource, source.name))
				return err
			}
		}
		if len(events) < p.pageSize {
			return nil
		}
	}
	p.feed.unavailable(ctx, "auth: session events window exceeded the page limit", slog.String(keyEventSource, source.name))
	return errTooManyPages
}

func (p *SessionPolling) apply(ctx context.Context, source pollSource, e SessionEvent) error {
	outcome, err := p.feed.Apply(ctx, e)
	if err != nil {
		return errStoreUnavailable
	}
	key := source.name + ":" + e.EventID
	if _, counted := p.seen[key]; !counted {
		p.seen[key] = e.OccurredAt
		p.feed.Record(ctx, e, outcome)
	}
	return nil
}

func (p *SessionPolling) forgetBefore(from time.Time) {
	for key, occurred := range p.seen {
		if occurred.Before(from) {
			delete(p.seen, key)
		}
	}
}

func (p *SessionPolling) fetch(ctx context.Context, source pollSource, from, to time.Time, first int) ([]*SessionEvent, error) {
	query := url.Values{
		"dateFrom": {strconv.FormatInt(from.UnixMilli(), 10)},
		"dateTo":   {strconv.FormatInt(to.UnixMilli(), 10)},
		"first":    {strconv.Itoa(first)},
		"max":      {strconv.Itoa(p.pageSize)},
	}
	for name, values := range source.filter {
		query[name] = values
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.admin+source.path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, errEventsUnavailable
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, errEventsUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusUnauthorized {
		p.authorize()
	}
	if response.StatusCode != http.StatusOK {
		return nil, &eventsStatusError{code: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPollingPageBytes))
	if err != nil {
		return nil, errEventsUnavailable
	}
	return source.decode(body)
}

type eventsStatusError struct{ code int }

func (e *eventsStatusError) Error() string {
	return fmt.Sprintf("%v: HTTP %d", errEventsUnavailable, e.code)
}

func (e *eventsStatusError) Unwrap() error { return errEventsUnavailable }

func (p *SessionPolling) adminEvents(body []byte) ([]*SessionEvent, error) {
	var events []struct {
		ID            string            `json:"id"`
		Time          int64             `json:"time"`
		OperationType string            `json:"operationType"`
		ResourceType  string            `json:"resourceType"`
		ResourcePath  string            `json:"resourcePath"`
		Details       map[string]string `json:"details"`
	}
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("%w: the admin events are not a JSON list", errEventsUnavailable)
	}
	polled := make([]*SessionEvent, len(events))
	for i, e := range events {
		switch kind, key := adminRevocation(e.OperationType, e.ResourceType, e.ResourcePath, e.Details); kind {
		case SubjectRevoked:
			polled[i] = p.event(e.ID, kind, key, "", e.Time)
		case SessionEnded:
			polled[i] = p.event(e.ID, kind, "", key, e.Time)
		}
	}
	return polled, nil
}

// adminRevocation recognizes the Keycloak 26.8 admin events that end sessions: a
// user disabled (details.updated_enabled, since 26.8.0), deleted or logged out by
// POST users/{id}/logout ends every session of it; DELETE sessions/{id} ends one.
func adminRevocation(operation, resource, path string, details map[string]string) (SessionEventKind, string) {
	switch resource {
	case "USER":
		user, rest, _ := strings.Cut(strings.TrimPrefix(path, "users/"), "/")
		switch {
		case !strings.HasPrefix(path, "users/") || user == "":
		case operation == "UPDATE" && rest == "" && details["updated_enabled"] == "false",
			operation == "DELETE" && rest == "",
			operation == "ACTION" && rest == "logout":
			return SubjectRevoked, user
		}
	case "USER_SESSION":
		session, found := strings.CutPrefix(path, "sessions/")
		if operation == "DELETE" && found && session != "" && !strings.Contains(session, "/") {
			return SessionEnded, session
		}
	}
	return "", ""
}

// userEvents decodes only what identifies the LOGOUT: the event also carries the
// user's ipAddress and details, which stay unread.
func (p *SessionPolling) userEvents(body []byte) ([]*SessionEvent, error) {
	var events []struct {
		ID        string `json:"id"`
		Time      int64  `json:"time"`
		Type      string `json:"type"`
		UserID    string `json:"userId"`
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("%w: the user events are not a JSON list", errEventsUnavailable)
	}
	polled := make([]*SessionEvent, len(events))
	for i, e := range events {
		if e.Type == "LOGOUT" {
			polled[i] = p.event(e.ID, SessionEnded, e.UserID, e.SessionID, e.Time)
		}
	}
	return polled, nil
}

func (p *SessionPolling) event(id string, kind SessionEventKind, user, session string, millis int64) *SessionEvent {
	return &SessionEvent{
		EventID:    id,
		Kind:       kind,
		Issuer:     p.feed.issuer,
		Subject:    ports.SubjectID(user),
		SessionID:  session,
		OccurredAt: time.UnixMilli(millis),
	}
}
