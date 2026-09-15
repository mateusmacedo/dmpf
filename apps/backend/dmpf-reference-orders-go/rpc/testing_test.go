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
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-orders-go/rpc"
	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/memory"
	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/orders"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
)

const (
	occurred    = dmpfports.Instant(1_755_432_000_000_000_000)
	traceID     = "0af7651916cd43dd8448eb211c80319c"
	parentSpan  = "b7ad6b7169203331"
	traceparent = "00-" + traceID + "-" + parentSpan + "-01"
)

var unlimited = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}

type harness struct {
	store *memory.Store
	conn  *grpc.ClientConn
	spans *tracetest.InMemoryExporter
	logs  *bytes.Buffer
}

func newHarness(t *testing.T, limit admission.Limit) *harness {
	t.Helper()

	store := memory.New()
	service := ordersapp.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) ordersapp.Resources {
			return ordersapp.Resources{Orders: tx.Orders(), Outbox: tx.Outbox()}
		}),
		Reader:    store.Reader(),
		Clock:     memory.FixedClock{At: occurred},
		IDs:       &memory.SequenceIDs{Prefix: "m-"},
		Authorize: dmpfapplication.AllowAll[ordersapp.Command](),
		ItemLimit: 1,
	}

	logs := &bytes.Buffer{}
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tenants, err := metrics.DeclareTenants(rpc.Tenant)
	if err != nil {
		t.Fatalf("DeclareTenants() = %v", err)
	}
	ctrl, err := admission.New(admission.Config{Limits: rpc.Limits(limit), Tenants: tenants, MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	server, _, err := dmpfgrpc.NewServer(dmpfgrpc.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{rpc.ServiceName},
		UnaryInterceptors:          rpc.Interceptors(provider.Tracer("rpc-test"), ctrl, nil, slog.New(slog.NewJSONHandler(logs, nil))),
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

	return &harness{store: store, conn: conn, spans: spans, logs: logs}
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
