package grpc

import (
	"context"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
)

// Serve runs ready before the first connection is accepted, so a probe that
// cannot speak the health protocol — a TLS listener refuses the kubelet's gRPC
// probe, which is why hmg falls back to tcpSocket — still only succeeds on a
// process that answered its dependencies.
func Serve(ctx context.Context, listen func() (net.Listener, error), server *grpc.Server, healthServer *health.Server, services []string, ready func(context.Context) error, logger *slog.Logger) error {
	if err := ready(ctx); err != nil {
		return err
	}
	listener, err := listen()
	if err != nil {
		return err
	}

	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	logger.InfoContext(ctx, "grpc listening", "addr", listener.Addr().String())
	for _, service := range services {
		healthServer.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
	}

	select {
	case <-ctx.Done():
		Drain(ctx, server, healthServer, logger)
		return nil
	case err := <-failed:
		Drain(ctx, server, healthServer, logger)
		return err
	}
}

// Drain stops accepting, lets the calls in flight finish and reports when the
// window expires, because a forced Stop cuts answers the caller is waiting for
// and a silent exit is indistinguishable from a clean one.
func Drain(ctx context.Context, server *grpc.Server, healthServer *health.Server, logger *slog.Logger) {
	healthServer.Shutdown()
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(observability.ShutdownGrace):
		logger.WarnContext(ctx, "grpc shutdown grace expired", "grace", observability.ShutdownGrace.String())
		server.Stop()
	}
}
