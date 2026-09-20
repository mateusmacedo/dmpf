package http_test

import (
	"context"
	"net/http"
	"testing"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type stubAuthenticator struct {
	identity ports.Identity
	err      error
}

func (s stubAuthenticator) Authenticate(context.Context, ports.Credential) (ports.Identity, error) {
	return s.identity, s.err
}

func tenantOf(name string) *ports.TenantID {
	id := ports.TenantID(name)
	return &id
}

func resolvedIdentity() ports.Identity {
	return ports.Identity{
		Subject:     "alice",
		Tenant:      tenantOf("acme"),
		Permissions: []ports.Permission{"orders:read"},
	}
}

func presented() ports.Credential {
	return ports.Credential{Scheme: "Bearer", Value: "t-000001"}
}

func TestResolveIdentityLetsAVerifiedSubjectThrough(t *testing.T) {
	route := route(http.MethodGet)

	got, status, code := provider.ResolveIdentity(t.Context(), stubAuthenticator{identity: resolvedIdentity()}, route, presented())

	if status != 0 || code != "" {
		t.Fatalf("status = %d, code = %q, want the request to proceed", status, code)
	}
	if got.Subject == nil || *got.Subject != "alice" {
		t.Fatalf("Subject = %v, want alice", got.Subject)
	}
	if got.Tenant == nil || *got.Tenant != "acme" {
		t.Fatalf("Tenant = %v, want acme", got.Tenant)
	}
}

func TestResolveIdentityRefusesWithoutACredential(t *testing.T) {
	route := route(http.MethodGet)

	_, status, code := provider.ResolveIdentity(t.Context(), stubAuthenticator{}, route, ports.Credential{})

	if status != http.StatusUnauthorized || code != "unauthenticated" {
		t.Fatalf("status = %d, code = %q, want 401 unauthenticated (IDN-15)", status, code)
	}
}

func TestResolveIdentitySeparatesTheTwoRefusals(t *testing.T) {
	tests := []struct {
		name       string
		identity   ports.Identity
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "verification failed",
			err:        ports.ErrCredentialRejected,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name:       "verified but no subject",
			identity:   ports.Identity{Tenant: tenantOf("acme")},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name:       "verified subject without the tenant the route demands",
			identity:   ports.Identity{Subject: "alice", Permissions: []ports.Permission{}},
			wantStatus: http.StatusForbidden,
			wantCode:   "tenant-unresolved",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, status, code := provider.ResolveIdentity(t.Context(), stubAuthenticator{identity: tt.identity, err: tt.err}, route(http.MethodGet), presented())

			if status != tt.wantStatus || code != tt.wantCode {
				t.Fatalf("status = %d, code = %q, want %d %q (IDN-06)", status, code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestResolveIdentityLetsAPlatformRouteThroughWithoutACredential(t *testing.T) {
	platform := route(http.MethodGet)
	platform.Requires = provider.RequireNeither
	platform.PlatformReach = provider.ReachAllTenants

	got, status, _ := provider.ResolveIdentity(t.Context(), stubAuthenticator{}, platform, ports.Credential{})

	if status != 0 {
		t.Fatalf("status = %d, want a platform chain without a subject to be legitimate (IDN-18)", status)
	}
	if got.Subject != nil || got.Tenant != nil {
		t.Fatalf("got = %+v, want absence to stay absence (IDN-20)", got)
	}
}

func TestExecutionContextCrossesToTheHandlerAndNowhereElse(t *testing.T) {
	spec := ports.ExecutionContextSpec{
		RequestID:     "req-1",
		CorrelationID: "cor-1",
		TraceContext:  "trace-1",
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "en",
	}
	execution, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("NewExecutionContext() err = %v", err)
	}

	if _, ok := provider.ExecutionContextFrom(context.Background()); ok {
		t.Fatal("a context the edge never mounted must report absence")
	}

	carried, ok := provider.ExecutionContextFrom(provider.WithExecutionContext(context.Background(), execution))
	if !ok {
		t.Fatal("ExecutionContextFrom() ok = false after WithExecutionContext")
	}
	if carried.RequestID() != "req-1" {
		t.Fatalf("RequestID() = %q, want req-1", carried.RequestID())
	}
}
