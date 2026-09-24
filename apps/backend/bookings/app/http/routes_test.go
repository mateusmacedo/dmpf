package httpedge_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	httpedge "github.com/mateusmacedo/dmpf/apps/backend/bookings/app/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var testBudget = deadline.Budget{
	Dependency:        "postgres",
	Method:            "route",
	Limit:             2 * time.Second,
	Slack:             200 * time.Millisecond,
	EstimatedDuration: 200 * time.Millisecond,
}

func TestEveryRouteCarriesTheBudgetTheEdgeDeclares(t *testing.T) {
	for _, route := range httpedge.Routes(testBudget) {
		if route.Budget != testBudget {
			t.Fatalf("%s carries %+v, want the budget the edge declared: the deadline is the edge policy's, not the caller's", route.Name, route.Budget)
		}
		if err := route.ValidateEdge(); err != nil {
			t.Fatalf("%s: Validate() = %v", route.Name, err)
		}
	}
}

func TestARouteWithoutABudgetIsRefused(t *testing.T) {
	for _, route := range httpedge.Routes(deadline.Budget{}) {
		if err := route.Validate(); err == nil {
			t.Fatalf("%s validated with no budget, so nothing would stop a route from serving without a governed deadline", route.Name)
		}
	}
}

func TestEveryRouteNamesTheContractItServes(t *testing.T) {
	for _, route := range httpedge.Routes(testBudget) {
		if route.Name == "" {
			t.Fatalf("route %+v has no name", route)
		}
		if !strings.HasPrefix(route.ContractRef, "contracts/openapi/bookings/v1/openapi.yaml#/paths/") {
			t.Fatalf("%s points at %q, want the published OpenAPI of this context (RST-04)", route.Name, route.ContractRef)
		}
	}
}

func TestEveryWriteRouteDeclaresItsIdempotencyKey(t *testing.T) {
	for _, route := range httpedge.Routes(testBudget) {
		if route.Method != http.MethodPost {
			continue
		}
		if route.IdempotencyKey != httpedge.IdempotencyHeader {
			t.Fatalf("%s is a POST without a declared idempotency key (RST-02)", route.Name)
		}
	}
}

func TestTheReadRoutesCarryNoIdempotencyKey(t *testing.T) {
	for _, route := range httpedge.Routes(testBudget) {
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
	for _, route := range httpedge.Routes(testBudget) {
		key := route.Method + " " + route.Path
		if before, clash := seen[key]; clash {
			t.Fatalf("%s and %s both answer %q; the mux would refuse the second", before, route.Name, key)
		}
		seen[key] = route.Name
	}
}
