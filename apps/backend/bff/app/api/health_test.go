package api_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
)

func TestLivenessAnswersWithoutCredential(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodGet, api.LivenessPath, nil, "Authorization", "")

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

			rec := f.do(t, http.MethodGet, api.ReadinessPath, nil, "Authorization", "")

			if rec.Code != tc.code {
				t.Fatalf("%s = %d %q, want %d", api.ReadinessPath, rec.Code, rec.Body.String(), tc.code)
			}
			if tc.code != http.StatusNoContent && strings.Contains(rec.Body.String(), "orders") {
				t.Fatalf("body = %q, want no context named to the caller", rec.Body.String())
			}
			if tc.code != http.StatusNoContent && !strings.Contains(f.logs.String(), "orders") {
				t.Fatalf("log = %q, want the context that is down", f.logs.String())
			}
		})
	}
}

func TestReadinessIsNotServedWithoutAProbe(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	if rec := f.do(t, http.MethodGet, api.ReadinessPath, nil); rec.Code != http.StatusNotFound {
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
		f.do(t, http.MethodGet, api.ReadinessPath, nil, "Authorization", "")
	}

	if got := checks.Load(); got != 1 {
		t.Fatalf("checks = %d, want 1 for probes within the cache window", got)
	}
}

func TestReadinessRefusesWhileTheEdgeDrains(t *testing.T) {
	var draining atomic.Bool
	f := newFixture(t, &fakeContexts{}, withReady(func(context.Context) error { return nil }), withDraining(draining.Load))

	if rec := f.do(t, http.MethodGet, api.ReadinessPath, nil, "Authorization", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("%s = %d before the drain, want 204", api.ReadinessPath, rec.Code)
	}
	draining.Store(true)
	if rec := f.do(t, http.MethodGet, api.ReadinessPath, nil, "Authorization", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("%s = %d while draining, want 503 despite the cached check", api.ReadinessPath, rec.Code)
	}
}
