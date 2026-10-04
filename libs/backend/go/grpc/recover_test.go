package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const (
	panicValue      = "orders: lost row of tenant acme at 10.0.0.7"
	panicLeak       = "10.0.0.7"
	streamService   = "company.probe.service.v1.StreamProbeService"
	streamMethod    = "/" + streamService + "/Watch"
	panicChildEnv   = "DMPF_TEST_GRPC_PANIC"
	panicChildServe = "serve"
)

func TestAPanickingCallAnswersInternalAndTheServerKeepsServing(t *testing.T) {
	var calls atomic.Int32
	p := serveProbe(t, func(context.Context) error {
		if calls.Add(1) == 1 {
			panic(panicValue)
		}
		return nil
	})

	got := status.Convert(p.call(t))

	if got.Code() != codes.Internal || got.Message() != "internal failure" {
		t.Fatalf("status = %v %q, want INTERNAL %q: a panic is Unexpected (ERR-22) and answers only the public projection (ERR-20)", got.Code(), got.Message(), "internal failure")
	}
	if err := p.call(t); err != nil {
		t.Fatalf("second Probe() = %v, want nil: the panic of one call does not end the process", err)
	}
}

func TestAPanickingCallMarksTheServerSpanAsUnexpected(t *testing.T) {
	p := serveProbe(t, func(context.Context) error { panic(panicValue) })

	_ = p.call(t)

	spans := p.spans.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %v, want the SERVER span of the call", spanNames(spans))
	}
	span := spans[0]
	attributes := stringAttributes(span.Attributes)
	if span.Status.Code != otelcodes.Error || attributes["error.type"] != "Unexpected" || attributes[tracing.KeyOutcomeCategory] != "Unexpected" {
		t.Fatalf("span status = %v, attributes = %v, want Error with error.type and %s Unexpected (ERR-22, RF-B1)", span.Status.Code, attributes, tracing.KeyOutcomeCategory)
	}
	if strings.Contains(span.Status.Description, panicLeak) {
		t.Fatalf("span status = %q, want no panic value (ERR-20)", span.Status.Description)
	}
}

func TestAPanickingCallIsLoggedAsOneUnexpectedGRPCCall(t *testing.T) {
	provider, logs := newMemoryLogs(slog.LevelDebug)
	p := serveLoggedProbe(t, provider, func(context.Context) error { panic(panicValue) })

	_ = p.call(t, kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme", kernel.IdempotencyKey, "k-1")

	record := onlyGRPCCall(t, logs)
	want := map[string]any{
		"scope":                               grpcScope,
		"level":                               "ERROR",
		"rpc.system.name":                     "grpc",
		"rpc.method":                          probeSpanName,
		"rpc.response.status_code":            "INTERNAL",
		tracing.KeyOutcomeCategory:            "Unexpected",
		"error.type":                          "Unexpected",
		tracing.KeyIdempotencyKey:             "k-1",
		"correlation_id":                      "corr-1",
		"tenant_id":                           "acme",
		"baggage:" + tracing.KeyCorrelationID: "corr-1",
	}
	for key, value := range want {
		if record[key] != value {
			t.Errorf("grpc call %s = %v, want %v (RF-A3, RF-A4, RF-A5)", key, record[key], value)
		}
	}
	assertNoPanicLeak(t, logs)
}

func TestAPanickingStreamAnswersInternalAndIsLoggedAsUnexpected(t *testing.T) {
	provider, logs := newMemoryLogs(slog.LevelDebug)
	spans := tracetest.NewInMemoryExporter()
	conn, ends := serveStream(t, provider, spans)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, streamMethod)
	if err != nil {
		t.Fatalf("NewStream() = %v", err)
	}
	if err := stream.SendMsg(&healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("SendMsg() = %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend() = %v", err)
	}
	got := status.Convert(stream.RecvMsg(&healthpb.HealthCheckResponse{}))

	if got.Code() != codes.Internal || got.Message() != "internal failure" {
		t.Fatalf("status = %v %q, want INTERNAL %q (ERR-20, ERR-22)", got.Code(), got.Message(), "internal failure")
	}
	ends.await(t, 1)
	record := onlyGRPCCall(t, logs)
	if record["level"] != "ERROR" || record["rpc.method"] != strings.TrimPrefix(streamMethod, "/") ||
		record["rpc.response.status_code"] != "INTERNAL" || record["error.type"] != "Unexpected" {
		t.Fatalf("grpc call = %v, want the stream's INTERNAL at ERROR with error.type Unexpected (RF-A5)", record)
	}
	assertNoPanicLeak(t, logs)
	span := awaitSpan(t, spans)
	if span.Status.Code != otelcodes.Error || stringAttributes(span.Attributes)["error.type"] != "Unexpected" {
		t.Fatalf("span status = %v, attributes = %v, want Error with error.type Unexpected", span.Status.Code, stringAttributes(span.Attributes))
	}
}

// The runtime writes the stack of an unrecovered panic to the stderr of the
// process (go1.27 runtime/panic.go:734, printpanics), so only a process of its own shows it.
func TestServingAPanickingCall(t *testing.T) {
	if os.Getenv(panicChildEnv) != panicChildServe {
		t.Skip("runs only as the process TestAPanickingCallWritesNothingToTheStderrOfTheProcess starts")
	}
	p := serveProbe(t, func(context.Context) error { panic(panicValue) })

	if code := status.Code(p.call(t)); code != codes.Internal {
		t.Fatalf("Probe() code = %v, want INTERNAL", code)
	}
}

func TestAPanickingCallWritesNothingToTheStderrOfTheProcess(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestServingAPanickingCall$", "-test.count=1")
	command.Env = append(os.Environ(), panicChildEnv+"="+panicChildServe)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr

	err := command.Run()

	if err != nil || stderr.Len() != 0 {
		t.Fatalf("process = %v, stderr = %q, stdout = %q, want exit 0 and nothing on stderr: the log leaves only by OTLP (RF-A1)", err, stderr.String(), stdout.String())
	}
}

func serveStream(t *testing.T, logs log.LoggerProvider, spans *tracetest.InMemoryExporter) (*grpc.ClientConn, *endCounter) {
	t.Helper()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	ends := &endCounter{}
	server, _, err := kernel.NewServer(kernel.ServerConfig{
		InsecureForDevelopmentOnly: true,
		LoggerProvider:             logs,
		TracerProvider:             tracerProvider,
		Propagator:                 propagation.TraceContext{},
	}, grpc.StatsHandler(ends))
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: streamService,
		HandlerType: (*any)(nil),
		Streams: []grpc.StreamDesc{{
			StreamName:    "Watch",
			ServerStreams: true,
			Handler:       func(any, grpc.ServerStream) error { panic(panicValue) },
		}},
	}, struct{}{})
	return connect(t, listen(t, server)), ends
}

func onlyGRPCCall(t *testing.T, logs *memoryLogs) map[string]any {
	t.Helper()
	var calls []map[string]any
	for _, record := range logs.snapshot() {
		if record["msg"] == "grpc call" {
			calls = append(calls, record)
		}
	}
	if len(calls) != 1 {
		t.Fatalf("log = %v, want one grpc call record for the panicking call (RF-A5)", logs.snapshot())
	}
	return calls[0]
}

func assertNoPanicLeak(t *testing.T, logs *memoryLogs) {
	t.Helper()
	line, _ := json.Marshal(logs.snapshot())
	if strings.Contains(string(line), panicLeak) || strings.Contains(string(line), "goroutine ") {
		t.Fatalf("log = %s, want neither the panic value nor its stack (LOG-13, DAT-23)", line)
	}
}

func awaitSpan(t *testing.T, spans *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if ended := spans.GetSpans(); len(ended) == 1 {
			return ended[0]
		}
	}
	t.Fatalf("exported %v, want the SERVER span of the stream", spanNames(spans.GetSpans()))
	return tracetest.SpanStub{}
}
