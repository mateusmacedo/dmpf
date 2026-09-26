// Package rpc is the gRPC side of the edge: one client per context, with the
// method names read from the generated descriptors and a policy per method
// (GRP-08, GRP-09, GRP-16) over grpc.
package rpc

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/reflect/protoreflect"

	bookingsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/service/v1"
	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	methodLimit     = 1500 * time.Millisecond
	methodSlack     = 100 * time.Millisecond
	methodEstimated = 200 * time.Millisecond
)

var (
	ordersService       = ordersv1.File_company_orders_service_v1_orders_service_proto.Services().ByName("OrdersService")
	reservationsService = reservationsv1.File_company_reservations_service_v1_reservations_service_proto.Services().ByName("ReservationsService")
	bookingsService     = bookingsv1.File_company_bookings_service_v1_bookings_service_proto.Services().ByName("BookingsService")

	OrdersServiceName       = string(ordersService.FullName())
	ReservationsServiceName = string(reservationsService.FullName())
	BookingsServiceName     = string(bookingsService.FullName())

	MethodAddItem         = fullMethod(ordersService, "AddItem")
	MethodPlaceOrder      = fullMethod(ordersService, "PlaceOrder")
	MethodFindOrder       = fullMethod(ordersService, "FindOrder")
	MethodReserve         = fullMethod(reservationsService, "Reserve")
	MethodCancel          = fullMethod(reservationsService, "Cancel")
	MethodFindReservation = fullMethod(reservationsService, "FindReservation")

	MethodReserveBooking         = fullMethod(bookingsService, "ReserveBooking")
	MethodCancelBooking          = fullMethod(bookingsService, "CancelBooking")
	MethodRegisterResource       = fullMethod(bookingsService, "RegisterResource")
	MethodFindBooking            = fullMethod(bookingsService, "FindBooking")
	MethodFindBookingsByResource = fullMethod(bookingsService, "FindBookingsByResource")
)

func fullMethod(service protoreflect.ServiceDescriptor, name protoreflect.Name) string {
	method := service.Methods().ByName(name)
	if method == nil {
		panic("rpc: " + string(service.FullName()) + " declares no method " + string(name))
	}
	return "/" + string(service.FullName()) + "/" + string(method.Name())
}

// Options is what the composition root decides for both clients: transport
// security (TLS or the development opt-out) and where the calls record.
type Options struct {
	TLS         *tls.Config
	Insecure    bool
	Clock       clock.Clock
	Tracer      trace.Tracer
	Instruments *metrics.Instruments
	Logger      *slog.Logger
	Service     string
}

func OrdersConfig(opts Options) kernelgrpc.Config {
	return config("orders", OrdersServiceName, opts, map[string]kernelgrpc.MethodPolicy{
		MethodAddItem:    policy("orders", MethodAddItem, false),
		MethodPlaceOrder: policy("orders", MethodPlaceOrder, false),
		MethodFindOrder:  policy("orders", MethodFindOrder, true),
	})
}

func ReservationsConfig(opts Options) kernelgrpc.Config {
	return config("reservations", ReservationsServiceName, opts, map[string]kernelgrpc.MethodPolicy{
		MethodReserve:         policy("reservations", MethodReserve, false),
		MethodCancel:          policy("reservations", MethodCancel, false),
		MethodFindReservation: policy("reservations", MethodFindReservation, true),
	})
}

func BookingsConfig(opts Options) kernelgrpc.Config {
	return config("bookings", BookingsServiceName, opts, map[string]kernelgrpc.MethodPolicy{
		MethodReserveBooking:         policy("bookings", MethodReserveBooking, false),
		MethodCancelBooking:          policy("bookings", MethodCancelBooking, false),
		MethodRegisterResource:       policy("bookings", MethodRegisterResource, false),
		MethodFindBooking:            policy("bookings", MethodFindBooking, true),
		MethodFindBookingsByResource: policy("bookings", MethodFindBookingsByResource, true),
	})
}

func policy(dependency, method string, read bool) kernelgrpc.MethodPolicy {
	p := kernelgrpc.MethodPolicy{
		Budget: deadline.Budget{
			Dependency:        dependency,
			Method:            method,
			Limit:             methodLimit,
			Slack:             methodSlack,
			EstimatedDuration: methodEstimated,
		},
		Idempotent: read,
	}
	if read {
		p.RetryableCodes = []codes.Code{codes.Unavailable}
	}
	return p
}

func config(dependency, healthService string, opts Options, methods map[string]kernelgrpc.MethodPolicy) kernelgrpc.Config {
	return kernelgrpc.Config{
		TLS:                        opts.TLS,
		InsecureForDevelopmentOnly: opts.Insecure,
		Sheet:                      resilience.Defaults(dependency),
		Methods:                    methods,
		HealthServiceName:          healthService,
		Service:                    opts.Service,
		Clock:                      opts.Clock,
		Tracer:                     opts.Tracer,
		Instruments:                opts.Instruments,
		Logger:                     opts.Logger,
	}
}

// Dial opens a client with the edge's context interceptor innermost, so the
// traceparent written to the metadata is the one of the client span.
func Dial(target string, cfg kernelgrpc.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append([]grpc.DialOption{grpc.WithChainUnaryInterceptor(contextInterceptor)}, extra...)
	return kernelgrpc.Dial(target, cfg, options...)
}

type Orders struct{ conn grpc.ClientConnInterface }

func NewOrders(conn grpc.ClientConnInterface) Orders { return Orders{conn: conn} }

func (o Orders) AddItem(ctx context.Context, req *ordersv1.AddItemRequest) (*ordersv1.AddItemResponse, error) {
	return invoke[ordersv1.AddItemResponse](ctx, o.conn, MethodAddItem, req)
}

func (o Orders) PlaceOrder(ctx context.Context, req *ordersv1.PlaceOrderRequest) (*ordersv1.PlaceOrderResponse, error) {
	return invoke[ordersv1.PlaceOrderResponse](ctx, o.conn, MethodPlaceOrder, req)
}

func (o Orders) FindOrder(ctx context.Context, req *ordersv1.FindOrderRequest) (*ordersv1.FindOrderResponse, error) {
	return invoke[ordersv1.FindOrderResponse](ctx, o.conn, MethodFindOrder, req)
}

type Reservations struct{ conn grpc.ClientConnInterface }

func NewReservations(conn grpc.ClientConnInterface) Reservations { return Reservations{conn: conn} }

func (r Reservations) Reserve(ctx context.Context, req *reservationsv1.ReserveRequest) (*reservationsv1.ReserveResponse, error) {
	return invoke[reservationsv1.ReserveResponse](ctx, r.conn, MethodReserve, req)
}

func (r Reservations) Cancel(ctx context.Context, req *reservationsv1.CancelRequest) (*reservationsv1.CancelResponse, error) {
	return invoke[reservationsv1.CancelResponse](ctx, r.conn, MethodCancel, req)
}

func (r Reservations) FindReservation(ctx context.Context, req *reservationsv1.FindReservationRequest) (*reservationsv1.FindReservationResponse, error) {
	return invoke[reservationsv1.FindReservationResponse](ctx, r.conn, MethodFindReservation, req)
}

type Bookings struct{ conn grpc.ClientConnInterface }

func NewBookings(conn grpc.ClientConnInterface) Bookings { return Bookings{conn: conn} }

func (b Bookings) ReserveBooking(ctx context.Context, req *bookingsv1.ReserveBookingRequest) (*bookingsv1.ReserveBookingResponse, error) {
	return invoke[bookingsv1.ReserveBookingResponse](ctx, b.conn, MethodReserveBooking, req)
}

func (b Bookings) CancelBooking(ctx context.Context, req *bookingsv1.CancelBookingRequest) (*bookingsv1.CancelBookingResponse, error) {
	return invoke[bookingsv1.CancelBookingResponse](ctx, b.conn, MethodCancelBooking, req)
}

func (b Bookings) RegisterResource(ctx context.Context, req *bookingsv1.RegisterResourceRequest) (*bookingsv1.RegisterResourceResponse, error) {
	return invoke[bookingsv1.RegisterResourceResponse](ctx, b.conn, MethodRegisterResource, req)
}

func (b Bookings) FindBooking(ctx context.Context, req *bookingsv1.FindBookingRequest) (*bookingsv1.FindBookingResponse, error) {
	return invoke[bookingsv1.FindBookingResponse](ctx, b.conn, MethodFindBooking, req)
}

func (b Bookings) FindBookingsByResource(ctx context.Context, req *bookingsv1.FindBookingsByResourceRequest) (*bookingsv1.FindBookingsByResourceResponse, error) {
	return invoke[bookingsv1.FindBookingsByResourceResponse](ctx, b.conn, MethodFindBookingsByResource, req)
}

func invoke[Resp any](ctx context.Context, conn grpc.ClientConnInterface, method string, req any) (*Resp, error) {
	resp := new(Resp)
	if err := conn.Invoke(ctx, method, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
