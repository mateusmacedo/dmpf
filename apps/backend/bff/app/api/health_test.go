package api_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
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
			if tc.code != http.StatusNoContent && !strings.Contains(rec.Body.String(), "orders") {
				t.Fatalf("body = %q, want the context that is down", rec.Body.String())
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
