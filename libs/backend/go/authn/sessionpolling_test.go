package authn_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	pollingClientID     = "dmpf-session-events"
	pollingClientSecret = "s3cr3t-session-events"
	pollingToken        = "admin-token-1"
)

type adminEvent struct {
	ID            string            `json:"id"`
	Time          int64             `json:"time"`
	OperationType string            `json:"operationType"`
	ResourceType  string            `json:"resourceType"`
	ResourcePath  string            `json:"resourcePath"`
	Details       map[string]string `json:"details,omitempty"`
}

type userEvent struct {
	ID        string            `json:"id"`
	Time      int64             `json:"time"`
	Type      string            `json:"type"`
	UserID    string            `json:"userId"`
	SessionID string            `json:"sessionId"`
	IPAddress string            `json:"ipAddress"`
	Details   map[string]string `json:"details,omitempty"`
}

type eventQuery struct {
	path     string
	dateFrom int64
	dateTo   int64
	first    int
	max      int
	params   url.Values
}

// keycloakAdmin answers the two Admin REST collections the way Keycloak 26.8
// does: inclusive dateFrom/dateTo in epoch milliseconds, newest first, paged by
// first/max, behind a client-credentials bearer.
type keycloakAdmin struct {
	t       *testing.T
	server  *httptest.Server
	mu      sync.Mutex
	admin   []adminEvent
	user    []userEvent
	down    map[string]bool
	queries []eventQuery
	served  chan struct{}
	issue   []string
	grants  int
}

func newKeycloakAdmin(t *testing.T) *keycloakAdmin {
	t.Helper()

	k := &keycloakAdmin{t: t, down: map[string]bool{}, served: make(chan struct{}, 64)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /realms/dmpf/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok {
			id, secret = r.PostFormValue("client_id"), r.PostFormValue("client_secret")
		}
		if id != pollingClientID || secret != pollingClientSecret || r.PostFormValue("grant_type") != "client_credentials" {
			http.Error(w, `{"error":"unauthorized_client"}`, http.StatusUnauthorized)
			return
		}
		k.mu.Lock()
		token := pollingToken
		if k.grants < len(k.issue) {
			token = k.issue[k.grants]
		}
		k.grants++
		k.mu.Unlock()
		writeJSON(w, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 300})
	})
	mux.HandleFunc("GET /admin/realms/dmpf/admin-events", func(w http.ResponseWriter, r *http.Request) {
		k.serve(w, r, "admin-events", func(q eventQuery) any {
			return page(k.admin, q, func(e adminEvent) int64 { return e.Time })
		})
	})
	mux.HandleFunc("GET /admin/realms/dmpf/events", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "LOGOUT" {
			t.Errorf("events queried with type=%q, want LOGOUT only", r.URL.Query().Get("type"))
		}
		k.serve(w, r, "events", func(q eventQuery) any {
			return page(k.user, q, func(e userEvent) int64 { return e.Time })
		})
	})
	k.server = httptest.NewServer(mux)
	t.Cleanup(k.server.Close)
	return k
}

func (k *keycloakAdmin) issuer() string { return k.server.URL + "/realms/dmpf" }

func (k *keycloakAdmin) serve(w http.ResponseWriter, r *http.Request, path string, answer func(eventQuery) any) {
	defer func() {
		select {
		case k.served <- struct{}{}:
		default:
		}
	}()
	if r.Header.Get("Authorization") != "Bearer "+pollingToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	params := r.URL.Query()
	q := eventQuery{path: path, params: params}
	q.dateFrom, _ = strconv.ParseInt(params.Get("dateFrom"), 10, 64)
	q.dateTo, _ = strconv.ParseInt(params.Get("dateTo"), 10, 64)
	q.first, _ = strconv.Atoi(params.Get("first"))
	q.max, _ = strconv.Atoi(params.Get("max"))

	k.mu.Lock()
	defer k.mu.Unlock()
	k.queries = append(k.queries, q)
	if k.down[path] {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}
	writeJSON(w, answer(q))
}

func page[E any](events []E, q eventQuery, at func(E) int64) []E {
	var window []E
	for _, e := range events {
		if at(e) >= q.dateFrom && at(e) <= q.dateTo {
			window = append(window, e)
		}
	}
	slices.SortStableFunc(window, func(a, b E) int { return cmp.Compare(at(b), at(a)) })
	if q.first >= len(window) {
		return []E{}
	}
	return window[q.first:min(q.first+q.max, len(window))]
}

func (k *keycloakAdmin) addUser(events ...userEvent) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.user = append(k.user, events...)
}

func (k *keycloakAdmin) addAdmin(events ...adminEvent) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.admin = append(k.admin, events...)
}

func (k *keycloakAdmin) setDown(path string, down bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.down[path] = down
}

func (k *keycloakAdmin) issueFirst(tokens ...string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.issue = tokens
}

func (k *keycloakAdmin) tokenGrants() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.grants
}

func (k *keycloakAdmin) takeQueries() []eventQuery {
	k.mu.Lock()
	defer k.mu.Unlock()
	taken := k.queries
	k.queries = nil
	return taken
}

type pollingFixture struct {
	admin   *keycloakAdmin
	clock   *clock.Fake
	store   *countingStore
	reader  *sdkmetric.ManualReader
	sink    *records
	raw     *records
	polling *authn.SessionPolling
}

func newPollingFixture(t *testing.T) *pollingFixture {
	t.Helper()

	f := &pollingFixture{admin: newKeycloakAdmin(t), clock: clock.NewFake(feedEpoch), reader: sdkmetric.NewManualReader(),
		sink: &records{}, raw: &records{}}
	f.store = &countingStore{inner: authn.NewMemoryStore(f.clock)}
	feed, err := authn.NewFeed(f.store, authn.FeedConfig{
		Issuer: f.admin.issuer(), TTL: 5 * time.Minute, Skew: time.Minute, Clock: f.clock,
		LoggerProvider: f.sink.apiProvider(f.raw), MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(f.reader)),
	})
	if err != nil {
		t.Fatalf("NewFeed() err = %v", err)
	}
	f.polling, err = authn.NewSessionPolling(feed, authn.SessionPollingConfig{
		ClientID: pollingClientID, ClientSecret: pollingClientSecret, Interval: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSessionPolling() err = %v", err)
	}
	return f
}

func (f *pollingFixture) ago(d time.Duration) int64 { return f.clock.Now().Add(-d).UnixMilli() }

func (f *pollingFixture) revoked(t *testing.T, ref authn.SessionRef) authn.RevocationKind {
	t.Helper()
	kind, err := f.store.Revoked(t.Context(), ref)
	if err != nil {
		t.Fatalf("Revoked() err = %v", err)
	}
	return kind
}

func (f *pollingFixture) applied(t *testing.T) map[string]int64 {
	t.Helper()
	var data metricdata.ResourceMetrics
	if err := f.reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	got := map[string]int64{}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if sum, ok := m.Data.(metricdata.Sum[int64]); ok && m.Name == "dmpf.auth.revocation.events" {
				for _, point := range sum.DataPoints {
					outcome, _ := point.Attributes.Value("outcome")
					got[outcome.AsString()] += point.Value
				}
			}
		}
	}
	return got
}

func logout(id, user, session string, at int64) userEvent {
	return userEvent{ID: id, Time: at, Type: "LOGOUT", UserID: user, SessionID: session, IPAddress: "203.0.113.7",
		Details: map[string]string{"redirect_uri": "http://127.0.0.1/logged-out"}}
}

func TestSessionPollingReadsEveryPageOfTheWindowWithAFixedDateTo(t *testing.T) {
	f := newPollingFixture(t)
	const total = 230
	var events []userEvent
	for i := range total {
		at := f.ago(time.Duration(i/7) * time.Second)
		events = append(events, logout(fmt.Sprintf("evt-%03d", i), "user-"+strconv.Itoa(i), "sid-"+strconv.Itoa(i), at))
	}
	f.admin.addUser(events...)
	now := f.clock.Now()

	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}

	for i := range total {
		if kind := f.revoked(t, authn.SessionRef{Subject: "other", ID: "sid-" + strconv.Itoa(i)}); kind != authn.RevocationSession {
			t.Fatalf("sid-%d = %q, want session: every LOGOUT of the window is applied, pages and ties included", i, kind)
		}
	}
	var pages []eventQuery
	for _, q := range f.admin.takeQueries() {
		if q.path == "events" {
			pages = append(pages, q)
		}
	}
	if len(pages) != 3 {
		t.Fatalf("%d pages of LOGOUT read, want 3 for %d events of 100 a page, stopping at the incomplete one", len(pages), total)
	}
	for i, q := range pages {
		if q.first != i*100 || q.max != 100 || q.dateTo != now.UnixMilli() || q.dateFrom != now.Add(-6*time.Minute).UnixMilli() {
			t.Errorf("page %d = %+v, want first=%d max=100 over [now-(TTL+skew), now] with dateTo fixed", i, q, i*100)
		}
	}
}

func TestSessionPollingMapsTheAdminEventsThatEndASubject(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addAdmin(
		adminEvent{ID: "a-1", Time: f.ago(10 * time.Second), OperationType: "UPDATE", ResourceType: "USER",
			ResourcePath: "users/disabled", Details: map[string]string{"previous_enabled": "true", "updated_enabled": "false"}},
		adminEvent{ID: "a-2", Time: f.ago(9 * time.Second), OperationType: "ACTION", ResourceType: "USER",
			ResourcePath: "users/logged-out/logout"},
		adminEvent{ID: "a-3", Time: f.ago(8 * time.Second), OperationType: "UPDATE", ResourceType: "USER",
			ResourcePath: "users/enabled", Details: map[string]string{"previous_enabled": "false", "updated_enabled": "true"}},
		adminEvent{ID: "a-4", Time: f.ago(7 * time.Second), OperationType: "UPDATE", ResourceType: "USER",
			ResourcePath: "users/renamed", Details: map[string]string{"previous_username": "a", "updated_username": "b"}},
		adminEvent{ID: "a-5", Time: f.ago(6 * time.Second), OperationType: "ACTION", ResourceType: "USER",
			ResourcePath: "users/reset/reset-password-email"},
		adminEvent{ID: "a-6", Time: f.ago(5 * time.Second), OperationType: "UPDATE", ResourceType: "CLIENT",
			ResourcePath: "clients/x", Details: map[string]string{"updated_enabled": "false"}},
	)
	issuedBefore := f.clock.Now().Add(-time.Minute)

	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}

	for subject, want := range map[ports.SubjectID]authn.RevocationKind{
		"disabled": authn.RevocationSubject, "logged-out": authn.RevocationSubject,
		"enabled": "", "renamed": "", "reset": "", "x": "",
	} {
		if got := f.revoked(t, authn.SessionRef{Subject: subject, IssuedAt: issuedBefore}); got != want {
			t.Errorf("subject %q = %q, want %q", subject, got, want)
		}
	}
	if got := len(f.store.calls()); got != 2 {
		t.Errorf("%d revocations recorded, want 2: an admin event outside the filter never reaches the store", got)
	}
}

func TestSessionPollingMapsTheAdminEventsThatDeleteAUserOrEndOneSession(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addAdmin(
		adminEvent{ID: "d-1", Time: f.ago(10 * time.Second), OperationType: "DELETE", ResourceType: "USER",
			ResourcePath: "users/deleted"},
		adminEvent{ID: "d-2", Time: f.ago(9 * time.Second), OperationType: "DELETE", ResourceType: "USER_SESSION",
			ResourcePath: "sessions/sid-ended"},
		adminEvent{ID: "d-3", Time: f.ago(8 * time.Second), OperationType: "DELETE", ResourceType: "USER",
			ResourcePath: "users/kept/role-mappings/realm"},
		adminEvent{ID: "d-4", Time: f.ago(7 * time.Second), OperationType: "DELETE", ResourceType: "USER",
			ResourcePath: "users/consented/consents/portal"},
		adminEvent{ID: "d-5", Time: f.ago(6 * time.Second), OperationType: "ACTION", ResourceType: "REALM",
			ResourcePath: "logout-all"},
		adminEvent{ID: "d-6", Time: f.ago(5 * time.Second), OperationType: "DELETE", ResourceType: "CLIENT",
			ResourcePath: "clients/x"},
		adminEvent{ID: "d-7", Time: f.ago(4 * time.Second), OperationType: "UPDATE", ResourceType: "USER_SESSION",
			ResourcePath: "sessions/sid-updated"},
	)
	issuedBefore := f.clock.Now().Add(-time.Minute)

	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}

	for subject, want := range map[ports.SubjectID]authn.RevocationKind{
		"deleted": authn.RevocationSubject, "kept": "", "consented": "", "x": "",
	} {
		if got := f.revoked(t, authn.SessionRef{Subject: subject, IssuedAt: issuedBefore}); got != want {
			t.Errorf("subject %q = %q, want %q", subject, got, want)
		}
	}
	for session, want := range map[string]authn.RevocationKind{
		"sid-ended": authn.RevocationSession, "sid-updated": "",
	} {
		if got := f.revoked(t, authn.SessionRef{Subject: "anyone", ID: session}); got != want {
			t.Errorf("session %q = %q, want %q", session, got, want)
		}
	}
	if got := len(f.store.calls()); got != 2 {
		t.Errorf("%d revocations recorded, want 2: only the user deletion and the session end reach the store", got)
	}
	for _, q := range f.admin.takeQueries() {
		if q.path != "admin-events" {
			continue
		}
		if types := q.params["resourceTypes"]; !slices.Equal(types, []string{"USER", "USER_SESSION"}) {
			t.Errorf("resourceTypes = %v, want USER and USER_SESSION", types)
		}
		if operations := q.params["operationTypes"]; !slices.Equal(operations, []string{"UPDATE", "ACTION", "DELETE"}) {
			t.Errorf("operationTypes = %v, want UPDATE, ACTION and DELETE", operations)
		}
	}
}

func TestSessionPollingAsksForANewTokenAfterTheAdminRESTRefusesIt(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.issueFirst("revoked-admin-token")
	f.admin.addAdmin(adminEvent{ID: "a-1", Time: f.ago(2 * time.Second), OperationType: "ACTION", ResourceType: "USER",
		ResourcePath: "users/bob/logout"})
	bob := authn.SessionRef{Subject: "bob", IssuedAt: f.clock.Now().Add(-time.Minute)}

	if err := f.polling.Poll(t.Context()); err == nil {
		t.Fatal("Poll() err = nil, want the refused token reported")
	}
	logged := f.sink.all()
	if len(logged) == 0 || attributesOf(logged[0])["http.response.status_code"] != "401" {
		t.Fatalf("records = %d, want the 401 of the Admin REST logged", len(logged))
	}

	f.clock.Advance(5 * time.Second)
	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() after the refusal err = %v, want the next cycle to recover with a new token", err)
	}
	if kind := f.revoked(t, bob); kind != authn.RevocationSubject {
		t.Fatalf("bob = %q after the recovery, want subject", kind)
	}
	if grants := f.admin.tokenGrants(); grants != 2 {
		t.Fatalf("%d token grants, want 2: the refused token is dropped once, and the new one is reused", grants)
	}
}

func TestSessionPollingKeepsOneSourceWhenTheOtherIsDown(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addUser(logout("u-1", "alice", aliceSession, f.ago(3*time.Second)))
	f.admin.addAdmin(adminEvent{ID: "a-1", Time: f.ago(2 * time.Second), OperationType: "ACTION", ResourceType: "USER",
		ResourcePath: "users/bob/logout"})
	f.admin.setDown("admin-events", true)
	bob := authn.SessionRef{Subject: "bob", IssuedAt: f.clock.Now().Add(-time.Minute)}

	err := f.polling.Poll(t.Context())

	if err == nil {
		t.Fatal("Poll() err = nil, want the admin events failure reported")
	}
	if kind := f.revoked(t, authn.SessionRef{ID: aliceSession}); kind != authn.RevocationSession {
		t.Fatalf("the LOGOUT source answered, yet its session = %q", kind)
	}
	if kind := f.revoked(t, bob); kind != "" {
		t.Fatalf("bob = %q with the admin events down, want nothing yet", kind)
	}
	logged := f.sink.all()
	if len(logged) != 1 || logged[0].Body().AsString() != "auth: session events unavailable" ||
		attributesOf(logged[0])["dmpf.auth.revocation.source"] != "admin" ||
		attributesOf(logged[0])["error.type"] != "TransientDependency" ||
		attributesOf(logged[0])["http.response.status_code"] != "500" {
		t.Fatalf("records = %d, want one admin source unavailable with the HTTP status", len(logged))
	}

	f.admin.setDown("admin-events", false)
	f.clock.Advance(5 * time.Second)
	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() after recovery err = %v", err)
	}
	if kind := f.revoked(t, bob); kind != authn.RevocationSubject {
		t.Fatalf("bob = %q after the admin events came back, want subject", kind)
	}
}

func TestSessionPollingLogsAWindowPastThePageLimit(t *testing.T) {
	f := newPollingFixture(t)
	feed, err := authn.NewFeed(f.store, authn.FeedConfig{
		Issuer: f.admin.issuer(), TTL: 5 * time.Minute, Skew: time.Minute, Clock: f.clock,
		LoggerProvider: f.sink.apiProvider(f.raw),
	})
	if err != nil {
		t.Fatalf("NewFeed() err = %v", err)
	}
	polling, err := authn.NewSessionPolling(feed, authn.SessionPollingConfig{
		ClientID: pollingClientID, ClientSecret: pollingClientSecret, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("NewSessionPolling() err = %v", err)
	}
	for i := range 1001 {
		f.admin.addUser(logout(fmt.Sprintf("evt-%04d", i), "user", "sid-"+strconv.Itoa(i), f.ago(time.Second)))
	}

	if err := polling.Poll(t.Context()); err == nil {
		t.Fatal("Poll() err = nil, want the truncated window reported")
	}

	logged := f.sink.all()
	if len(logged) != 1 || logged[0].Body().AsString() != "auth: session events window exceeded the page limit" ||
		attributesOf(logged[0])["dmpf.auth.revocation.source"] != "user" {
		t.Fatalf("records = %d, want one truncated user window", len(logged))
	}
}

func TestSessionPollingStaysQuietWhenTheContextEnds(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addUser(logout("u-1", "alice", aliceSession, f.ago(3*time.Second)))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := f.polling.Poll(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Poll() err = %v, want %v", err, context.Canceled)
	}
	f.polling.Run(ctx)

	if logged := f.sink.all(); len(logged) != 0 {
		t.Fatalf("records = %d, want none: a stop is not an unavailable dependency", len(logged))
	}
	if queries := f.admin.takeQueries(); len(queries) != 0 {
		t.Fatalf("%d queries after the context ended, want none", len(queries))
	}
}

func TestSessionPollingRewritesWhatTheStoreLost(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addUser(logout("u-1", "alice", aliceSession, f.ago(3*time.Second)))
	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}

	f.store.mu.Lock()
	f.store.inner = authn.NewMemoryStore(f.clock)
	f.store.mu.Unlock()
	if kind := f.revoked(t, authn.SessionRef{ID: aliceSession}); kind != "" {
		t.Fatalf("after the loss the session = %q, want gone", kind)
	}
	f.clock.Advance(5 * time.Second)
	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}

	if kind := f.revoked(t, authn.SessionRef{ID: aliceSession}); kind != authn.RevocationSession {
		t.Fatalf("the next cycle left the session = %q, want it written again", kind)
	}
	if got := f.applied(t); got["applied"] != 1 || len(got) != 1 {
		t.Fatalf("events = %v, want the rewrite not counted again", got)
	}
}

func TestSessionPollingCountsEachEventOnceAndForgetsWhatLeftTheWindow(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addUser(
		logout("u-1", "alice", "sid-1", f.ago(4*time.Second)),
		logout("u-2", "bob", "sid-2", f.ago(3*time.Second)),
	)
	f.admin.addAdmin(adminEvent{ID: "a-1", Time: f.ago(2 * time.Second), OperationType: "ACTION", ResourceType: "USER",
		ResourcePath: "users/carol/logout"})

	for range 3 {
		if err := f.polling.Poll(t.Context()); err != nil {
			t.Fatalf("Poll() err = %v", err)
		}
		f.clock.Advance(5 * time.Second)
	}
	if got := f.applied(t); got["applied"] != 3 || len(got) != 1 {
		t.Fatalf("events after 3 cycles = %v, want 3 applied, each event once", got)
	}
	if calls := len(f.store.calls()); calls != 9 {
		t.Fatalf("%d Revoke calls, want 9: every cycle reapplies the whole window", calls)
	}
	if got := f.polling.Tracked(); got != 3 {
		t.Fatalf("Tracked() = %d, want the 3 ids of the window", got)
	}

	f.clock.Advance(6 * time.Minute)
	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}
	if got := f.polling.Tracked(); got != 0 {
		t.Fatalf("Tracked() = %d after the window passed, want 0: the map is bounded by the window", got)
	}
}

func TestSessionPollingRetriesAFailedStoreOnTheNextCycle(t *testing.T) {
	f := newPollingFixture(t)
	f.admin.addUser(logout("u-1", "alice", aliceSession, f.ago(3*time.Second)))
	f.store.mu.Lock()
	f.store.fail = errors.New("redis: refused for dmpf:revocation:sid:" + aliceSession)
	f.store.mu.Unlock()

	if err := f.polling.Poll(t.Context()); err == nil || strings.Contains(err.Error(), aliceSession) {
		t.Fatalf("Poll() err = %v, want a failure that does not echo the key", err)
	}
	logged := f.sink.all()
	if len(logged) != 1 || logged[0].Body().AsString() != "auth: revocation store unavailable" {
		t.Fatalf("records = %d, want one store unavailable", len(logged))
	}
	if got := f.applied(t); len(got) != 0 {
		t.Fatalf("events = %v, want nothing counted while the store is down", got)
	}

	f.store.mu.Lock()
	f.store.fail = nil
	f.store.mu.Unlock()
	f.clock.Advance(5 * time.Second)
	if err := f.polling.Poll(t.Context()); err != nil {
		t.Fatalf("Poll() err = %v", err)
	}
	if got := f.applied(t); got["applied"] != 1 {
		t.Fatalf("events = %v, want the event counted once the store answered", got)
	}
	requireNothingPersonal(t, f.raw, pollingToken, "")
	for _, record := range f.raw.all() {
		for _, value := range attributesOf(record) {
			if strings.Contains(value, "203.0.113.7") || strings.Contains(value, pollingClientSecret) {
				t.Errorf("record %q carries %q", record.Body().AsString(), value)
			}
		}
	}
}

func TestSessionPollingRunsEachIntervalUntilTheContextEnds(t *testing.T) {
	f := newPollingFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		f.polling.Run(ctx)
		close(done)
	}()

	for cycle := range 3 {
		for range 2 {
			select {
			case <-f.admin.served:
			case <-time.After(5 * time.Second):
				t.Fatalf("cycle %d: the polling did not read both sources", cycle)
			}
		}
		for f.clock.Pending() == 0 {
			time.Sleep(time.Millisecond)
		}
		f.clock.Advance(5 * time.Second)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after the context ended")
	}
}

func TestNewSessionPollingRefusesAStartItCouldNotHonour(t *testing.T) {
	admin := newKeycloakAdmin(t)
	feedFor := func(issuer string) *authn.Feed {
		feed, err := authn.NewFeed(authn.NewMemoryStore(nil), authn.FeedConfig{Issuer: issuer, TTL: time.Minute})
		if err != nil {
			t.Fatalf("NewFeed() err = %v", err)
		}
		return feed
	}
	valid := authn.SessionPollingConfig{ClientID: pollingClientID, ClientSecret: pollingClientSecret}

	tests := []struct {
		name string
		feed *authn.Feed
		cfg  authn.SessionPollingConfig
		want error
	}{
		{name: "no feed", cfg: valid, want: authn.ErrNoRevocationFeed},
		{name: "no client id", feed: feedFor(admin.issuer()), cfg: authn.SessionPollingConfig{ClientSecret: "x"},
			want: authn.ErrMissingVariable},
		{name: "no client secret", feed: feedFor(admin.issuer()), cfg: authn.SessionPollingConfig{ClientID: "x"},
			want: authn.ErrMissingVariable},
		{name: "not a Keycloak realm issuer", feed: feedFor("https://idp.example.com/oauth2"), cfg: valid,
			want: authn.ErrNotAKeycloakRealm},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := authn.NewSessionPolling(tt.feed, tt.cfg); !errors.Is(err, tt.want) {
				t.Fatalf("NewSessionPolling() err = %v, want %v", err, tt.want)
			}
		})
	}
}
