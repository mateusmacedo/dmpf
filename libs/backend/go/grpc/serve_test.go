package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

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
	var logs bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- kernel.Serve(ctx, listenBuf(), server, healthServer, []string{serveTestService},
			func(context.Context) error { <-release; return nil }, slog.New(slog.NewJSONHandler(&logs, nil)))
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
	if !strings.Contains(logs.String(), `"grpc listening"`) || !strings.Contains(logs.String(), `"addr"`) {
		t.Fatalf("logs = %s, want the readiness line with the address", logs.String())
	}
}

func TestServeReturnsTheReadinessFailureWithoutServing(t *testing.T) {
	server, healthServer, err := kernel.NewServer(kernel.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{serveTestService}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	unreachable := errors.New("database unreachable")

	err = kernel.Serve(context.Background(), listenBuf(), server, healthServer, []string{serveTestService},
		func(context.Context) error { return unreachable }, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))

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
	server, healthServer, err := kernel.NewServer(kernel.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{serveTestService}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- kernel.Serve(ctx, func() (net.Listener, error) { return listener, nil }, server, healthServer, []string{serveTestService},
			func(context.Context) error { return nil }, slog.New(slog.NewJSONHandler(&logs, nil)))
	}()
	eventually(t, healthServer, healthpb.HealthCheckResponse_SERVING)
	cancel()
	<-done

	addr := ""
	for _, line := range strings.Split(logs.String(), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) == nil && record["msg"] == "grpc listening" {
			addr, _ = record["addr"].(string)
		}
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "0" || port == "" {
		t.Fatalf("addr = %q, want the address the listener resolved, not the configured one", addr)
	}
}
