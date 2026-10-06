// Package rpc is the gRPC side of the edge: one client per context, with the
// method names read from the generated descriptors and a policy per method
// (GRP-08, GRP-09, GRP-16) over grpc.
package rpc

import (
	"context"
	"crypto/tls"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"

	bookingsv1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/service/v1"
	ordersv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/service/v1"
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
	TLS            *tls.Config
	Insecure       bool
	Clock          clock.Clock
	Tracer         trace.Tracer
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Propagator     propagation.TextMapPropagator
	Instruments    *metrics.Instruments
	LoggerProvider log.LoggerProvider
}

func OrdersConfig(opts Options) kernelgrpc.Config {
	return config("orders", OrdersServiceName, opts, map[string]kernelgrpc.MethodPolicy{
		MethodAddItem:    policy("orders", MethodAddItem),
		MethodPlaceOrder: policy("orders", MethodPlaceOrder),
		MethodFindOrder:  policy("orders", MethodFindOrder),
	})
}

func ReservationsConfig(opts Options) kernelgrpc.Config {
	return config("reservations", ReservationsServiceName, opts, map[string]kernelgrpc.MethodPolicy{
		MethodReserve:         policy("reservations", MethodReserve),
		MethodCancel:          policy("reservations", MethodCancel),
		MethodFindReservation: policy("reservations", MethodFindReservation),
	})
}

func BookingsConfig(opts Options) kernelgrpc.Config {
	return config("bookings", BookingsServiceName, opts, map[string]kernelgrpc.MethodPolicy{
		MethodReserveBooking:         policy("bookings", MethodReserveBooking),
		MethodCancelBooking:          policy("bookings", MethodCancelBooking),
		MethodRegisterResource:       policy("bookings", MethodRegisterResource),
		MethodFindBooking:            policy("bookings", MethodFindBooking),
		MethodFindBookingsByResource: policy("bookings", MethodFindBookingsByResource),
	})
}

// policy retries every method on Unavailable: a read has no effect, and a
// command carries the key the context deduplicates by (GRP-09, IDM-01).
func policy(dependency, method string) kernelgrpc.MethodPolicy {
	return kernelgrpc.MethodPolicy{
		Budget: deadline.Budget{
			Dependency:        dependency,
			Method:            method,
			Limit:             methodLimit,
			Slack:             methodSlack,
			EstimatedDuration: methodEstimated,
		},
		Idempotent:     true,
		RetryableCodes: []codes.Code{codes.Unavailable},
	}
}

func config(dependency, healthService string, opts Options, methods map[string]kernelgrpc.MethodPolicy) kernelgrpc.Config {
	return kernelgrpc.Config{
		TLS:                        opts.TLS,
		InsecureForDevelopmentOnly: opts.Insecure,
		Sheet:                      resilience.Defaults(dependency),
		Methods:                    methods,
		HealthServiceName:          healthService,
		Clock:                      opts.Clock,
		Tracer:                     opts.Tracer,
		TracerProvider:             opts.TracerProvider,
		MeterProvider:              opts.MeterProvider,
		Propagator:                 opts.Propagator,
		Instruments:                opts.Instruments,
		LoggerProvider:             opts.LoggerProvider,
	}
}

// Dial opens a client with the edge's context interceptor innermost, so the
// traceparent written to the metadata is the one of the client span.
func Dial(target string, cfg kernelgrpc.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append([]grpc.DialOption{grpc.WithChainUnaryInterceptor(contextInterceptor)}, extra...)
	return kernelgrpc.Dial(target, cfg, options...)
}

type Unary[Req, Resp any] func(context.Context, *Req) (*Resp, error)

type Orders struct {
	AddItem    Unary[ordersv1.AddItemRequest, ordersv1.AddItemResponse]
	PlaceOrder Unary[ordersv1.PlaceOrderRequest, ordersv1.PlaceOrderResponse]
	FindOrder  Unary[ordersv1.FindOrderRequest, ordersv1.FindOrderResponse]
}

func NewOrders(conn grpc.ClientConnInterface) Orders {
	return Orders{
		AddItem:    unaryOf[ordersv1.AddItemRequest, ordersv1.AddItemResponse](conn, MethodAddItem),
		PlaceOrder: unaryOf[ordersv1.PlaceOrderRequest, ordersv1.PlaceOrderResponse](conn, MethodPlaceOrder),
		FindOrder:  unaryOf[ordersv1.FindOrderRequest, ordersv1.FindOrderResponse](conn, MethodFindOrder),
	}
}

type Reservations struct {
	Reserve         Unary[reservationsv1.ReserveRequest, reservationsv1.ReserveResponse]
	Cancel          Unary[reservationsv1.CancelRequest, reservationsv1.CancelResponse]
	FindReservation Unary[reservationsv1.FindReservationRequest, reservationsv1.FindReservationResponse]
}

func NewReservations(conn grpc.ClientConnInterface) Reservations {
	return Reservations{
		Reserve:         unaryOf[reservationsv1.ReserveRequest, reservationsv1.ReserveResponse](conn, MethodReserve),
		Cancel:          unaryOf[reservationsv1.CancelRequest, reservationsv1.CancelResponse](conn, MethodCancel),
		FindReservation: unaryOf[reservationsv1.FindReservationRequest, reservationsv1.FindReservationResponse](conn, MethodFindReservation),
	}
}

type Bookings struct {
	ReserveBooking         Unary[bookingsv1.ReserveBookingRequest, bookingsv1.ReserveBookingResponse]
	CancelBooking          Unary[bookingsv1.CancelBookingRequest, bookingsv1.CancelBookingResponse]
	RegisterResource       Unary[bookingsv1.RegisterResourceRequest, bookingsv1.RegisterResourceResponse]
	FindBooking            Unary[bookingsv1.FindBookingRequest, bookingsv1.FindBookingResponse]
	FindBookingsByResource Unary[bookingsv1.FindBookingsByResourceRequest, bookingsv1.FindBookingsByResourceResponse]
}

func NewBookings(conn grpc.ClientConnInterface) Bookings {
	return Bookings{
		ReserveBooking:         unaryOf[bookingsv1.ReserveBookingRequest, bookingsv1.ReserveBookingResponse](conn, MethodReserveBooking),
		CancelBooking:          unaryOf[bookingsv1.CancelBookingRequest, bookingsv1.CancelBookingResponse](conn, MethodCancelBooking),
		RegisterResource:       unaryOf[bookingsv1.RegisterResourceRequest, bookingsv1.RegisterResourceResponse](conn, MethodRegisterResource),
		FindBooking:            unaryOf[bookingsv1.FindBookingRequest, bookingsv1.FindBookingResponse](conn, MethodFindBooking),
		FindBookingsByResource: unaryOf[bookingsv1.FindBookingsByResourceRequest, bookingsv1.FindBookingsByResourceResponse](conn, MethodFindBookingsByResource),
	}
}

func unaryOf[Req, Resp any](conn grpc.ClientConnInterface, method string) Unary[Req, Resp] {
	return func(ctx context.Context, req *Req) (*Resp, error) {
		resp := new(Resp)
		var header metadata.MD
		if err := conn.Invoke(ctx, method, req, resp, grpc.Header(&header)); err != nil {
			return nil, err
		}
		markReplayed(ctx, header)
		return resp, nil
	}
}
