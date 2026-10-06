package grpc_test

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
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
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const (
	callLimit  = 2 * time.Second
	drainLimit = 5 * time.Second
)

type downCollector struct {
	addr   string
	stalls bool
	held   atomic.Int64
}

func unusedLoopbackAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func refusingCollector(t *testing.T) *downCollector {
	return &downCollector{addr: unusedLoopbackAddr(t)}
}

func stallingCollector(t *testing.T) *downCollector {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	collector := &downCollector{addr: listener.Addr().String(), stalls: true}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
			collector.held.Add(1)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	return collector
}

func (c *downCollector) requireEverySignalExporting(t *testing.T) {
	t.Helper()
	if !c.stalls {
		return
	}
	for deadline := time.Now().Add(2 * time.Second); c.held.Load() < 3 && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	if held := c.held.Load(); held < 3 {
		t.Errorf("the stalled collector holds %d connections, want one per signal: traces, metrics and logs exporting to it", held)
	}
}

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
		returnsWithin(t, drainLimit, "the telemetry shutdown", func() { _ = runtime.Shutdown(grace) })
	})
	return runtime
}

func returnsWithin(t *testing.T, limit time.Duration, what string, run func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		run()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Errorf("%s did not return in %v with the export failing, want the telemetry never to hold it (NF Degradação)", what, limit)
	}
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
	for name, collector := range map[string]func(*testing.T) *downCollector{
		"unreachable endpoint": refusingCollector,
		"stalled collector":    stallingCollector,
	} {
		t.Run(name, func(t *testing.T) {
			down := collector(t)
			runtime := startTelemetryExportingTo(t, down.addr)
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
			down.requireEverySignalExporting(t)
		})
	}
}
