// comment-discipline-ok-file: arquivo de declarações da borda HTTP; cada godoc é contrato de API pública com referência normativa (RST-02, RST-04, RES-16, RES-17), dentro do limite de 3 linhas.

// Package api is the HTTP edge of the orders example: three routes, each one a
// dmpfhttp.Route referencing the published OpenAPI (RST-04), admitted before
// the body is read (RES-17) and idempotent by key on every POST (RST-02).
package api

import (
	"net/http"

	"go.opentelemetry.io/otel/trace"

	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	dmpfhttp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/deadline"
)

// Tenant is the single tenant this edge admits: the identity of FND-07 has no
// realization in the kernel, so every request resolves to it (MET-07).
const Tenant = "public"

// IdempotencyHeader carries the client-chosen key that makes a POST idempotent.
const IdempotencyHeader = "Idempotency-Key"

// CorrelationHeader lets the client name the chain; absent or malformed, the
// edge mints one.
const CorrelationHeader = "X-Correlation-ID"

// OpenAPIPath is where the api serves its published contract when Options
// carries the document, so a client or a test tool reads the same file the
// routes reference (RST-04).
const OpenAPIPath = "/openapi.yaml"

const contract = "contracts/openapi/orders/v1/openapi.yaml#/paths/"

// Options is what the composition root decides for the edge beyond the
// service itself: the deadline budget of the routes, the contract to serve
// (nil serves nothing) and the browser origins allowed to call it (none by
// default — CORS is a development convenience, not a feature of the edge).
type Options struct {
	Budget      deadline.Budget
	OpenAPI     []byte
	CORSOrigins []string
}

// Routes declares the three operations of the contract, all sharing the
// deadline budget of the Postgres dependency.
func Routes(budget deadline.Budget) [3]dmpfhttp.Route {
	return [3]dmpfhttp.Route{
		{
			Name:           "addItem",
			Method:         http.MethodPost,
			Path:           "/orders/{id}/items",
			ContractRef:    contract + "~1orders~1{id}~1items/post",
			Budget:         budget,
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:           "placeOrder",
			Method:         http.MethodPost,
			Path:           "/orders/{id}/place",
			ContractRef:    contract + "~1orders~1{id}~1place/post",
			Budget:         budget,
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:        "findOrder",
			Method:      http.MethodGet,
			Path:        "/orders/{id}",
			ContractRef: contract + "~1orders~1{id}/get",
			Budget:      budget,
		},
	}
}

// Limits gives every route the same limit, keyed by the ServeMux pattern the
// admission middleware reads back from the request (RES-16).
func Limits(limit admission.Limit) map[string]admission.Limit {
	limits := make(map[string]admission.Limit, 3)
	for _, route := range Routes(deadline.Budget{}) {
		limits[pattern(route)] = limit
	}
	return limits
}

func pattern(route dmpfhttp.Route) string { return route.Method + " " + route.Path }

// NewHandler validates the routes at construction and mounts them on a
// ServeMux, each behind admission, the idempotency check and the message
// context, in that order: the cheapest refusal comes first. The contract, when
// served, sits outside admission — static bytes, no dependency behind them.
func NewHandler(
	service ordersapp.Service,
	ctrl *admission.Controller,
	tracer trace.Tracer,
	instruments *metrics.Instruments,
	opts Options,
) (http.Handler, error) {
	routes := Routes(opts.Budget)
	for _, route := range routes {
		if err := route.Validate(); err != nil {
			return nil, err
		}
	}

	admit := dmpfhttp.Admission(ctrl, routeOf, tenantOf, instruments)
	edge := func(next http.Handler) http.Handler {
		return admit(requireIdempotencyKey(withMessageContext(tracer, next)))
	}

	h := handlers{service: service}
	mux := http.NewServeMux()
	mux.Handle(pattern(routes[0]), edge(http.HandlerFunc(h.addItem)))
	mux.Handle(pattern(routes[1]), edge(http.HandlerFunc(h.placeOrder)))
	mux.Handle(pattern(routes[2]), edge(http.HandlerFunc(h.findOrder)))
	if len(opts.OpenAPI) > 0 {
		mux.Handle("GET "+OpenAPIPath, serveOpenAPI(opts.OpenAPI))
	}
	return withCORS(opts.CORSOrigins, mux), nil
}

// routeOf is the declared pattern, never the raw path, so the admission key
// space stays bounded to the three routes (MET-07).
func routeOf(r *http.Request) string { return r.Pattern }

func tenantOf(*http.Request) string { return Tenant }
