package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const (
	probeTraceID     = "0af7651916cd43dd8448eb211c80319c"
	probeParentSpan  = "b7ad6b7169203331"
	probeTraceparent = "00-" + probeTraceID + "-" + probeParentSpan + "-01"
)

var probeSpanName = strings.TrimPrefix(chainMethod, "/")

type probe struct {
	conn   *grpc.ClientConn
	spans  *tracetest.InMemoryExporter
	reader *sdkmetric.ManualReader
	ends   *endCounter
	calls  int
}

func serveProbe(t *testing.T, handle func(context.Context) error) *probe {
	t.Helper()
	return serveLoggedProbe(t, nil, handle)
}

func serveLoggedProbe(t *testing.T, logs log.LoggerProvider, handle func(context.Context) error, extra ...grpc.ServerOption) *probe {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		_ = tracerProvider.Shutdown(context.Background())
		_ = meterProvider.Shutdown(context.Background())
	})

	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	ends := &endCounter{}
	server, _, err := kernel.NewServer(kernel.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{chainService},
		UnaryInterceptors:          kernel.ServerInterceptors(chainService, ctrl, nil, logs),
		LoggerProvider:             logs,
		TracerProvider:             tracerProvider,
		MeterProvider:              meterProvider,
		Propagator:                 propagation.TraceContext{},
	}, append(extra, grpc.StatsHandler(ends))...)
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(probeDesc(handle), struct{}{})
	reflection.Register(server)

	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &probe{conn: conn, spans: spans, reader: reader, ends: ends}
}

func probeDesc(handle func(context.Context) error) *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: chainService,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Probe",
			Handler: func(_ any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				req := new(healthpb.HealthCheckRequest)
				if err := decode(req); err != nil {
					return nil, err
				}
				answer := func(ctx context.Context, _ any) (any, error) {
					if err := handle(ctx); err != nil {
						return nil, err
					}
					return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
				}
				return interceptor(ctx, req, &grpc.UnaryServerInfo{FullMethod: chainMethod}, answer)
			},
		}},
	}
}

func (p *probe) call(t *testing.T, pairs ...string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
	err := p.conn.Invoke(ctx, chainMethod, &healthpb.HealthCheckRequest{}, &healthpb.HealthCheckResponse{})
	p.calls++
	p.awaitEnded(p.calls)
	return err
}

// otelgrpc ends the SERVER span and only then records rpc.server.call.duration on
// stats.End, after the caller already has the answer (otelgrpc@v0.72.0/stats_handler.go:391, :441).
func (p *probe) awaitEnded(n int) {
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		var collected metricdata.ResourceMetrics
		if len(p.spans.GetSpans()) >= n && p.reader.Collect(context.Background(), &collected) == nil &&
			histogramCount(collected, "rpc.server.call.duration") >= uint64(n) && p.ends.count() >= n {
			return
		}
	}
}

func stringAttributes(attributes []attribute.KeyValue) map[string]string {
	got := map[string]string{}
	for _, kv := range attributes {
		got[string(kv.Key)] = kv.Value.String()
	}
	return got
}

func TestTheServerSpanIsOtelgrpcsWithTheExecutionOfTheCall(t *testing.T) {
	p := serveProbe(t, func(context.Context) error { return nil })

	if err := p.call(t, "traceparent", probeTraceparent, kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme"); err != nil {
		t.Fatalf("Probe() = %v, want nil", err)
	}

	spans := p.spans.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want only the otelgrpc SERVER span (RF-B4): %v", len(spans), spanNames(spans))
	}
	span := spans[0]
	if span.Name != probeSpanName || span.SpanKind != trace.SpanKindServer {
		t.Fatalf("span = %q kind %v, want %q SERVER: {rpc.method} without the leading slash (RF-B4)", span.Name, span.SpanKind, probeSpanName)
	}
	if span.Parent.TraceID().String() != probeTraceID || span.Parent.SpanID().String() != probeParentSpan {
		t.Fatalf("parent = %s/%s, want the propagated %s/%s (TRC-07)", span.Parent.TraceID(), span.Parent.SpanID(), probeTraceID, probeParentSpan)
	}
	attributes := stringAttributes(span.Attributes)
	want := map[string]string{
		"rpc.system.name":          "grpc",
		"rpc.method":               probeSpanName,
		"rpc.response.status_code": "OK",
		tracing.KeyCorrelationID:   "corr-1",
		tracing.KeyTenantID:        "acme",
		tracing.KeyOutcomeCategory: "ok",
	}
	for key, value := range want {
		if attributes[key] != value {
			t.Errorf("%s = %q, want %q (RF-B4, RF-B8)", key, attributes[key], value)
		}
	}
	if attributes[tracing.KeyRequestID] == "" {
		t.Errorf("%s absent, want the request id of this execution (RF-B8)", tracing.KeyRequestID)
	}

	var collected metricdata.ResourceMetrics
	if err := p.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	if !recordedMetric(collected, "rpc.server.call.duration") {
		t.Errorf("rpc.server.call.duration was not recorded by the server's MeterProvider (RF-D2)")
	}
}

func TestTheHandlerRunsUnderTheExecutionBaggage(t *testing.T) {
	var members map[string]string
	var execution ports.ExecutionContext
	p := serveProbe(t, func(ctx context.Context) error {
		bag := baggage.FromContext(ctx)
		members = map[string]string{}
		for _, key := range tracing.ExecutionBaggageKeys {
			members[key] = bag.Member(key).Value()
		}
		execution, _ = ports.ExecutionContextFrom(ctx)
		return nil
	})

	if err := p.call(t, kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme"); err != nil {
		t.Fatalf("Probe() = %v, want nil", err)
	}

	want := map[string]string{
		tracing.KeyCorrelationID: "corr-1",
		tracing.KeyRequestID:     execution.RequestID(),
		tracing.KeyTenantID:      "acme",
	}
	for key, value := range want {
		if value == "" || members[key] != value {
			t.Errorf("baggage %s = %q, want %q: the spans and logs below the interceptor inherit it (RF-B8)", key, members[key], value)
		}
	}
}

func TestHealthAndReflectionOpenNoServerSpan(t *testing.T) {
	p := serveProbe(t, func(context.Context) error { return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := healthpb.NewHealthClient(p.conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
	stream, err := reflectionpb.NewServerReflectionClient(p.conn).ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatalf("ServerReflectionInfo() = %v", err)
	}
	if err := stream.Send(&reflectionpb.ServerReflectionRequest{MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{}}); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("Recv() = %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend() = %v", err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatalf("Recv() after CloseSend = nil, want the end of the stream")
	}

	if err := p.call(t); err != nil {
		t.Fatalf("Probe() = %v, want nil", err)
	}

	if spans := p.spans.GetSpans(); len(spans) != 1 || spans[0].Name != probeSpanName {
		t.Fatalf("exported %v, want only the probe's span: health and reflection are out by WithFilter (RF-B4)", spanNames(spans))
	}
}

func TestTheServerAnswersOnlyThePublicProjection(t *testing.T) {
	const secret = "pq: password authentication failed for user dmpf"
	cases := []struct {
		name     string
		err      error
		code     codes.Code
		message  string
		category string
	}{
		{name: "plain error", err: errors.New(secret), code: codes.Unknown, message: "unknown failure", category: "_OTHER"},
		{name: "wrapped status", err: fmt.Errorf("%s: %w", secret, status.Error(codes.Unavailable, "dependency unavailable")), code: codes.Unavailable, message: "dependency unavailable", category: "TransientDependency"},
		{name: "status", err: status.Error(codes.Internal, "internal failure"), code: codes.Internal, message: "internal failure", category: "Unexpected"},
		{name: "deadline", err: fmt.Errorf("%s: %w", secret, context.DeadlineExceeded), code: codes.DeadlineExceeded, message: "deadline exceeded", category: "DeadlineExceeded"},
		{name: "cancellation", err: fmt.Errorf("%s: %w", secret, context.Canceled), code: codes.Canceled, message: "canceled", category: "Cancelled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := serveProbe(t, func(context.Context) error { return c.err })

			got := status.Convert(p.call(t))

			if got.Code() != c.code || got.Message() != c.message {
				t.Fatalf("status = %v %q, want %v %q: the caller receives only the public projection (ERR-20)", got.Code(), got.Message(), c.code, c.message)
			}
			spans := p.spans.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("exported %v, want the SERVER span", spanNames(spans))
			}
			span := spans[0]
			if strings.Contains(span.Status.Description, "password") || (span.Status.Code == otelcodes.Error && span.Status.Description != c.message) {
				t.Fatalf("span status = %v %q, want at most the public message %q: otelgrpc records it in 6 codes (ERR-20)", span.Status.Code, span.Status.Description, c.message)
			}
			if category := stringAttributes(span.Attributes)[tracing.KeyOutcomeCategory]; category != c.category {
				t.Fatalf("%s = %q, want %q", tracing.KeyOutcomeCategory, category, c.category)
			}
		})
	}
}

func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}
	return names
}

func recordedMetric(collected metricdata.ResourceMetrics, name string) bool {
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == name {
				return true
			}
		}
	}
	return false
}
