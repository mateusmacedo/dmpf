package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const platformPermission ports.Permission = "platform.read"

func reachThroughTheEdge(t *testing.T, route kernelhttp.Route, credential string) (ports.PlatformReach, bool, int) {
	t.Helper()
	var (
		reach    ports.PlatformReach
		declared bool
		reached  bool
	)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reach, declared = ports.PlatformReachFrom(r.Context())
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	handler := withExecutionContext(slog.New(slog.NewTextHandler(io.Discard, nil)), authn.DevAuthenticator{}, route, next)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/platform/rows", nil)
	if credential != "" {
		r.Header.Set("Authorization", credential)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	if !reached {
		t.Fatalf("status = %d, the request never reached the handler", rec.Code)
	}
	return reach, declared, rec.Code
}

func platformRoute(requires kernelhttp.Requirement, reach kernelhttp.PlatformReach) kernelhttp.Route {
	return kernelhttp.Route{Name: "listRows", Method: http.MethodGet, Path: "/platform/rows", Requires: requires, PlatformReach: reach, Permission: platformPermission}
}

func TestTheEdgeDepositsTheReachOnARouteThatVerifiedItsSubject(t *testing.T) {
	route := platformRoute(kernelhttp.RequireSubject, kernelhttp.ReachAllTenants)

	reach, declared, _ := reachThroughTheEdge(t, route, `Bearer {"sub":"operator","permissions":["platform.read"]}`)

	if !declared || reach != ports.ReachAllTenants {
		t.Fatalf("reach = %v, %v, want ReachAllTenants", reach, declared)
	}
}

func TestTheEdgeNeverDepositsTheReachWithoutAVerifiedSubject(t *testing.T) {
	route := platformRoute(kernelhttp.RequireNeither, kernelhttp.ReachAllTenants)

	reach, declared, status := reachThroughTheEdge(t, route, "")

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: a route that requires neither admits the request", status)
	}
	if declared {
		t.Fatalf("reach = %v, want none: nobody verified who called", reach)
	}
}

func TestTheEdgeNeverDepositsTheReachOnARouteThatDeclaresNone(t *testing.T) {
	route := platformRoute(kernelhttp.RequireSubjectAndTenant, kernelhttp.ReachUndeclared)

	reach, declared, _ := reachThroughTheEdge(t, route, `Bearer {"sub":"operator","tenant":"acme","permissions":["platform.read"]}`)

	if declared {
		t.Fatalf("reach = %v, want none", reach)
	}
}
