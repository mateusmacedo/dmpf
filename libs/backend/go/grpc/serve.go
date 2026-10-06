package grpc

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
)

var shutdownGrace = observability.ShutdownGrace

const keyShutdownGrace = "dmpf.shutdown.grace"

// Serve runs ready before the first connection is accepted, so a probe that
// cannot speak the health protocol — a TLS listener refuses the kubelet's gRPC
// probe, which is why hmg falls back to tcpSocket — still only succeeds on a
// process that answered its dependencies.
func Serve(ctx context.Context, listen func() (net.Listener, error), server *grpc.Server, healthServer *health.Server, services []string, ready func(context.Context) error, logs log.LoggerProvider) error {
	if err := ready(ctx); err != nil {
		return err
	}
	listener, err := listen()
	if err != nil {
		return err
	}

	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	loggerOf(logs).LogAttrs(ctx, slog.LevelInfo, "grpc listening", serverAddress(listener.Addr())...)
	for _, service := range services {
		healthServer.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
	}

	select {
	case <-ctx.Done():
		Drain(ctx, server, healthServer, logs)
		return nil
	case err := <-failed:
		Drain(ctx, server, healthServer, logs)
		return err
	}
}

// Drain stops accepting, lets the calls in flight finish and reports when the
// window expires, because a forced Stop cuts answers the caller is waiting for
// and a silent exit is indistinguishable from a clean one.
func Drain(ctx context.Context, server *grpc.Server, healthServer *health.Server, logs log.LoggerProvider) {
	healthServer.Shutdown()
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(shutdownGrace):
		loggerOf(logs).WarnContext(ctx, "grpc shutdown grace expired", slog.String(keyShutdownGrace, shutdownGrace.String()))
		server.Stop()
	}
}

func serverAddress(addr net.Addr) []slog.Attr {
	host, port, err := net.SplitHostPort(addr.String())
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil {
		return []slog.Attr{slog.String(string(semconv.ServerAddressKey), addr.String())}
	}
	return []slog.Attr{slog.String(string(semconv.ServerAddressKey), host), slog.Int(string(semconv.ServerPortKey), number)}
}
