package rpc_test

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

var orderTable = memory.Table[domain.OrderID, domain.Snapshot]{
	Name:  "orders",
	Clone: func(s domain.Snapshot) domain.Snapshot { s.Items = slices.Clone(s.Items); return s },
}

const (
	occurred    = ports.Instant(1_755_432_000_000_000_000)
	traceID     = "0af7651916cd43dd8448eb211c80319c"
	parentSpan  = "b7ad6b7169203331"
	traceparent = "00-" + traceID + "-" + parentSpan + "-01"
	testTenant  = "acme"
)

var unlimited = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}

type harness struct {
	store     *memory.Store
	conn      *grpc.ClientConn
	spans     *tracetest.InMemoryExporter
	logs      *logRecords
	execution *capture
	calls     int
}

// capture is the innermost interceptor of the chain: it observes what the
// server rebuilt, which is otherwise only reachable from inside the handler.
type capture struct {
	execution ports.ExecutionContext
	present   bool
}

func (c *capture) intercept(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		c.execution, c.present = execution, true
	}
	return handler(ctx, req)
}

func newHarness(t *testing.T, limit admission.Limit) *harness {
	t.Helper()

	store := memory.New()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{Orders: orderTable.Repository(tx), Outbox: tx.Outbox(), Commands: tx.CommandInbox(application.CommandConsumer)}
		}),
		Reader:      orderTable.Reader(store),
		Clock:       memory.FixedClock{At: occurred},
		IDs:         &memory.SequenceIDs{Prefix: "m-"},
		Authorize:   usecase.AllowAll[application.Operation](),
		ItemLimit:   1,
		Idempotency: policy,
	}

	logs := &logRecords{}
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tenants, err := metrics.DeclareTenants(testTenant)
	if err != nil {
		t.Fatalf("DeclareTenants() = %v", err)
	}
	ctrl, err := admission.New(admission.Config{Limits: kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), limit), Tenants: tenants, MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	execution := &capture{}
	server, _, err := kernelgrpc.NewServer(kernelgrpc.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{rpc.ServiceName},
		TracerProvider:             provider,
		Propagator:                 propagation.TraceContext{},
		UnaryInterceptors: append(
			kernelgrpc.ServerInterceptors(rpc.ServiceName, ctrl, nil, sdklog.NewLoggerProvider(sdklog.WithProcessor(logs)),
				kernelgrpc.WithCommands(rpc.Commands()...)),
			execution.intercept,
		),
	})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})

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

	return &harness{store: store, conn: conn, spans: spans, logs: logs, execution: execution}
}

func (h *harness) invoke(ctx context.Context, method string, req, resp proto.Message, opts ...grpc.CallOption) error {
	err := h.conn.Invoke(ctx, rpc.FullMethod(method), req, resp, opts...)
	h.calls++
	h.awaitServerSpans(h.calls)
	return err
}

// otelgrpc ends the SERVER span on stats.End, after the caller already has the
// answer (otelgrpc@v0.72.0/stats_handler.go:375-391).
func (h *harness) awaitServerSpans(n int) {
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		ended := 0
		for _, span := range h.spans.GetSpans() {
			if span.SpanKind == trace.SpanKindServer {
				ended++
			}
		}
		if ended >= n {
			return
		}
	}
}

var keys atomic.Int64

func deadlineOnly(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// withDeadline also carries a key of its own, which every command requires
// (IDM-01) and the queries ignore.
func withDeadline(t *testing.T) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(deadlineOnly(t), kernelgrpc.IdempotencyKey, fmt.Sprintf("k-%d", keys.Add(1)))
}

func withKey(t *testing.T, key string) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(withoutKey(t), kernelgrpc.IdempotencyKey, key)
}

func withoutKey(t *testing.T) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(deadlineOnly(t), kernelgrpc.TenantKey, testTenant)
}

// withTenant is what the BFF puts on the wire: the deadline GRP-04 requires
// plus the tenant the edge resolved, without which persistence refuses the
// call (IDN-15).
func withTenant(t *testing.T) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(withDeadline(t), kernelgrpc.TenantKey, testTenant)
}

type logRecords struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (l *logRecords) OnEmit(_ context.Context, record *sdklog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, record.Clone())
	return nil
}

func (l *logRecords) snapshot() []sdklog.Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]sdklog.Record(nil), l.records...)
}

func (*logRecords) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (*logRecords) Shutdown(context.Context) error                         { return nil }
func (*logRecords) ForceFlush(context.Context) error                       { return nil }
