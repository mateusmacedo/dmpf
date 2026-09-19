package httpedge_test

import (
	"net/http"
	"strings"
	"testing"

	httpedge "github.com/mateusmacedo/dmpf/apps/backend/bookings/app/http"
)

func TestEveryRouteNamesTheContractItServes(t *testing.T) {
	for _, route := range httpedge.Routes() {
		if route.Name == "" {
			t.Fatalf("route %+v has no name", route)
		}
		if !strings.HasPrefix(route.ContractRef, "contracts/openapi/bookings/v1/openapi.yaml#/paths/") {
			t.Fatalf("%s points at %q, want the published OpenAPI of this context (RST-04)", route.Name, route.ContractRef)
		}
	}
}

func TestEveryWriteRouteDeclaresItsIdempotencyKey(t *testing.T) {
	for _, route := range httpedge.Routes() {
		if route.Method != http.MethodPost {
			continue
		}
		if route.IdempotencyKey != httpedge.IdempotencyHeader {
			t.Fatalf("%s is a POST without a declared idempotency key (RST-02)", route.Name)
		}
	}
}

func TestTheReadRoutesCarryNoIdempotencyKey(t *testing.T) {
	for _, route := range httpedge.Routes() {
		if route.Method != http.MethodGet {
			continue
		}
		if route.IdempotencyKey != "" {
			t.Fatalf("%s is a GET declaring an idempotency key; only a creating POST needs one", route.Name)
		}
	}
}

func TestNoTwoRoutesShareAMethodAndPath(t *testing.T) {
	seen := map[string]string{}
	for _, route := range httpedge.Routes() {
		key := route.Method + " " + route.Path
		if before, clash := seen[key]; clash {
			t.Fatalf("%s and %s both answer %q; the mux would refuse the second", before, route.Name, key)
		}
		seen[key] = route.Name
	}
}
