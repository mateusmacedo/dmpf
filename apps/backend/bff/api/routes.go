// Package api is the public REST edge: six routes, each a provider.Route that
// references a published OpenAPI operation (RST-04) and forwards to one gRPC
// call of the orders or reservations context.
package api

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	Tenant            = "public"
	IdempotencyHeader = "Idempotency-Key"
	CorrelationHeader = "X-Correlation-ID"

	OrdersContractPath       = "/openapi/orders/v1/openapi.yaml"
	ReservationsContractPath = "/openapi/reservations/v1/openapi.yaml"

	ordersContract       = "contracts/openapi/orders/v1/openapi.yaml#/paths/"
	reservationsContract = "contracts/openapi/reservations/v1/openapi.yaml#/paths/"
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

// Options is what the composition root decides beyond the clients: the route
// budget the request deadline derives from, the contracts to serve (nil serves
// nothing) and the browser origins allowed to call the edge (none by default).
type Options struct {
	Budget               deadline.Budget
	OrdersContract       []byte
	ReservationsContract []byte
	CORSOrigins          []string
}

func Routes(budget deadline.Budget) []provider.Route {
	return []provider.Route{
		{Name: "addItem", Method: http.MethodPost, Path: "/orders/{id}/items", ContractRef: ordersContract + "~1orders~1{id}~1items/post", Budget: budget, IdempotencyKey: IdempotencyHeader},
		{Name: "placeOrder", Method: http.MethodPost, Path: "/orders/{id}/place", ContractRef: ordersContract + "~1orders~1{id}~1place/post", Budget: budget, IdempotencyKey: IdempotencyHeader},
		{Name: "findOrder", Method: http.MethodGet, Path: "/orders/{id}", ContractRef: ordersContract + "~1orders~1{id}/get", Budget: budget},
		{Name: "findReservation", Method: http.MethodGet, Path: "/reservations/{order_id}", ContractRef: reservationsContract + "~1reservations~1{order_id}/get", Budget: budget},
		{Name: "reserve", Method: http.MethodPost, Path: "/reservations/{order_id}/reserve", ContractRef: reservationsContract + "~1reservations~1{order_id}~1reserve/post", Budget: budget, IdempotencyKey: IdempotencyHeader},
		{Name: "cancel", Method: http.MethodPost, Path: "/reservations/{order_id}/cancel", ContractRef: reservationsContract + "~1reservations~1{order_id}~1cancel/post", Budget: budget, IdempotencyKey: IdempotencyHeader},
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

func pattern(route provider.Route) string { return route.Method + " " + route.Path }

// NewHandler validates the routes and mounts each behind the request context,
// admission, the route deadline and the idempotency check, in that order: the
// span and the correlation open first, so that a refusal under load carries the
// same identifiers as any other answer, and admission still refuses before the
// body is read (RES-17).
func NewHandler(
	orders OrdersClient,
	reservations ReservationsClient,
	ctrl *admission.Controller,
	tracer trace.Tracer,
	instruments *metrics.Instruments,
	opts Options,
) (http.Handler, error) {
	h := handlers{orders: orders, reservations: reservations}
	serve := map[string]http.HandlerFunc{
		"addItem":         h.addItem,
		"placeOrder":      h.placeOrder,
		"findOrder":       h.findOrder,
		"findReservation": h.findReservation,
		"reserve":         h.reserve,
		"cancel":          h.cancel,
	}

	admit := provider.Admission(ctrl, routeOf, tenantOf, instruments, refuseAsRejection)
	mux := http.NewServeMux()
	for _, route := range Routes(opts.Budget) {
		if err := route.Validate(); err != nil {
			return nil, err
		}
		handler := requireIdempotencyKey(serve[route.Name])
		mux.Handle(pattern(route), withRecover(withRequestContext(tracer, admit(withRouteDeadline(route.Budget, handler)))))
	}
	if len(opts.OrdersContract) > 0 {
		mux.Handle("GET "+OrdersContractPath, serveContract(opts.OrdersContract))
	}
	if len(opts.ReservationsContract) > 0 {
		mux.Handle("GET "+ReservationsContractPath, serveContract(opts.ReservationsContract))
	}
	return withCORS(opts.CORSOrigins, mux), nil
}

func routeOf(r *http.Request) string { return r.Pattern }

func tenantOf(*http.Request) string { return Tenant }
