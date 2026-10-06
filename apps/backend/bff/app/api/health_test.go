package api_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
)

func TestLivenessAnswersWithoutCredential(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.doAdmin(t, http.MethodGet, api.LivenessPath, "Authorization", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("%s = %d, want 204", api.LivenessPath, rec.Code)
	}
}

func TestReadinessFollowsTheContexts(t *testing.T) {
	down := errors.New("rpc: contexts not ready: orders")
	for _, tc := range []struct {
		name  string
		ready func(context.Context) error
		code  int
	}{
		{"contexts serving", func(context.Context) error { return nil }, http.StatusNoContent},
		{"a context down", func(context.Context) error { return down }, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{}, withReady(tc.ready))

			rec := f.doAdmin(t, http.MethodGet, api.ReadinessPath, "Authorization", "")

			if rec.Code != tc.code {
				t.Fatalf("%s = %d %q, want %d", api.ReadinessPath, rec.Code, rec.Body.String(), tc.code)
			}
			if tc.code != http.StatusNoContent && strings.Contains(rec.Body.String(), "orders") {
				t.Fatalf("body = %q, want no context named to the caller", rec.Body.String())
			}
			if tc.code != http.StatusNoContent && !strings.Contains(f.logs.String(), `"msg":"not ready"`) {
				t.Fatalf("log = %q, want the readiness failure recorded", f.logs.String())
			}
		})
	}
}

func TestReadinessIsNotServedWithoutAProbe(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	if rec := f.doAdmin(t, http.MethodGet, api.ReadinessPath); rec.Code != http.StatusNotFound {
		t.Fatalf("%s without probe = %d, want 404", api.ReadinessPath, rec.Code)
	}
}

func TestReadinessProbesWithinASecondShareOneCheck(t *testing.T) {
	var checks atomic.Int32
	f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error {
		checks.Add(1)
		return nil
	}))

	for range 3 {
		f.doAdmin(t, http.MethodGet, api.ReadinessPath, "Authorization", "")
	}

	if got := checks.Load(); got != 1 {
		t.Fatalf("checks = %d, want 1 for probes within the cache window", got)
	}
}

func TestReadinessRefusesWhileTheEdgeDrains(t *testing.T) {
	var draining atomic.Bool
	f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error { return nil }), withDraining(draining.Load))

	if rec := f.doAdmin(t, http.MethodGet, api.ReadinessPath, "Authorization", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("%s = %d before the drain, want 204", api.ReadinessPath, rec.Code)
	}
	draining.Store(true)
	if rec := f.doAdmin(t, http.MethodGet, api.ReadinessPath, "Authorization", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("%s = %d while draining, want 503 despite the cached check", api.ReadinessPath, rec.Code)
	}
}

func TestReadinessLogsAConnectionErrorWithoutItsMessage(t *testing.T) {
	refused := errors.New(`failed to connect to user=bookings database=bookings: 10.0.0.7:5432 (pg.internal): dial error`)
	f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error { return refused }))

	f.doAdmin(t, http.MethodGet, api.ReadinessPath, "Authorization", "")

	logs := f.logs.String()
	for _, leaked := range []string{"10.0.0.7", "pg.internal", "user=bookings", "database=bookings"} {
		if strings.Contains(logs, leaked) {
			t.Errorf("log = %q, want no %q from the error message (RF-A6, DAT-23)", logs, leaked)
		}
	}
	if !strings.Contains(logs, `"error.type":`) {
		t.Errorf("log = %q, want the error reduced to error.type (RF-A3)", logs)
	}
}

func TestTheHealthRoutesOpenNoServerSpanNorPoint(t *testing.T) {
	up := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("rpc: contexts not ready: orders") }
	for _, c := range []struct {
		name         string
		method, path string
		ready        func(context.Context) error
		headers      []string
		status       int
	}{
		{"liveness", http.MethodGet, api.LivenessPath, up, nil, http.StatusNoContent},
		{"liveness by HEAD", http.MethodHead, api.LivenessPath, up, nil, http.StatusNoContent},
		{"readiness", http.MethodGet, api.ReadinessPath, up, nil, http.StatusNoContent},
		{"readiness refused", http.MethodGet, api.ReadinessPath, down, nil, http.StatusServiceUnavailable},
		{"readiness from an allowed origin", http.MethodGet, api.ReadinessPath, up, []string{"Origin", "http://localhost:8082"}, http.StatusNoContent},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{}, withReady(c.ready), withCORS("http://localhost:8082"))

			if rec := f.doAdmin(t, c.method, c.path, c.headers...); rec.Code != c.status {
				t.Fatalf("%s %s = %d, want %d", c.method, c.path, rec.Code, c.status)
			}

			var names []string
			for _, span := range f.spans.GetSpans() {
				names = append(names, span.SpanKind.String()+" "+span.Name)
			}
			if len(names) != 0 {
				t.Fatalf("spans = %v, want none for a health probe (RF-B2)", names)
			}
			if count := durationCount(t, f); count != 0 {
				t.Fatalf("http.server.request.duration counted %d requests, want none for a health probe (RF-B2)", count)
			}
		})
	}
}

func TestTheReadinessPathWithoutAProbeKeepsItsServerSpanAndPoint(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	if rec := f.doAdmin(t, http.MethodGet, api.ReadinessPath); rec.Code != http.StatusNotFound {
		t.Fatalf("%s without probe = %d, want 404", api.ReadinessPath, rec.Code)
	}

	serverSpan(t, f, http.MethodGet)
	if count := durationCount(t, f); count != 1 {
		t.Fatalf("http.server.request.duration counted %d requests, want 1 for a 404 of the mux (RF-B2)", count)
	}
}

func TestThePublicPortServesNoHealthRoute(t *testing.T) {
	for _, path := range []string{api.LivenessPath, api.ReadinessPath} {
		t.Run(path, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error { return nil }))

			if rec := f.do(t, http.MethodGet, path, nil, "Authorization", ""); rec.Code != http.StatusNotFound {
				t.Fatalf("public %s = %d, want 404: the probes answer only on the administration port", path, rec.Code)
			}

			serverSpan(t, f, http.MethodGet)
			if count := durationCount(t, f); count != 1 {
				t.Fatalf("http.server.request.duration counted %d requests, want 1 for a 404 of the mux (RF-B2)", count)
			}
			record := onlyAccessLog(t, f)
			if _, routed := record["http.route"]; routed || record[severityKey] != "INFO" ||
				record["http.response.status_code"] != int64(http.StatusNotFound) || record["dmpf.outcome_category"] != "rejected" {
				t.Fatalf("access log = %v, want the 404 of the mux at INFO, rejected, without http.route (RF-A5)", record)
			}
		})
	}
}

func healthStart() *obsclock.Fake {
	return obsclock.NewFake(time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC))
}

func TestASuccessfulHealthProbeIsLoggedAtMostOnceEveryFiveMinutesPerRoute(t *testing.T) {
	clock := healthStart()
	f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error { return nil }), withClock(clock))
	probe := func() {
		for range 3 {
			for _, path := range []string{api.LivenessPath, api.ReadinessPath} {
				if rec := f.doAdmin(t, http.MethodGet, path, "Authorization", ""); rec.Code != http.StatusNoContent {
					t.Fatalf("%s = %d, want 204", path, rec.Code)
				}
			}
		}
	}
	perRoute := func() map[string]int {
		logged := map[string]int{}
		for _, record := range accessLogs(t, f) {
			route, _ := record["http.route"].(string)
			for key, value := range map[string]any{
				severityKey:                 "INFO",
				"http.request.method":       http.MethodGet,
				"http.response.status_code": int64(http.StatusNoContent),
				"dmpf.outcome_category":     "accepted",
			} {
				if record[key] != value {
					t.Fatalf("access log %s = %v, want %v (record %v)", key, record[key], value, record)
				}
			}
			logged[route]++
		}
		return logged
	}

	probe()
	if got := perRoute(); got[api.LivenessPath] != 1 || got[api.ReadinessPath] != 1 || len(got) != 2 {
		t.Fatalf("access logs per route = %v, want one for each health route", got)
	}

	clock.Advance(5*time.Minute - time.Nanosecond)
	probe()
	if got := perRoute(); got[api.LivenessPath] != 1 || got[api.ReadinessPath] != 1 {
		t.Fatalf("access logs per route = %v, want none more within five minutes", got)
	}

	clock.Advance(time.Nanosecond)
	probe()
	if got := perRoute(); got[api.LivenessPath] != 2 || got[api.ReadinessPath] != 2 {
		t.Fatalf("access logs per route = %v, want one more each once five minutes passed", got)
	}

	other := newFixture(t, &fakeContexts{}, withClock(clock))
	other.doAdmin(t, http.MethodGet, api.LivenessPath, "Authorization", "")
	if got := len(accessLogs(t, other)); got != 1 {
		t.Fatalf("access logs of another process = %d, want its own window to log the first success", got)
	}
}

func TestAFailedHealthProbeIsAlwaysLogged(t *testing.T) {
	var draining atomic.Bool
	f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error { return nil }), withDraining(draining.Load), withClock(healthStart()))

	for _, drain := range []bool{false, true, true, false} {
		draining.Store(drain)
		f.doAdmin(t, http.MethodGet, api.ReadinessPath, "Authorization", "")
	}

	perStatus := map[int64]int{}
	for _, record := range accessLogs(t, f) {
		status, _ := record["http.response.status_code"].(int64)
		perStatus[status]++
		if status == http.StatusServiceUnavailable && (record[severityKey] != "ERROR" || record["dmpf.outcome_category"] != "failed" || record["http.route"] != api.ReadinessPath) {
			t.Fatalf("access log = %v, want the refusal at ERROR, failed, on %s", record, api.ReadinessPath)
		}
	}
	if perStatus[http.StatusServiceUnavailable] != 2 || perStatus[http.StatusNoContent] != 1 || len(perStatus) != 2 {
		t.Fatalf("access logs per status = %v, want every 503 and only the first 204 (LOG-10)", perStatus)
	}
}
