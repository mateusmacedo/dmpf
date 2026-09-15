// Package rpc is the gRPC side of the edge: one client per context, with the
// method names read from the generated descriptors and a policy per method
// (GRP-08, GRP-09, GRP-16) over dmpf-provider-grpc.
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

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/reservations/service/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/deadline"
)

const (
	methodLimit     = 1500 * time.Millisecond
	methodSlack     = 100 * time.Millisecond
	methodEstimated = 200 * time.Millisecond
)

var (
	ordersService       = ordersv1.File_company_orders_service_v1_orders_service_proto.Services().ByName("OrdersService")
	reservationsService = reservationsv1.File_company_reservations_service_v1_reservations_service_proto.Services().ByName("ReservationsService")

	OrdersServiceName       = string(ordersService.FullName())
	ReservationsServiceName = string(reservationsService.FullName())

	MethodAddItem         = fullMethod(ordersService, "AddItem")
	MethodPlaceOrder      = fullMethod(ordersService, "PlaceOrder")
	MethodFindOrder       = fullMethod(ordersService, "FindOrder")
	MethodReserve         = fullMethod(reservationsService, "Reserve")
	MethodCancel          = fullMethod(reservationsService, "Cancel")
	MethodFindReservation = fullMethod(reservationsService, "FindReservation")
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

func OrdersConfig(opts Options) dmpfgrpc.Config {
	return config("orders", OrdersServiceName, opts, map[string]dmpfgrpc.MethodPolicy{
		MethodAddItem:    policy("orders", MethodAddItem, false),
		MethodPlaceOrder: policy("orders", MethodPlaceOrder, false),
		MethodFindOrder:  policy("orders", MethodFindOrder, true),
	})
}

func ReservationsConfig(opts Options) dmpfgrpc.Config {
	return config("reservations", ReservationsServiceName, opts, map[string]dmpfgrpc.MethodPolicy{
		MethodReserve:         policy("reservations", MethodReserve, false),
		MethodCancel:          policy("reservations", MethodCancel, false),
		MethodFindReservation: policy("reservations", MethodFindReservation, true),
	})
}

func policy(dependency, method string, read bool) dmpfgrpc.MethodPolicy {
	p := dmpfgrpc.MethodPolicy{
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

func config(dependency, healthService string, opts Options, methods map[string]dmpfgrpc.MethodPolicy) dmpfgrpc.Config {
	return dmpfgrpc.Config{
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
func Dial(target string, cfg dmpfgrpc.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append([]grpc.DialOption{grpc.WithChainUnaryInterceptor(contextInterceptor)}, extra...)
	return dmpfgrpc.Dial(target, cfg, options...)
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

func invoke[Resp any](ctx context.Context, conn grpc.ClientConnInterface, method string, req any) (*Resp, error) {
	resp := new(Resp)
	if err := conn.Invoke(ctx, method, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
