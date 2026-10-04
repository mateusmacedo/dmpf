package grpc_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

type endCounter struct{ ends atomic.Int64 }

func (*endCounter) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context { return ctx }

func (c *endCounter) HandleRPC(_ context.Context, rs stats.RPCStats) {
	if _, ended := rs.(*stats.End); ended {
		c.ends.Add(1)
	}
}

func (*endCounter) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context { return ctx }

func (*endCounter) HandleConn(context.Context, stats.ConnStats) {}

func (c *endCounter) count() int { return int(c.ends.Load()) }

func (c *endCounter) await(t *testing.T, n int) {
	t.Helper()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if c.count() >= n {
			return
		}
	}
	t.Fatalf("stats.End reached the last handler %d times, want %d", c.count(), n)
}

type undecodableCodec struct{}

func (undecodableCodec) Marshal(any) ([]byte, error) { return []byte{0xff, 0xff, 0xff}, nil }
func (undecodableCodec) Unmarshal([]byte, any) error { return nil }
func (undecodableCodec) Name() string                { return "proto" }

type contextFailure struct {
	cause          error
	category, code string
}

func (f contextFailure) Error() string         { return "orders: " + f.cause.Error() }
func (f contextFailure) Unwrap() error         { return f.cause }
func (f contextFailure) ErrorCategory() string { return f.category }
func (f contextFailure) ErrorCode() string     { return f.code }

func grpcCalls(logs *memoryLogs) []map[string]any {
	var calls []map[string]any
	for _, record := range logs.snapshot() {
		if record["msg"] == "grpc call" {
			calls = append(calls, record)
		}
	}
	return calls
}

func TestACallRefusedBeforeTheChainIsLoggedOnceUnderTheServerSpan(t *testing.T) {
	const receiveLimit = 16
	cases := []struct {
		name     string
		extra    []grpc.ServerOption
		req      *healthpb.HealthCheckRequest
		opts     []grpc.CallOption
		code     codes.Code
		category string
		leak     string
	}{
		{
			name: "undecodable request", req: &healthpb.HealthCheckRequest{},
			opts: []grpc.CallOption{grpc.ForceCodec(undecodableCodec{})},
			code: codes.Internal, category: "Unexpected", leak: "unmarshal",
		},
		{
			name: "request above the receive limit", req: &healthpb.HealthCheckRequest{Service: strings.Repeat("x", 4*receiveLimit)},
			extra: []grpc.ServerOption{grpc.MaxRecvMsgSize(receiveLimit)},
			code:  codes.ResourceExhausted, category: "RateLimited", leak: "larger than max",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, logs := newMemoryLogs(slog.LevelDebug)
			p := serveLoggedProbe(t, provider, func(context.Context) error {
				t.Error("the handler ran, want the call refused while decoding, before the unary chain")
				return nil
			}, c.extra...)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ctx = metadata.AppendToOutgoingContext(ctx, "traceparent", probeTraceparent,
				kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme")

			err := p.conn.Invoke(ctx, chainMethod, c.req, &healthpb.HealthCheckResponse{}, c.opts...)
			p.calls++
			p.awaitEnded(p.calls)

			if got := status.Code(err); got != c.code {
				t.Fatalf("Probe() code = %v (%v), want %v", got, err, c.code)
			}
			spans := p.spans.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("exported %v, want the SERVER span of the call", spanNames(spans))
			}
			calls := grpcCalls(logs)
			if len(calls) != 1 {
				t.Fatalf("grpc call records = %v, want exactly one for the hop (RF-A5)", calls)
			}
			record := calls[0]
			want := map[string]any{
				"scope":                    grpcScope,
				"level":                    "ERROR",
				"rpc.system.name":          "grpc",
				"rpc.method":               probeSpanName,
				"rpc.response.status_code": canonical(c.code),
				tracing.KeyOutcomeCategory: c.category,
				"error.type":               c.category,
				"record.trace_id":          spans[0].SpanContext.TraceID().String(),
				"record.span_id":           spans[0].SpanContext.SpanID().String(),
			}
			for key, value := range want {
				if record[key] != value {
					t.Errorf("grpc call %s = %v, want %v (RF-A2, RF-A3, RF-A5)", key, record[key], value)
				}
			}
			if record["record.trace_id"] != probeTraceID {
				t.Errorf("grpc call trace = %v, want the propagated %s", record["record.trace_id"], probeTraceID)
			}
			for _, absent := range []string{"correlation_id", "tenant_id", tracing.KeyIdempotencyKey,
				"baggage:" + tracing.KeyCorrelationID, "baggage:" + tracing.KeyRequestID, "baggage:" + tracing.KeyTenantID} {
				if _, present := record[absent]; present {
					t.Errorf("grpc call = %v, want no %s: no execution was assembled (CTX-26)", record, absent)
				}
			}
			if line, _ := json.Marshal(record); strings.Contains(string(line), c.leak) {
				t.Errorf("grpc call = %s, want no status message (LOG-13)", line)
			}
		})
	}
}

func TestEveryServedCallIsLoggedExactlyOnce(t *testing.T) {
	cases := []struct {
		name   string
		handle func(context.Context) error
		code   codes.Code
		level  string
		want   map[string]any
	}{
		{name: "success", handle: func(context.Context) error { return nil }, code: codes.OK, level: "INFO"},
		{name: "rejected", handle: func(context.Context) error { return status.Error(codes.NotFound, "absent") }, code: codes.NotFound, level: "INFO"},
		{name: "denied", handle: func(context.Context) error { return status.Error(codes.PermissionDenied, "forbidden") }, code: codes.PermissionDenied, level: "WARN"},
		{name: "failure", handle: func(context.Context) error { return status.Error(codes.Unavailable, "down") }, code: codes.Unavailable, level: "ERROR"},
		{
			name: "categorized failure", handle: func(context.Context) error { return categorizedFailure{} }, code: codes.Unknown, level: "ERROR",
			want: map[string]any{"error.type": "Unexpected", tracing.KeyErrorCode: "orders.lost", tracing.KeyOutcomeCategory: "Unexpected"},
		},
		{
			name: "categorized deadline", handle: func(context.Context) error {
				return contextFailure{cause: context.DeadlineExceeded, category: "DeadlineExceeded", code: "orders.timeout"}
			}, code: codes.DeadlineExceeded, level: "ERROR",
			want: map[string]any{"error.type": "DeadlineExceeded", tracing.KeyErrorCode: "orders.timeout", tracing.KeyOutcomeCategory: "DeadlineExceeded"},
		},
		{
			name: "categorized cancellation", handle: func(context.Context) error {
				return contextFailure{cause: context.Canceled, category: "Cancelled", code: "orders.abandoned"}
			}, code: codes.Canceled, level: "ERROR",
			want: map[string]any{"error.type": "Cancelled", tracing.KeyErrorCode: "orders.abandoned", tracing.KeyOutcomeCategory: "Cancelled"},
		},
		{name: "panic", handle: func(context.Context) error { panic(panicValue) }, code: codes.Internal, level: "ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, logs := newMemoryLogs(slog.LevelDebug)
			p := serveLoggedProbe(t, provider, c.handle)

			got := p.call(t, kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme", kernel.IdempotencyKey, "k-1")
			if code := status.Code(got); code != c.code {
				t.Fatalf("Probe() code = %v, want %v", code, c.code)
			}

			calls := grpcCalls(logs)
			if len(calls) != 1 {
				t.Fatalf("grpc call records = %v, want exactly one for the hop (RF-A5)", calls)
			}
			want := map[string]any{
				"rpc.response.status_code":            canonical(c.code),
				"level":                               c.level,
				"correlation_id":                      "corr-1",
				"tenant_id":                           "acme",
				tracing.KeyIdempotencyKey:             "k-1",
				"baggage:" + tracing.KeyCorrelationID: "corr-1",
			}
			for key, value := range c.want {
				want[key] = value
			}
			for key, value := range want {
				if calls[0][key] != value {
					t.Errorf("grpc call %s = %v, want %v: the record carries what the chain assembled (RF-A4, RF-A5)", key, calls[0][key], value)
				}
			}
			if calls[0]["baggage:"+tracing.KeyRequestID] == nil {
				t.Errorf("grpc call = %v, want the request id of this execution in the baggage (RF-B8)", calls[0])
			}
		})
	}
}

func TestTheCallIsLoggedWithTheStatusThatReachedTheClient(t *testing.T) {
	cases := []struct {
		name    string
		handle  func(context.Context) error
		extra   []grpc.ServerOption
		timeout time.Duration
		client  codes.Code
		codes   map[string]string
	}{
		{
			name: "reply above the send limit", handle: func(context.Context) error { return nil },
			extra: []grpc.ServerOption{grpc.MaxSendMsgSize(1)}, timeout: 5 * time.Second, client: codes.ResourceExhausted,
			codes: map[string]string{"RESOURCE_EXHAUSTED": "RateLimited"},
		},
		{
			name: "success after the deadline", handle: func(context.Context) error { time.Sleep(300 * time.Millisecond); return nil },
			timeout: 100 * time.Millisecond, client: codes.DeadlineExceeded,
			codes: map[string]string{"DEADLINE_EXCEEDED": "DeadlineExceeded", "CANCELLED": "Cancelled"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, logs := newMemoryLogs(slog.LevelDebug)
			p := serveLoggedProbe(t, provider, c.handle, c.extra...)
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			defer cancel()
			ctx = metadata.AppendToOutgoingContext(ctx, "traceparent", probeTraceparent,
				kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme")

			err := p.conn.Invoke(ctx, chainMethod, &healthpb.HealthCheckRequest{}, &healthpb.HealthCheckResponse{})
			p.calls++
			p.awaitEnded(p.calls)

			if got := status.Code(err); got != c.client {
				t.Fatalf("Probe() code = %v (%v), want %v", got, err, c.client)
			}
			spans := p.spans.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("exported %v, want the SERVER span of the call", spanNames(spans))
			}
			calls := grpcCalls(logs)
			if len(calls) != 1 {
				t.Fatalf("grpc call records = %v, want exactly one for the hop (RF-A5)", calls)
			}
			record := calls[0]
			code, _ := record["rpc.response.status_code"].(string)
			category, final := c.codes[code]
			if !final || code != stringAttributes(spans[0].Attributes)["rpc.response.status_code"] {
				t.Fatalf("grpc call = %v, want the final status grpc-go closed the call with, as on the SERVER span %v (RF-A5)",
					record, stringAttributes(spans[0].Attributes)["rpc.response.status_code"])
			}
			want := map[string]any{
				"level":                    "ERROR",
				tracing.KeyOutcomeCategory: category,
				"error.type":               category,
				"correlation_id":           "corr-1",
				"tenant_id":                "acme",
				"record.trace_id":          spans[0].SpanContext.TraceID().String(),
				"record.span_id":           spans[0].SpanContext.SpanID().String(),
			}
			for key, value := range want {
				if record[key] != value {
					t.Errorf("grpc call %s = %v, want %v (RF-A3, RF-A4, RF-A5)", key, record[key], value)
				}
			}
		})
	}
}

func TestTheRecordOfAServedCallLeavesByTheChainsLogger(t *testing.T) {
	provider, logs := newMemoryLogs(slog.LevelDebug)
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
		UnaryInterceptors:          kernel.ServerInterceptors(chainService, ctrl, nil, provider),
	}, grpc.StatsHandler(ends))
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(probeDesc(func(context.Context) error { return nil }), struct{}{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = connect(t, listen(t, server)).Invoke(ctx, chainMethod, &healthpb.HealthCheckRequest{}, &healthpb.HealthCheckResponse{})
	ends.await(t, 1)

	if err != nil {
		t.Fatalf("Probe() = %v, want nil", err)
	}
	if calls := grpcCalls(logs); len(calls) != 1 || calls[0]["scope"] != grpcScope {
		t.Fatalf("grpc call records = %v, want the one record on the LoggerProvider of ServerInterceptors (RF-A1, RF-A5)", calls)
	}
}

func TestATrustedPeersRefusalIsLoggedOnceWithoutAnExecution(t *testing.T) {
	authority := newPKI(t)
	provider, logs := newMemoryLogs(slog.LevelDebug)
	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	certFile, keyFile := authority.issue(t, "orders-api", false)
	config, err := kernel.APIServerConfig(kernel.APIServer{
		CertFile: certFile, KeyFile: keyFile, ClientCAFile: authority.CAFile, TrustedClients: []string{"spiffe://dmpf/bff"},
		Services: kernel.HealthServices(chainService), Interceptors: kernel.ServerInterceptors(chainService, ctrl, nil, provider),
		LoggerProvider: provider,
	})
	if err != nil {
		t.Fatalf("APIServerConfig() = %v", err)
	}
	config.TracerProvider, config.Propagator = tracerProvider, propagation.TraceContext{}
	ends := &endCounter{}
	server, _, err := kernel.NewServer(config, grpc.StatsHandler(ends))
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(probeDesc(func(context.Context) error {
		t.Error("the handler ran, want the untrusted call refused before the chain")
		return nil
	}), struct{}{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	clientCert, clientKey := authority.issue(t, "intruder", true)
	pair, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatal(err)
	}
	pem, err := os.ReadFile(authority.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(pem)
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair},
	})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, kernel.CorrelationKey, "corr-1", kernel.TenantKey, "acme")

	err = conn.Invoke(ctx, chainMethod, &healthpb.HealthCheckRequest{}, &healthpb.HealthCheckResponse{})
	ends.await(t, 1)

	if code := status.Code(err); code != codes.PermissionDenied {
		t.Fatalf("Probe() code = %v, want PERMISSION_DENIED from TrustedPeers", code)
	}
	calls := grpcCalls(logs)
	if len(calls) != 1 {
		t.Fatalf("grpc call records = %v, want exactly one for the hop (RF-A5)", calls)
	}
	want := map[string]any{"rpc.response.status_code": "PERMISSION_DENIED", "level": "WARN", tracing.KeyOutcomeCategory: "Forbidden"}
	for key, value := range want {
		if calls[0][key] != value {
			t.Errorf("grpc call %s = %v, want %v (RF-A5)", key, calls[0][key], value)
		}
	}
	for _, absent := range []string{"correlation_id", "tenant_id", "baggage:" + tracing.KeyRequestID} {
		if _, present := calls[0][absent]; present {
			t.Errorf("grpc call = %v, want no %s: the chain refused before assembling the execution (CTX-26)", calls[0], absent)
		}
	}
}

func TestHealthAndReflectionAreNotLogged(t *testing.T) {
	provider, logs := newMemoryLogs(slog.LevelDebug)
	p := serveLoggedProbe(t, provider, func(context.Context) error { return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := healthpb.NewHealthClient(p.conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
	stream, err := reflectionpb.NewServerReflectionClient(p.conn).ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatalf("ServerReflectionInfo() = %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend() = %v", err)
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("Recv() = %v, want the end of the stream", err)
	}
	if err := p.call(t); err != nil {
		t.Fatalf("Probe() = %v, want nil", err)
	}
	p.ends.await(t, 3)

	if calls := grpcCalls(logs); len(calls) != 1 || calls[0]["rpc.method"] != probeSpanName {
		t.Fatalf("grpc call records = %v, want only the probe's: health and reflection are out (RF-A5, RF-B4)", calls)
	}
}

func TestAStreamIsLoggedExactlyOnce(t *testing.T) {
	cases := []struct {
		name    string
		handler grpc.StreamHandler
		code    codes.Code
	}{
		{name: "completed", handler: func(any, grpc.ServerStream) error { return nil }, code: codes.OK},
		{name: "panic", handler: func(any, grpc.ServerStream) error { panic(panicValue) }, code: codes.Internal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, logs := newMemoryLogs(slog.LevelDebug)
			conn, ends := serveCountedStream(t, provider, c.handler)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, streamMethod)
			if err != nil {
				t.Fatalf("NewStream() = %v", err)
			}
			if err := stream.CloseSend(); err != nil {
				t.Fatalf("CloseSend() = %v", err)
			}
			err = stream.RecvMsg(&healthpb.HealthCheckResponse{})
			answered := status.Code(err)
			if errors.Is(err, io.EOF) {
				answered = codes.OK
			}
			if answered != c.code {
				t.Fatalf("RecvMsg() = %v, want %v", err, c.code)
			}
			ends.await(t, 1)

			calls := grpcCalls(logs)
			if len(calls) != 1 || calls[0]["rpc.response.status_code"] != canonical(c.code) {
				t.Fatalf("grpc call records = %v, want exactly one %s for the stream (RF-A5)", calls, canonical(c.code))
			}
		})
	}
}

func serveCountedStream(t *testing.T, logs log.LoggerProvider, handler grpc.StreamHandler) (*grpc.ClientConn, *endCounter) {
	t.Helper()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(tracetest.NewInMemoryExporter()))
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
		Streams:     []grpc.StreamDesc{{StreamName: "Watch", ServerStreams: true, Handler: handler}},
	}, struct{}{})
	return connect(t, listen(t, server)), ends
}

func canonical(code codes.Code) string {
	return map[codes.Code]string{
		codes.OK: "OK", codes.Internal: "INTERNAL", codes.ResourceExhausted: "RESOURCE_EXHAUSTED", codes.Unavailable: "UNAVAILABLE",
		codes.NotFound: "NOT_FOUND", codes.PermissionDenied: "PERMISSION_DENIED", codes.Unknown: "UNKNOWN",
		codes.DeadlineExceeded: "DEADLINE_EXCEEDED", codes.Canceled: "CANCELLED",
	}[code]
}
