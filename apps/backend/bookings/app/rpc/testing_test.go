package rpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	bookingsTable  = memory.Table[domain.BookingID, domain.BookingSnapshot]{Name: "bookings"}
	resourcesTable = memory.Table[domain.ResourceCode, domain.ResourceSnapshot]{Name: "resources"}
)

const (
	occurred   = ports.Instant(1_755_432_000_000_000_000)
	testTenant = "acme"
)

var unlimited = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}

// byResource answers FindBookingsByResource with what the test planted: the
// memory store has no relation query, and the mapping is what is under test.
type byResource map[domain.ResourceID][]domain.BookingSnapshot

func (r byResource) LoadByResource(_ context.Context, id domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return r[id], nil
}

type harness struct {
	store *memory.Store
	conn  *grpc.ClientConn
}

func newHarness(t *testing.T, planted byResource) *harness {
	t.Helper()

	store := memory.New()
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{
				Bookings:  bookingsTable.Repository(tx),
				Resources: resourcesTable.Repository(tx),
				Outbox:    tx.Outbox(),
			}
		}),
		Reader:         bookingsTable.Reader(store),
		ResourceReader: planted,
		Clock:          memory.FixedClock{At: occurred},
		IDs:            &memory.SequenceIDs{Prefix: "m-"},
		Authorize:      usecase.AllowAll[application.Operation](),
	}

	ctrl, err := admission.New(admission.Config{Limits: kernel.MethodLimits(rpc.ServiceName, rpc.Methods(), unlimited), MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	server, _, err := kernel.NewServer(kernel.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{rpc.ServiceName},
		UnaryInterceptors:          kernel.ServerInterceptors(rpc.ServiceName, noop.NewTracerProvider().Tracer("rpc-test"), ctrl, nil, nil),
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

	return &harness{store: store, conn: conn}
}

func (h *harness) invoke(ctx context.Context, method string, req, resp proto.Message) error {
	return h.conn.Invoke(ctx, rpc.FullMethod(method), req, resp)
}

// withTenant is what the BFF puts on the wire: the deadline GRP-04 requires
// plus the tenant the edge resolved, without which persistence refuses the
// call (IDN-15).
func withTenant(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return metadata.AppendToOutgoingContext(ctx, kernel.TenantKey, testTenant)
}
