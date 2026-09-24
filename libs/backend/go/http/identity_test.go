package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

// CTX-06: a field of the request asserting the subject or the tenant never
// feeds the context, and one that diverges from what verification resolved is
// an elevation attempt, refused as forbidden rather than ignored.
func TestAnAssertedIdentityThatDivergesIsRefused(t *testing.T) {
	subject, tenant := ports.SubjectID("alice"), ports.TenantID("acme")
	resolved := provider.Resolved{Subject: &subject, Tenant: &tenant}

	cases := map[string]struct {
		header, value, query string
		want                 int
	}{
		"no assertion":              {want: 0},
		"same tenant in a header":   {header: "X-Tenant-ID", value: "acme", want: 0},
		"other tenant in a header":  {header: "X-Tenant-ID", value: "globex", want: http.StatusForbidden},
		"other subject in a header": {header: "X-Subject-ID", value: "mallory", want: http.StatusForbidden},
		"other tenant in the query": {query: "tenant_id=globex", want: http.StatusForbidden},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/orders/o-1?"+c.query, nil)
			if c.header != "" {
				r.Header.Set(c.header, c.value)
			}
			status, code := provider.RefuseAssertedIdentity(r, resolved)
			if status != c.want {
				t.Fatalf("status = %d (%s), want %d", status, code, c.want)
			}
			if c.want != 0 && code != "identity-mismatch" {
				t.Fatalf("code = %q, want identity-mismatch", code)
			}
		})
	}
}

// A tenant asserted where verification resolved none still diverges: absence
// is not an empty value the request may fill (IDN-20).
func TestAnAssertedTenantOverAnAbsentOneIsRefused(t *testing.T) {
	subject := ports.SubjectID("alice")
	r := httptest.NewRequest(http.MethodGet, "/orders/o-1", nil)
	r.Header.Set("X-Tenant-ID", "acme")

	if status, _ := provider.RefuseAssertedIdentity(r, provider.Resolved{Subject: &subject}); status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
}

// IDN-16 at the edge: the subject is authorized here, against the permission
// the route declares, because CTX-12 keeps both from crossing the fan-out.
func TestResolveIdentityDeniesASubjectWithoutTheRoutesPermission(t *testing.T) {
	r := route(http.MethodPost)
	r.Permission = "orders:write"

	_, status, code := provider.ResolveIdentity(t.Context(), stubAuthenticator{identity: resolvedIdentity()}, r, presented())

	if status != http.StatusForbidden || code != "permission-denied" {
		t.Fatalf("status = %d, code = %q, want 403 permission-denied (IDN-08)", status, code)
	}
}

func TestResolveIdentityLetsTheDeclaredPermissionThrough(t *testing.T) {
	r := route(http.MethodGet)
	r.Permission = "orders:read"

	if _, status, code := provider.ResolveIdentity(t.Context(), stubAuthenticator{identity: resolvedIdentity()}, r, presented()); status != 0 {
		t.Fatalf("status = %d, code = %q, want the request to proceed", status, code)
	}
}

// IDN-17: a route demanding a subject and declaring no permission closes.
func TestResolveIdentityDeniesARouteThatDeclaresNoPermission(t *testing.T) {
	r := route(http.MethodGet)
	r.Permission = ""

	if _, status, code := provider.ResolveIdentity(t.Context(), stubAuthenticator{identity: resolvedIdentity()}, r, presented()); status != http.StatusForbidden || code != "permission-undeclared" {
		t.Fatalf("status = %d, code = %q, want 403 permission-undeclared", status, code)
	}
}

// CTX-06 reads every value a field carries: a repeated header or query key
// whose later value diverges, or an assertion present but empty, is refused
// the same way as a single divergent value.
func TestEveryAssertedValueIsChecked(t *testing.T) {
	subject, tenant := ports.SubjectID("alice"), ports.TenantID("acme")
	resolved := provider.Resolved{Subject: &subject, Tenant: &tenant}

	repeated := httptest.NewRequest(http.MethodGet, "/orders/o-1", nil)
	repeated.Header.Add("X-Tenant-ID", "acme")
	repeated.Header.Add("X-Tenant-ID", "globex")
	empty := httptest.NewRequest(http.MethodGet, "/orders/o-1", nil)
	empty.Header["X-Tenant-Id"] = []string{""}
	query := httptest.NewRequest(http.MethodGet, "/orders/o-1?tenant_id=acme&tenant_id=globex", nil)

	for name, r := range map[string]*http.Request{"repeated header": repeated, "empty assertion": empty, "repeated query": query} {
		if status, _ := provider.RefuseAssertedIdentity(r, resolved); status != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", name, status)
		}
	}
}
