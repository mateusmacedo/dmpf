// Package api is the public REST edge: each route is a provider.Route that
// references a published OpenAPI operation (RST-04) and forwards to one gRPC
// call of the orders, reservations or bookings context.
package api

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	bookingsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/service/v1"
	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	IdempotencyHeader = "Idempotency-Key"
	CorrelationHeader = "X-Correlation-ID"

	// DefaultLocale resolves CTX-01's mandatory field when the caller states no
	// preference; leaving it empty would make the context a construction defect.
	DefaultLocale = "en"

	OrdersContractPath       = "/openapi/orders/v1/openapi.yaml"
	ReservationsContractPath = "/openapi/reservations/v1/openapi.yaml"
	BookingsContractPath     = "/openapi/bookings/v1/openapi.yaml"

	ordersContract       = "contracts/openapi/orders/v1/openapi.yaml#/paths/"
	reservationsContract = "contracts/openapi/reservations/v1/openapi.yaml#/paths/"
	bookingsContract     = "contracts/openapi/bookings/v1/openapi.yaml#/paths/"
)

type OrdersClient interface {
	AddItem(context.Context, *ordersv1.AddItemRequest) (*ordersv1.AddItemResponse, error)
	PlaceOrder(context.Context, *ordersv1.PlaceOrderRequest) (*ordersv1.PlaceOrderResponse, error)
	FindOrder(context.Context, *ordersv1.FindOrderRequest) (*ordersv1.FindOrderResponse, error)
}

type ReservationsClient interface {
	Reserve(context.Context, *reservationsv1.ReserveRequest) (*reservationsv1.ReserveResponse, error)
	Cancel(context.Context, *reservationsv1.CancelRequest) (*reservationsv1.CancelResponse, error)
	FindReservation(context.Context, *reservationsv1.FindReservationRequest) (*reservationsv1.FindReservationResponse, error)
}

type BookingsClient interface {
	ReserveBooking(context.Context, *bookingsv1.ReserveBookingRequest) (*bookingsv1.ReserveBookingResponse, error)
	CancelBooking(context.Context, *bookingsv1.CancelBookingRequest) (*bookingsv1.CancelBookingResponse, error)
	RegisterResource(context.Context, *bookingsv1.RegisterResourceRequest) (*bookingsv1.RegisterResourceResponse, error)
	FindBooking(context.Context, *bookingsv1.FindBookingRequest) (*bookingsv1.FindBookingResponse, error)
	FindBookingsByResource(context.Context, *bookingsv1.FindBookingsByResourceRequest) (*bookingsv1.FindBookingsByResourceResponse, error)
}

// Options is what the composition root decides beyond the clients: the route
// budget the request deadline derives from, the contracts to serve (nil serves
// nothing) and the browser origins allowed to call the edge (none by default).
type Options struct {
	Budget               deadline.Budget
	Authenticator        ports.Authenticator
	OrdersContract       []byte
	ReservationsContract []byte
	BookingsContract     []byte
	CORSOrigins          []string
}

func Routes(budget deadline.Budget) []kernelhttp.Route {
	return []kernelhttp.Route{
		{Name: "addItem", Method: http.MethodPost, Path: "/orders/{id}/items", ContractRef: ordersContract + "~1orders~1{id}~1items/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "orders:write"},
		{Name: "placeOrder", Method: http.MethodPost, Path: "/orders/{id}/place", ContractRef: ordersContract + "~1orders~1{id}~1place/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "orders:write"},
		{Name: "findOrder", Method: http.MethodGet, Path: "/orders/{id}", ContractRef: ordersContract + "~1orders~1{id}/get", Budget: budget, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "orders:read"},
		{Name: "findReservation", Method: http.MethodGet, Path: "/reservations/{order_id}", ContractRef: reservationsContract + "~1reservations~1{order_id}/get", Budget: budget, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "reservations:read"},
		{Name: "reserve", Method: http.MethodPost, Path: "/reservations/{order_id}/reserve", ContractRef: reservationsContract + "~1reservations~1{order_id}~1reserve/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "reservations:write"},
		{Name: "cancel", Method: http.MethodPost, Path: "/reservations/{order_id}/cancel", ContractRef: reservationsContract + "~1reservations~1{order_id}~1cancel/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "reservations:write"},
		{Name: "reserveBooking", Method: http.MethodPost, Path: "/bookings/booking", ContractRef: bookingsContract + "~1bookings~1booking/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "bookings:write"},
		{Name: "findBookingByResource", Method: http.MethodGet, Path: "/bookings/booking", ContractRef: bookingsContract + "~1bookings~1booking/get", Budget: budget, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "bookings:read"},
		{Name: "findBooking", Method: http.MethodGet, Path: "/bookings/booking/{id}", ContractRef: bookingsContract + "~1bookings~1booking~1{id}/get", Budget: budget, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "bookings:read"},
		{Name: "cancelBooking", Method: http.MethodPost, Path: "/bookings/booking/{id}/cancel", ContractRef: bookingsContract + "~1bookings~1booking~1{id}~1cancel/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "bookings:write"},
		{Name: "registerResource", Method: http.MethodPost, Path: "/bookings/resource", ContractRef: bookingsContract + "~1bookings~1resource/post", Budget: budget, IdempotencyKey: IdempotencyHeader, Requires: kernelhttp.RequireSubjectAndTenant, Permission: "bookings:write"},
	}
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
	orders OrdersClient,
	reservations ReservationsClient,
	bookings BookingsClient,
	ctrl *admission.Controller,
	tracer trace.Tracer,
	instruments *metrics.Instruments,
	opts Options,
) (http.Handler, error) {
	if opts.Authenticator == nil {
		return nil, ErrAuthenticatorRequired
	}

	h := handlers{orders: orders, reservations: reservations, bookings: bookings}
	serve := map[string]http.HandlerFunc{
		"addItem":         h.addItem,
		"placeOrder":      h.placeOrder,
		"findOrder":       h.findOrder,
		"findReservation": h.findReservation,
		"reserve":         h.reserve,
		"cancel":          h.cancel,

		"reserveBooking":        h.reserveBooking,
		"findBookingByResource": h.findBookingByResource,
		"findBooking":           h.findBooking,
		"cancelBooking":         h.cancelBooking,
		"registerResource":      h.registerResource,
	}

	admit := kernelhttp.Admission(ctrl, routeOf, tenantOf, instruments, refuseAsRejection)
	mux := http.NewServeMux()
	for _, route := range Routes(opts.Budget) {
		if err := route.ValidateEdge(); err != nil {
			return nil, err
		}
		handler := requireIdempotencyKey(serve[route.Name])
		mounted := withExecutionContext(tracer, opts.Authenticator, route, admit(handler))
		mux.Handle(pattern(route), withRecover(withRouteDeadline(route.Budget, mounted)))
	}
	if len(opts.OrdersContract) > 0 {
		mux.Handle("GET "+OrdersContractPath, serveContract(opts.OrdersContract))
	}
	if len(opts.ReservationsContract) > 0 {
		mux.Handle("GET "+ReservationsContractPath, serveContract(opts.ReservationsContract))
	}
	if len(opts.BookingsContract) > 0 {
		mux.Handle("GET "+BookingsContractPath, serveContract(opts.BookingsContract))
	}
	return withCORS(opts.CORSOrigins, mux), nil
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
