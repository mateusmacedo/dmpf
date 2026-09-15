package dmpfreferencereservations

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

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-reservations-go/rpc"
	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
)

func servingStatus(t *testing.T, h *health.Server) healthpb.HealthCheckResponse_ServingStatus {
	t.Helper()
	resp, err := h.Check(context.Background(), &healthpb.HealthCheckRequest{Service: rpc.ServiceName})
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

func TestServeGRPCServesOnlyAfterReadyAndStopsServingOnShutdown(t *testing.T) {
	server, healthServer, err := dmpfgrpc.NewServer(dmpfgrpc.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{rpc.ServiceName}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- serveGRPC(ctx, listenBuf(), server, healthServer, []string{rpc.ServiceName},
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
			t.Fatalf("serveGRPC() = %v, want nil on shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveGRPC() did not return after cancellation")
	}
	if got := servingStatus(t, healthServer); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("status after shutdown = %v, want NOT_SERVING", got)
	}
	if !strings.Contains(logs.String(), `"grpc listening"`) || !strings.Contains(logs.String(), `"addr"`) {
		t.Fatalf("logs = %s, want the readiness line with the address", logs.String())
	}
}

func TestServeGRPCReturnsTheReadinessFailureWithoutServing(t *testing.T) {
	server, healthServer, err := dmpfgrpc.NewServer(dmpfgrpc.ServerConfig{InsecureForDevelopmentOnly: true, Services: []string{rpc.ServiceName}})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	unreachable := errors.New("database unreachable")

	err = serveGRPC(context.Background(), listenBuf(), server, healthServer, []string{rpc.ServiceName},
		func(context.Context) error { return unreachable }, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))

	if !errors.Is(err, unreachable) {
		t.Fatalf("serveGRPC() = %v, want the readiness error", err)
	}
	if got := servingStatus(t, healthServer); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("status = %v, want NOT_SERVING", got)
	}
}

func TestHealthServicesCoverTheOverallStatus(t *testing.T) {
	server, healthServer, err := dmpfgrpc.NewServer(dmpfgrpc.ServerConfig{InsecureForDevelopmentOnly: true, Services: healthServices()})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	defer server.Stop()

	resp, err := healthServer.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}
	if got := resp.GetStatus(); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("overall status = %v before ready, want NOT_SERVING: the probe of a pod without schema must not pass", got)
	}
}

func TestTheReadinessLineCarriesThePortTheKernelChose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	server, healthServer, err := dmpfgrpc.NewServer(dmpfgrpc.ServerConfig{InsecureForDevelopmentOnly: true, Services: healthServices()})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- serveGRPC(ctx, func() (net.Listener, error) { return listener, nil }, server, healthServer, healthServices(),
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

func listenBuf() func() (net.Listener, error) {
	return func() (net.Listener, error) { return bufconn.Listen(1 << 20), nil }
}
