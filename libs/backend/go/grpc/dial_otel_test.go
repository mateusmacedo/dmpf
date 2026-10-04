package grpc_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	flakyService = "dmpf.test.v1.Flaky"
	flakyMethod  = "/" + flakyService + "/Call"
)

type flaky struct {
	mu          sync.Mutex
	failures    int
	traceparent []string
}

func (f *flaky) answer(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	md, _ := metadata.FromIncomingContext(ctx)
	f.traceparent = append(f.traceparent, strings.Join(md.Get("traceparent"), ","))
	if f.failures > 0 {
		f.failures--
		return status.Error(codes.Unavailable, "transient")
	}
	return nil
}

func (f *flaky) desc() *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: flakyService,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Call",
			Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				if err := decode(new(healthpb.HealthCheckRequest)); err != nil {
					return nil, err
				}
				if err := f.answer(ctx); err != nil {
					return nil, err
				}
				return &healthpb.HealthCheckResponse{}, nil
			},
		}},
	}
}

func flakyConfig() provider.Config {
	sheet := resilience.Defaults("orders")
	sheet.MaxAttempts = resilience.Declare(3)
	sheet.Backoff = resilience.Declare(resilience.BackoffPolicy{Base: time.Millisecond, Factor: 2, Cap: 4 * time.Millisecond})
	return provider.Config{
		InsecureForDevelopmentOnly: true,
		Sheet:                      sheet,
		Clock:                      clock.System(),
		Rand:                       func() float64 { return 0 },
		Methods: map[string]provider.MethodPolicy{
			flakyMethod: {
				Budget: deadline.Budget{
					Dependency:        "orders",
					Method:            flakyMethod,
					Limit:             time.Second,
					Slack:             50 * time.Millisecond,
					EstimatedDuration: 10 * time.Millisecond,
				},
				Idempotent:     true,
				RetryableCodes: []codes.Code{codes.Unavailable},
			},
		},
	}
}

func TestEveryAttemptIsAClientSpanUnderTheResilienceSpan(t *testing.T) {
	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		_ = tracerProvider.Shutdown(context.Background())
		_ = meterProvider.Shutdown(context.Background())
	})

	server := &flaky{failures: 2}
	srv := grpc.NewServer()
	srv.RegisterService(server.desc(), struct{}{})
	healthServer := health.NewServer()
	healthServer.SetServingStatus(flakyService, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, healthServer)
	dialer := listen(t, srv)

	cfg := flakyConfig()
	cfg.HealthServiceName = flakyService
	cfg.Tracer = tracerProvider.Tracer("test")
	cfg.TracerProvider = tracerProvider
	cfg.MeterProvider = meterProvider
	cfg.Propagator = propagation.TraceContext{}
	conn, err := provider.Dial("passthrough:///bufnet", cfg, grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(2*time.Second))
	if err := conn.Invoke(ctx, flakyMethod, &healthpb.HealthCheckRequest{}, &healthpb.HealthCheckResponse{}); err != nil {
		t.Fatalf("Invoke() = %v, want nil after two retries", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	var internal []tracetest.SpanStub
	var clients []tracetest.SpanStub
	for _, span := range spans.GetSpans() {
		switch span.SpanKind {
		case trace.SpanKindInternal:
			internal = append(internal, span)
		case trace.SpanKindClient:
			clients = append(clients, span)
		}
	}
	if len(internal) != 1 || internal[0].Name != "dmpf.resilience orders" {
		t.Fatalf("INTERNAL spans = %v, want one dmpf.resilience orders over the attempts (RF-B5)", spanNames(internal))
	}
	if len(clients) != 3 {
		t.Fatalf("CLIENT spans = %v, want one per attempt, three for two retries (RF-B4)", spanNames(clients))
	}
	wantName := strings.TrimPrefix(flakyMethod, "/")
	for i, span := range clients {
		if span.Name != wantName {
			t.Errorf("CLIENT span %d = %q, want %q: {rpc.method} without the leading slash (RF-B4)", i, span.Name, wantName)
		}
		if span.Parent.SpanID() != internal[0].SpanContext.SpanID() {
			t.Errorf("CLIENT span %d has parent %s, want the resilience span %s (RF-B5)", i, span.Parent.SpanID(), internal[0].SpanContext.SpanID())
		}
		if attributes := stringAttributes(span.Attributes); attributes["rpc.system.name"] != "grpc" {
			t.Errorf("CLIENT span %d rpc.system.name = %q, want grpc (RF-B4)", i, attributes["rpc.system.name"])
		}
		want := "00-" + span.SpanContext.TraceID().String() + "-" + span.SpanContext.SpanID().String() + "-01"
		if server.traceparent[i] != want {
			t.Errorf("attempt %d carried traceparent %q, want its CLIENT span %q (TRC-07)", i+1, server.traceparent[i], want)
		}
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	if got := histogramCount(collected, "rpc.client.call.duration"); got != 3 {
		t.Errorf("rpc.client.call.duration counted %d attempts, want 3 from the client's MeterProvider (RF-D2)", got)
	}
}

func histogramCount(collected metricdata.ResourceMetrics, name string) uint64 {
	var count uint64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if histogram, ok := m.Data.(metricdata.Histogram[float64]); ok && m.Name == name {
				for _, point := range histogram.DataPoints {
					count += point.Count
				}
			}
		}
	}
	return count
}

func TestAHealthCallOpensNoClientSpan(t *testing.T) {
	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	dialer := serve(t, &hop{name: "C", deadlines: make(chan deadlineSeen, 1)})

	cfg := routeConfig(time.Second, 50*time.Millisecond)
	cfg.TracerProvider = tracerProvider
	conn, err := provider.Dial("passthrough:///bufnet", cfg, grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}

	for _, span := range spans.GetSpans() {
		if span.SpanKind == trace.SpanKindClient {
			t.Fatalf("health opened CLIENT span %q, want it out by WithFilter (RF-B4)", span.Name)
		}
	}
}
