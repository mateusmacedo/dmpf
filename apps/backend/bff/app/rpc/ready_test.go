package rpc_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
)

func healthOnly(t *testing.T, service string, status healthpb.HealthCheckResponse_ServingStatus) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	h := health.NewServer()
	h.SetServingStatus(service, status)
	healthpb.RegisterHealthServer(srv, h)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	return func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }
}

func refused(context.Context, string) (net.Conn, error) { return nil, errors.New("connection refused") }

func ordersChannel(t *testing.T, dialer func(context.Context, string) (net.Conn, error)) *grpc.ClientConn {
	t.Helper()
	opts := rpc.Options{Insecure: true, Clock: obsclock.System(), Service: "bff-test"}
	conn, err := rpc.Dial("passthrough:///orders", rpc.OrdersConfig(opts), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial(orders) = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestReadinessPassesOnceTheContextAnswersServing(t *testing.T) {
	conn := ordersChannel(t, healthOnly(t, rpc.OrdersServiceName, healthpb.HealthCheckResponse_SERVING))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := (rpc.Readiness{"orders": conn}).Check(ctx); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
}

func TestReadinessNamesEveryContextThatIsNotServing(t *testing.T) {
	readiness := rpc.Readiness{
		"orders":       ordersChannel(t, healthOnly(t, rpc.OrdersServiceName, healthpb.HealthCheckResponse_NOT_SERVING)),
		"reservations": ordersChannel(t, refused),
		"bookings":     ordersChannel(t, healthOnly(t, rpc.OrdersServiceName, healthpb.HealthCheckResponse_SERVING)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := readiness.Check(ctx)

	if !errors.Is(err, rpc.ErrNotReady) || !strings.HasSuffix(err.Error(), ": orders, reservations") {
		t.Fatalf("Check() = %v, want ErrNotReady naming orders and reservations", err)
	}
}
