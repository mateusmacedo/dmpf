package rpc_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
)

type received struct {
	method      string
	md          metadata.MD
	deadline    time.Time
	hasDeadline bool
}

type fakeContexts struct {
	mu    sync.Mutex
	calls []received

	findOrder func(n int) (*ordersv1.FindOrderResponse, error)
	reserve   func(n int) (*reservationsv1.ReserveResponse, error)
}

func (f *fakeContexts) record(ctx context.Context, method string) int {
	md, _ := metadata.FromIncomingContext(ctx)
	d, ok := ctx.Deadline()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, received{method: method, md: md.Copy(), deadline: d, hasDeadline: ok})
	n := 0
	for _, c := range f.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

func (f *fakeContexts) callsTo(method string) []received {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []received
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func unary[Req any, PReq interface {
	*Req
	proto.Message
}](f *fakeContexts, name string, respond func(ctx context.Context, n int) (any, error)) grpc.MethodDesc {
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			if err := dec(PReq(new(Req))); err != nil {
				return nil, err
			}
			return respond(ctx, f.record(ctx, name))
		},
	}
}

func (f *fakeContexts) register(srv *grpc.Server) {
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.OrdersServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			unary[ordersv1.AddItemRequest](f, "AddItem", func(context.Context, int) (any, error) {
				return &ordersv1.AddItemResponse{}, nil
			}),
			unary[ordersv1.PlaceOrderRequest](f, "PlaceOrder", func(context.Context, int) (any, error) {
				return &ordersv1.PlaceOrderResponse{}, nil
			}),
			unary[ordersv1.FindOrderRequest](f, "FindOrder", func(_ context.Context, n int) (any, error) {
				if f.findOrder != nil {
					return f.findOrder(n)
				}
				return &ordersv1.FindOrderResponse{Order: &ordersv1.Order{OrderId: "o-1", Status: ordersv1.OrderStatus_ORDER_STATUS_OPEN}}, nil
			}),
		},
	}, f)
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.ReservationsServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			unary[reservationsv1.ReserveRequest](f, "Reserve", func(_ context.Context, n int) (any, error) {
				if f.reserve != nil {
					return f.reserve(n)
				}
				return &reservationsv1.ReserveResponse{}, nil
			}),
			unary[reservationsv1.CancelRequest](f, "Cancel", func(context.Context, int) (any, error) {
				return &reservationsv1.CancelResponse{}, nil
			}),
			unary[reservationsv1.FindReservationRequest](f, "FindReservation", func(context.Context, int) (any, error) {
				return &reservationsv1.FindReservationResponse{}, nil
			}),
		},
	}, f)
	h := health.NewServer()
	h.SetServingStatus(rpc.OrdersServiceName, healthpb.HealthCheckResponse_SERVING)
	h.SetServingStatus(rpc.ReservationsServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, h)
}

func (f *fakeContexts) serveBuffered(t *testing.T) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	f.register(srv)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	return func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }
}

func (f *fakeContexts) serveTCP(t *testing.T, opts ...grpc.ServerOption) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() = %v", err)
	}
	srv := grpc.NewServer(opts...)
	f.register(srv)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)
	return listener.Addr().String()
}
