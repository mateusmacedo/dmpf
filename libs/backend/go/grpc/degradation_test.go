package grpc_test

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const (
	callLimit  = 2 * time.Second
	drainLimit = 5 * time.Second
)

func startTelemetryExportingTo(t *testing.T, collector string) *otelboot.Runtime {
	t.Helper()
	for _, pair := range os.Environ() {
		if key, _, _ := strings.Cut(pair, "="); strings.HasPrefix(key, "OTEL_") {
			t.Setenv(key, "")
			_ = os.Unsetenv(key)
		}
	}
	for key, value := range map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + collector,
		"OTEL_TRACES_EXPORTER":        "otlp",
		"OTEL_METRICS_EXPORTER":       "otlp",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_PROPAGATORS":            "tracecontext",
		"OTEL_GO_X_OBSERVABILITY":     "true",
		"OTEL_RESOURCE_ATTRIBUTES":    "service.version=test,service.instance.id=orders-api-1,dmpf.process.role=api",
		"OTEL_METRIC_EXPORT_INTERVAL": "10",
		"OTEL_BSP_SCHEDULE_DELAY":     "10",
		"OTEL_BSP_MAX_QUEUE_SIZE":     "8",
		"OTEL_BLRP_SCHEDULE_DELAY":    "10",
		"OTEL_BLRP_MAX_QUEUE_SIZE":    "8",
	} {
		t.Setenv(key, value)
	}

	runtime, err := boot.StartTelemetry(context.Background(), boot.Telemetry{
		Service: "orders",
		Role:    "api",
		Class:   tracing.ClassWrite,
		Signals: boot.Signals{Sampling: tracing.UniformRates(1), Level: slog.LevelInfo},
	})
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	t.Cleanup(func() {
		grace, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		obstest.ReturnsWithin(t, drainLimit, "the telemetry shutdown", func() { _ = runtime.Shutdown(grace) })
	})
	return runtime
}

func serveUnderTheRuntime(t *testing.T, runtime *otelboot.Runtime, handle func(context.Context) error) func(context.Context) error {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	config, err := kernel.APIServerConfig(kernel.APIServer{
		Insecure:       true,
		Services:       []string{chainService},
		Interceptors:   kernel.ServerInterceptors(chainService, ctrl, runtime.Instruments(), runtime.LoggerProvider()),
		LoggerProvider: runtime.LoggerProvider(),
	})
	if err != nil {
		t.Fatalf("APIServerConfig() = %v", err)
	}
	server, healthServer, err := kernel.NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(probeDesc(handle), struct{}{})

	listener := bufconn.Listen(1 << 20)
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- kernel.Serve(ctx, func() (net.Listener, error) { return listener, nil }, server, healthServer,
			kernel.HealthServices(chainService), func(context.Context) error { return nil }, runtime.LoggerProvider())
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("Serve() = %v, want nil on shutdown", err)
			}
		case <-time.After(drainLimit):
			t.Errorf("the drain did not return in %v with the export failing, want the telemetry never to hold it (NF Degradação)", drainLimit)
		}
	})

	conn := connect(t, func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) })
	return func(ctx context.Context) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "traceparent", probeTraceparent, kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme")
		return conn.Invoke(ctx, chainMethod, &healthpb.HealthCheckRequest{}, &healthpb.HealthCheckResponse{})
	}
}

func TestAFailingExportNeverFailsNorHoldsACallOfTheServer(t *testing.T) {
	for name, collector := range map[string]func(testing.TB) *obstest.DownCollector{
		"unreachable endpoint": obstest.UnreachableCollector,
		"stalled collector":    obstest.StallingCollector,
	} {
		t.Run(name, func(t *testing.T) {
			down := collector(t)
			runtime := startTelemetryExportingTo(t, down.Addr)
			var handled atomic.Int64
			call := serveUnderTheRuntime(t, runtime, func(context.Context) error {
				handled.Add(1)
				return nil
			})

			calls := int64(generous.Burst)
			for n := range calls {
				ctx, cancel := context.WithTimeout(context.Background(), callLimit)
				began := time.Now()
				err := call(ctx)
				cancel()
				if err != nil {
					t.Fatalf("call %d = %v after %v with the export failing, want OK within %v: telemetry never fails nor holds a call (NF Degradação)",
						n+1, err, time.Since(began).Round(time.Millisecond), callLimit)
				}
			}
			if got := handled.Load(); got != calls {
				t.Fatalf("the handler ran %d times for %d calls, want every call answered by it", got, calls)
			}
			down.RequireEverySignalExporting(t)
		})
	}
}
