package rpc_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

var reservationsTable = memory.Table[domain.OrderID, domain.Snapshot]{Name: "reservations"}

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
	logs      *bytes.Buffer
	execution *capture
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
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{Inbox: tx.Inbox("reservations"), Reservations: reservationsTable.Repository(tx), Outbox: tx.Outbox()}
		}),
		Reader:    reservationsTable.Reader(store),
		Clock:     memory.FixedClock{At: occurred},
		IDs:       &memory.SequenceIDs{Prefix: "m-"},
		Authorize: usecase.AllowAll[application.Operation](),
		Consumer:  "reservations",
	}

	logs := &bytes.Buffer{}
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tenants, err := metrics.DeclareTenants(testTenant)
	if err != nil {
		t.Fatalf("DeclareTenants() = %v", err)
	}
	ctrl, err := admission.New(admission.Config{Limits: kernel.MethodLimits(rpc.ServiceName, rpc.Methods(), limit), Tenants: tenants, MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	execution := &capture{}
	server, _, err := kernel.NewServer(kernel.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{rpc.ServiceName},
		UnaryInterceptors: append(
			kernel.ServerInterceptors(rpc.ServiceName, provider.Tracer("rpc-test"), ctrl, nil, slog.New(slog.NewJSONHandler(logs, nil))),
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

func (h *harness) invoke(ctx context.Context, method string, req, resp proto.Message) error {
	return h.conn.Invoke(ctx, rpc.FullMethod(method), req, resp)
}

func withDeadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// withTenant is what the BFF puts on the wire: the deadline GRP-04 requires
// plus the tenant the edge resolved, without which persistence refuses the
// call (IDN-15).
func withTenant(t *testing.T) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(withDeadline(t), kernel.TenantKey, testTenant)
}
