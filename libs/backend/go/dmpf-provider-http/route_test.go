package dmpfhttp_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	dmpfhttp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-http"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func route(method string) dmpfhttp.Route {
	return dmpfhttp.Route{
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
	}
}

func TestRouteValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*dmpfhttp.Route)
		want   error
	}{
		"complete":             {func(*dmpfhttp.Route) {}, nil},
		"no name":              {func(r *dmpfhttp.Route) { r.Name = "" }, dmpfhttp.ErrIncompleteRoute},
		"no path":              {func(r *dmpfhttp.Route) { r.Path = "" }, dmpfhttp.ErrIncompleteRoute},
		"no contract (RST-04)": {func(r *dmpfhttp.Route) { r.ContractRef = "" }, dmpfhttp.ErrContractRequired},
		"unknown method":       {func(r *dmpfhttp.Route) { r.Method = "PURGE" }, dmpfhttp.ErrMethodNotAllowed},
		"empty method":         {func(r *dmpfhttp.Route) { r.Method = "" }, dmpfhttp.ErrMethodNotAllowed},
		"budget without slack": {func(r *dmpfhttp.Route) { r.Budget.Slack = 0 }, nil},
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
