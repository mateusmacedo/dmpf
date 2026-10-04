package grpc_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	lognoop "go.opentelemetry.io/otel/log/noop"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

const serveTestService = "company.orders.v1.OrdersService"

func servingStatus(t *testing.T, h *health.Server) healthpb.HealthCheckResponse_ServingStatus {
	t.Helper()
	resp, err := h.Check(context.Background(), &healthpb.HealthCheckRequest{Service: serveTestService})
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}
	return resp.GetStatus()
}

func eventually(t *testing.T, h *health.Server, want healthpb.HealthCheckResponse_ServingStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if servingStatus(t, h) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("status = %v, want %v", servingStatus(t, h), want)
}

func listenBuf() func() (net.Listener, error) {
	return func() (net.Listener, error) { return bufconn.Listen(1 << 20), nil }
}

func TestServeServesOnlyAfterReadyAndStopsServingOnShutdown(t *testing.T) {
	server, healthServer, err := kernel.NewServer(kernel.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{serveTestService}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, logs := newMemoryLogs(slog.LevelInfo)
	done := make(chan error, 1)

	go func() {
		done <- kernel.Serve(ctx, listenBuf(), server, healthServer, []string{serveTestService},
			func(context.Context) error { <-release; return nil }, provider)
	}()

	time.Sleep(20 * time.Millisecond)
	if got := servingStatus(t, healthServer); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("status before ready = %v, want NOT_SERVING", got)
	}
	close(release)
	eventually(t, healthServer, healthpb.HealthCheckResponse_SERVING)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() = %v, want nil on shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return after cancellation")
	}
	if got := servingStatus(t, healthServer); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("status after shutdown = %v, want NOT_SERVING", got)
	}
	if addr := listeningAddr(logs); addr == "" {
		t.Fatalf("logs = %v, want the readiness line with the address", logs.snapshot())
	}
}

func TestServeReturnsTheReadinessFailureWithoutServing(t *testing.T) {
	server, healthServer, err := kernel.NewServer(kernel.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{serveTestService}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	unreachable := errors.New("database unreachable")

	err = kernel.Serve(context.Background(), listenBuf(), server, healthServer, []string{serveTestService},
		func(context.Context) error { return unreachable }, lognoop.NewLoggerProvider())

	if !errors.Is(err, unreachable) {
		t.Fatalf("Serve() = %v, want the readiness error", err)
	}
	if got := servingStatus(t, healthServer); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("status = %v, want NOT_SERVING", got)
	}
}

func TestTheReadinessLineCarriesThePortTheListenerResolved(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	resolved := listener.Addr().(*net.TCPAddr).Port
	server, healthServer, err := kernel.NewServer(kernel.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{serveTestService}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, exported := productionLogs(t)
	done := make(chan error, 1)

	go func() {
		done <- kernel.Serve(ctx, func() (net.Listener, error) { return listener, nil }, server, healthServer, []string{serveTestService},
			func(context.Context) error { return nil }, provider)
	}()
	eventually(t, healthServer, healthpb.HealthCheckResponse_SERVING)
	cancel()
	<-done

	attributes := exported("grpc listening")
	if attributes["server.address"].AsString() != "127.0.0.1" || attributes["server.port"].AsInt64() != int64(resolved) {
		t.Fatalf("grpc listening = %v, want server.address 127.0.0.1 and server.port %d past the processor: the address the listener resolved (RF-A3)", attributes, resolved)
	}
}

func TestAnExpiredGraceIsLoggedUnderAPlatformKey(t *testing.T) {
	const grace = 50 * time.Millisecond
	defer kernel.SetShutdownGrace(grace)()
	listener := bufconn.Listen(1 << 20)
	server, healthServer, err := kernel.NewServer(kernel.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{serveTestService}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, exported := productionLogs(t)
	done := make(chan error, 1)

	go func() {
		done <- kernel.Serve(ctx, func() (net.Listener, error) { return listener, nil }, server, healthServer, []string{serveTestService},
			func(context.Context) error { return nil }, provider)
	}()
	eventually(t, healthServer, healthpb.HealthCheckResponse_SERVING)
	watch, err := healthpb.NewHealthClient(connect(t, func(context.Context, string) (net.Conn, error) { return listener.Dial() })).
		Watch(context.Background(), &healthpb.HealthCheckRequest{Service: serveTestService})
	if err != nil {
		t.Fatalf("Watch() = %v", err)
	}
	if _, err := watch.Recv(); err != nil {
		t.Fatalf("Recv() = %v, want the stream that holds the graceful stop open", err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return after the grace expired")
	}

	attributes := exported("grpc shutdown grace expired")
	if attributes["dmpf.shutdown.grace"].AsString() != grace.String() {
		t.Fatalf("grpc shutdown grace expired = %v, want dmpf.shutdown.grace %s past the processor (RF-A3)", attributes, grace)
	}
}
