package http_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func route(method string) provider.Route {
	return provider.Route{
		Name:        "placeOrder",
		Method:      method,
		Path:        "/v1/orders",
		ContractRef: "openapi/orders/v1.yaml#/paths/~1v1~1orders/post",
		Budget: deadline.Budget{
			Dependency:        "orders",
			Method:            "placeOrder",
			Limit:             time.Second,
			Slack:             50 * time.Millisecond,
			EstimatedDuration: 100 * time.Millisecond,
		},
		RetryableStatus: []int{http.StatusServiceUnavailable},
		Permission:      "orders:read",
	}
}

func TestRouteValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*provider.Route)
		want   error
	}{
		"complete":             {func(*provider.Route) {}, nil},
		"no name":              {func(r *provider.Route) { r.Name = "" }, provider.ErrIncompleteRoute},
		"no path":              {func(r *provider.Route) { r.Path = "" }, provider.ErrIncompleteRoute},
		"no contract (RST-04)": {func(r *provider.Route) { r.ContractRef = "" }, provider.ErrContractRequired},
		"unknown method":       {func(r *provider.Route) { r.Method = "PURGE" }, provider.ErrMethodNotAllowed},
		"empty method":         {func(r *provider.Route) { r.Method = "" }, provider.ErrMethodNotAllowed},
		"budget without slack": {func(r *provider.Route) { r.Budget.Slack = 0 }, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := route(http.MethodPost)
			tc.mutate(&r)
			err := r.Validate()
			switch {
			case name == "budget without slack":
				if err == nil {
					t.Fatal("Validate() = nil, want the budget's refusal (GRP-17)")
				}
			case tc.want == nil && err != nil:
				t.Fatalf("Validate() = %v, want nil", err)
			case tc.want != nil && !errors.Is(err, tc.want):
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRouteIdempotent(t *testing.T) {
	cases := map[string]struct {
		method string
		key    string
		want   bool
	}{
		"GET":                   {http.MethodGet, "", true},
		"HEAD":                  {http.MethodHead, "", true},
		"PUT":                   {http.MethodPut, "", true},
		"DELETE":                {http.MethodDelete, "", true},
		"POST without key":      {http.MethodPost, "", false},
		"POST with key":         {http.MethodPost, "Idempotency-Key", true},
		"PATCH never":           {http.MethodPatch, "", false},
		"PATCH even with a key": {http.MethodPatch, "Idempotency-Key", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := route(tc.method)
			r.IdempotencyKey = tc.key
			if got := r.Idempotent(); got != tc.want {
				t.Fatalf("Idempotent() = %v, want %v (RST-02)", got, tc.want)
			}
		})
	}
}

func TestRouteWithoutDeclarationRequiresBoth(t *testing.T) {
	undeclared := route(http.MethodPost)

	if undeclared.Requires != provider.RequireSubjectAndTenant {
		t.Fatalf("the zero Requirement must be RequireSubjectAndTenant, got %v (IDN-17)", undeclared.Requires)
	}
	if !undeclared.RequiresSubject() || !undeclared.RequiresTenant() {
		t.Fatal("a route that declares nothing must demand subject and tenant (IDN-17)")
	}
}

func TestRouteRequirementPredicates(t *testing.T) {
	cases := map[string]struct {
		requires    provider.Requirement
		wantSubject bool
		wantTenant  bool
	}{
		"both by omission": {provider.RequireSubjectAndTenant, true, true},
		"subject only":     {provider.RequireSubject, true, false},
		"tenant only":      {provider.RequireTenant, false, true},
		"platform":         {provider.RequireNeither, false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := route(http.MethodGet)
			r.Requires = tc.requires

			if got := r.RequiresSubject(); got != tc.wantSubject {
				t.Fatalf("RequiresSubject() = %v, want %v (IDN-16)", got, tc.wantSubject)
			}
			if got := r.RequiresTenant(); got != tc.wantTenant {
				t.Fatalf("RequiresTenant() = %v, want %v (IDN-16)", got, tc.wantTenant)
			}
		})
	}
}

func TestRouteValidateRefusesPlatformRouteWithUndeclaredReach(t *testing.T) {
	platform := route(http.MethodGet)
	platform.Requires = provider.RequireNeither

	if err := platform.Validate(); !errors.Is(err, provider.ErrPlatformReachRequired) {
		t.Fatalf("Validate() err = %v, want %v (IDN-19)", err, provider.ErrPlatformReachRequired)
	}

	platform.PlatformReach = provider.ReachAllTenants
	if err := platform.Validate(); err != nil {
		t.Fatalf("a platform route that declares its reach is valid, got err = %v", err)
	}
}

func TestRouteValidateRefusesReachDeclaredByNonPlatformRoute(t *testing.T) {
	scoped := route(http.MethodGet)
	scoped.PlatformReach = provider.ReachAllTenants

	if err := scoped.Validate(); !errors.Is(err, provider.ErrPlatformReachRequired) {
		t.Fatalf("Validate() err = %v, want %v: reach belongs to the platform route alone (IDN-19)", err, provider.ErrPlatformReachRequired)
	}
}

func TestRouteRequirementUnknownValueIsRefused(t *testing.T) {
	unknown := route(http.MethodGet)
	unknown.Requires = provider.Requirement(99)

	if err := unknown.Validate(); !errors.Is(err, provider.ErrRequirementUnknown) {
		t.Fatalf("Validate() err = %v, want %v", err, provider.ErrRequirementUnknown)
	}
}

// An inbound edge refuses to start with a route that demands a subject and
// declares no permission, instead of denying every request at runtime (IDN-17).
func TestValidateEdgeRefusesASubjectRouteWithoutPermission(t *testing.T) {
	r := route(http.MethodPost)
	r.Permission = ""
	if err := r.ValidateEdge(); !errors.Is(err, provider.ErrPermissionRequired) {
		t.Fatalf("ValidateEdge() = %v, want ErrPermissionRequired", err)
	}
	r.Permission = "orders:write"
	if err := r.ValidateEdge(); err != nil {
		t.Fatalf("ValidateEdge() = %v, want nil", err)
	}
}
