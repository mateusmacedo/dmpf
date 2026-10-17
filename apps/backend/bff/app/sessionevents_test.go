package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

type fakeRealm struct {
	issuer string
	polled atomic.Int64
}

func newFakeRealm(t *testing.T) *fakeRealm {
	t.Helper()
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)
	realm := &fakeRealm{issuer: server.URL + "/realms/dmpf"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /realms/dmpf/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": realm.issuer, "jwks_uri": realm.issuer + "/protocol/openid-connect/certs",
			"authorization_endpoint": realm.issuer + "/auth", "token_endpoint": realm.issuer + "/protocol/openid-connect/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("POST /realms/dmpf/protocol/openid-connect/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "polling", "token_type": "Bearer", "expires_in": 300})
	})
	events := func(w http.ResponseWriter, _ *http.Request) {
		realm.polled.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}
	mux.HandleFunc("GET /admin/realms/dmpf/admin-events", events)
	mux.HandleFunc("GET /admin/realms/dmpf/events", events)
	server.Config.Handler = mux
	return realm
}

func startSessionEvents(t *testing.T, env ...string) edge {
	t.Helper()
	base := append(slices.Clone(sessionEventsEnv), "SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0",
		"REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0")
	cfg, err := app.SessionEventsFromEnv(lookup(append(base, env...)...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}
	cfg.Polling.Interval = 20 * time.Millisecond
	e := edge{logs: &obstest.MemoryLogs{}, spans: tracetest.NewInMemoryExporter(), metrics: sdkmetric.NewManualReader(), done: make(chan error, 1)}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "bff", ServiceVersion: "test", ServiceInstanceID: "bff-session-events-1", Role: "session-events"},
		TraceExporter: e.spans,
		MetricReader:  e.metrics,
	}
	e.provider = otelboot.NewLoggerProvider(config, e.logs)
	config.LoggerProvider = e.provider
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	if e.runtime, err = otelboot.Start(ctx, config); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = e.runtime.Shutdown(context.Background())
	})

	go func() { e.done <- app.RunSessionEventsWith(ctx, cfg, e.runtime) }()
	e.addr = listeningAddr(t, e, "session events listening")
	e.admin = listeningAddr(t, e, "admin listening")
	return e
}

func post(t *testing.T, addr, path string, form url.Values) int {
	t.Helper()
	res, err := http.Post("http://"+addr+path, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST %s = %v", path, err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func TestTheSessionEventsRoleServesTheReceiverAndPolls(t *testing.T) {
	realm := newFakeRealm(t)
	e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer)

	if status := post(t, e.addr, "/backchannel-logout", url.Values{"logout_token": {"not-a-jwt"}}); status != http.StatusBadRequest {
		t.Fatalf("POST /backchannel-logout with an unverifiable token = %d, want 400", status)
	}
	if status := e.request(t, http.MethodGet, "/backchannel-logout"); status != http.StatusMethodNotAllowed {
		t.Fatalf("GET /backchannel-logout = %d, want 405", status)
	}
	if status := e.request(t, http.MethodGet, "/readyz"); status != http.StatusNotFound {
		t.Fatalf("GET /readyz on the receiver port = %d, want 404: health lives on the administration port", status)
	}
	if status := e.probe(t, http.MethodGet, "/livez"); status != http.StatusNoContent {
		t.Fatalf("GET /livez = %d, want 204", status)
	}
	if status := e.probe(t, http.MethodGet, "/readyz"); status/100 != 2 {
		t.Fatalf("GET /readyz = %d, want 2xx", status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for realm.polled.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := realm.polled.Load(); got < 4 {
		t.Fatalf("the realm was polled %d times, want both sources read on every cycle", got)
	}
	e.stop(t)
}

func TestTheSessionEventsRoleStopsPollingOnShutdown(t *testing.T) {
	realm := newFakeRealm(t)
	e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer)
	e.stop(t)

	settled := realm.polled.Load()
	time.Sleep(100 * time.Millisecond)
	if got := realm.polled.Load(); got != settled {
		t.Fatalf("the realm was polled %d times after RunSessionEventsWith returned, want none", got-settled)
	}
	if _, err := http.Get("http://" + e.admin + "/livez"); err == nil {
		t.Fatal("the administration port still answers after the shutdown")
	}
}

func TestTheSessionEventsRoleRefusesToStartWithoutDiscovery(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	issuer := gone.URL + "/realms/dmpf"
	gone.Close()
	cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "OIDC_ISSUER", issuer,
		"SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0", "OIDC_DISCOVERY_TIMEOUT_SECONDS", "1")...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}
	logs := &obstest.MemoryLogs{}
	config := otelboot.Config{Propagator: propagation.TraceContext{}, TraceExporter: tracetest.NewInMemoryExporter(),
		Resource: otelboot.Resource{ServiceName: "bff", ServiceVersion: "test", ServiceInstanceID: "bff-session-events-1"}}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	if err := app.RunSessionEventsWith(context.Background(), cfg, runtime); err == nil {
		t.Fatal("RunSessionEventsWith() = nil, want the start refused without the issuer's discovery")
	}
}
