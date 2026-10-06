// Package api is the public REST edge: each route is a provider.Route that
// references a published OpenAPI operation (RST-04) and forwards to one gRPC
// call of the orders, reservations or bookings context.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	IdempotencyHeader = "Idempotency-Key"
	ReplayedHeader    = "Idempotent-Replayed"
	CorrelationHeader = "X-Correlation-ID"

	OrdersContractPath       = "/openapi/orders/v1/openapi.yaml"
	ReservationsContractPath = "/openapi/reservations/v1/openapi.yaml"
	BookingsContractPath     = "/openapi/bookings/v1/openapi.yaml"

	ordersContract       = "apps/backend/orders/contract/openapi/v1/openapi.yaml#/paths/"
	reservationsContract = "apps/backend/reservations/contract/openapi/v1/openapi.yaml#/paths/"
	bookingsContract     = "apps/backend/bookings/contract/openapi/v1/openapi.yaml#/paths/"
)

// Options is what the composition root decides beyond the clients: the route
// budget the request deadline derives from, the contracts to serve (nil serves
// nothing), the browser origins allowed to call the edge (none by default) and
// the readiness of the contexts (nil serves no readiness route).
type Options struct {
	Budget               deadline.Budget
	Authenticator        ports.Authenticator
	OrdersContract       []byte
	ReservationsContract []byte
	BookingsContract     []byte
	CORSOrigins          []string
	Logger               *slog.Logger
	Ready                func(context.Context) error
	Draining             func() bool
	TracerProvider       trace.TracerProvider
	MeterProvider        metric.MeterProvider
	Clock                obsclock.Clock
}

type binding struct {
	route kernelhttp.Route
	build func(handlers) http.HandlerFunc
}

type surface struct {
	context  string
	contract string
	budget   deadline.Budget
}

func (s surface) command(name, path string, build func(handlers) http.HandlerFunc) binding {
	route := s.route(name, http.MethodPost, path, "write")
	route.IdempotencyKey = IdempotencyHeader
	return binding{route: route, build: build}
}

func (s surface) query(name, path string, build func(handlers) http.HandlerFunc) binding {
	return binding{route: s.route(name, http.MethodGet, path, "read"), build: build}
}

func (s surface) route(name, method, path, access string) kernelhttp.Route {
	return kernelhttp.Route{
		Name:        name,
		Method:      method,
		Path:        path,
		ContractRef: s.contract + strings.ReplaceAll(path, "/", "~1") + "/" + strings.ToLower(method),
		Budget:      s.budget,
		Requires:    kernelhttp.RequireSubjectAndTenant,
		Permission:  ports.Permission(s.context + ":" + access),
	}
}

func bindings(budget deadline.Budget) []binding {
	orders := surface{"orders", ordersContract, budget}
	reservations := surface{"reservations", reservationsContract, budget}
	bookings := surface{"bookings", bookingsContract, budget}
	return []binding{
		orders.command("addItem", "/orders/{id}/items", handlers.addItem),
		orders.command("placeOrder", "/orders/{id}/place", handlers.placeOrder),
		orders.query("findOrder", "/orders/{id}", handlers.findOrder),
		reservations.query("findReservation", "/reservations/{order_id}", handlers.findReservation),
		reservations.command("reserve", "/reservations/{order_id}/reserve", handlers.reserve),
		reservations.command("cancel", "/reservations/{order_id}/cancel", handlers.cancel),
		bookings.command("reserveBooking", "/bookings/booking", handlers.reserveBooking),
		bookings.query("findBookingByResource", "/bookings/booking", handlers.findBookingByResource),
		bookings.query("findBooking", "/bookings/booking/{id}", handlers.findBooking),
		bookings.command("cancelBooking", "/bookings/booking/{id}/cancel", handlers.cancelBooking),
		bookings.command("registerResource", "/bookings/resource", handlers.registerResource),
	}
}

func Routes(budget deadline.Budget) []kernelhttp.Route {
	bound := bindings(budget)
	routes := make([]kernelhttp.Route, len(bound))
	for i, b := range bound {
		routes[i] = b.route
	}
	return routes
}

func Limits(limit admission.Limit) map[string]admission.Limit {
	routes := Routes(deadline.Budget{})
	limits := make(map[string]admission.Limit, len(routes))
	for _, route := range routes {
		limits[pattern(route)] = limit
	}
	return limits
}

func pattern(route kernelhttp.Route) string { return route.Method + " " + route.Path }

// ErrAuthenticatorRequired refuses a handler that could not authenticate: every
// route here demands a subject, and a nil verifier would deny all of them at
// the first request instead of at the start.
var ErrAuthenticatorRequired = errors.New("api: no authenticator provided")

// WHY: the deadline wraps the execution context, not the reverse, because
// CTX-01 makes deadline mandatory and the context cannot resolve what the
// timeout has not yet set. Admission stays inside, still ahead of the body.
func NewHandler(
	orders rpc.Orders,
	reservations rpc.Reservations,
	bookings rpc.Bookings,
	ctrl *admission.Controller,
	instruments *metrics.Instruments,
	opts Options,
) (http.Handler, error) {
	if opts.Authenticator == nil {
		return nil, ErrAuthenticatorRequired
	}

	logger := loggerOf(opts)

	h := handlers{orders: orders, reservations: reservations, bookings: bookings}

	admit := kernelhttp.Admission(ctrl, routeOf, tenantOf, instruments, refuseAsRejection)
	mux := http.NewServeMux()
	for _, bound := range bindings(opts.Budget) {
		route := bound.route
		if err := route.ValidateEdge(); err != nil {
			return nil, err
		}
		handler := requireIdempotencyKey(route, bound.build(h))
		mounted := withExecutionContext(logger, opts.Authenticator, route, admit(handler))
		mux.Handle(pattern(route), withRecover(withRouteDeadline(route.Budget, mounted)))
	}
	for _, contract := range []struct {
		path     string
		document []byte
	}{
		{OrdersContractPath, opts.OrdersContract},
		{ReservationsContractPath, opts.ReservationsContract},
		{BookingsContractPath, opts.BookingsContract},
	} {
		if len(contract.document) > 0 {
			mux.Handle("GET "+contract.path, serveContract(contract.document))
		}
	}
	return otelhttp.NewHandler(withRoute(withAccessLog(logger, withCORS(opts.CORSOrigins, mux))), "bff", instrumentation(opts)...), nil
}

func instrumentation(opts Options) []otelhttp.Option {
	return []otelhttp.Option{
		otelhttp.WithTracerProvider(opts.TracerProvider),
		otelhttp.WithMeterProvider(opts.MeterProvider),
		otelhttp.WithPropagators(traceparentOnly{}),
	}
}

func loggerOf(opts Options) *slog.Logger {
	if opts.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return opts.Logger
}

func clockOf(opts Options) obsclock.Clock {
	if opts.Clock == nil {
		return obsclock.System()
	}
	return opts.Clock
}

func withRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if _, route, routed := strings.Cut(r.Pattern, " "); routed {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
}

func routeOf(r *http.Request) string { return r.Pattern }

// tenantOf keys the admission bucket by the tenant the edge authenticated
// (RES-16); admission runs inside withExecutionContext, so the context is there.
func tenantOf(r *http.Request) string {
	execution, ok := ports.ExecutionContextFrom(r.Context())
	if !ok {
		return ""
	}
	tenant, _ := execution.Tenant()
	return string(tenant)
}
