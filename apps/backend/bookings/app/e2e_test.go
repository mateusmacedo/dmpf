//go:build integration

package app_test

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/service/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

type fixedClock struct{}

func (fixedClock) Now() ports.Instant { return 1_755_432_000_000_000_000 }

type sequenceIDs struct {
	mu     sync.Mutex
	issued int
}

func (g *sequenceIDs) NewMessageID() ports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return ports.MessageID(fmt.Sprintf("m-%06d", g.issued))
}

// dial serves the use cases over Postgres through the server chain the api
// mounts, so the suite crosses the same hop the BFF does.
func dial(t *testing.T, pool *pgxpool.Pool) *grpc.ClientConn {
	t.Helper()
	bind := func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Bookings:  provider.NewBookingRepository(tx),
			Resources: provider.NewResourceRepository(tx),
			Outbox:    tx.Outbox(provider.Mapper{}),
			Commands:  tx.CommandInbox(application.CommandConsumer, time.Second),
		}
	}
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := application.Service{
		UoW:            postgres.NewUnitOfWork(pool, bind),
		Reader:         provider.NewBookingReader(postgres.NewReadPool(pool)),
		ResourceReader: provider.NewBookingsByResourceReader(postgres.NewReadPool(pool)),
		Clock:          fixedClock{},
		IDs:            &sequenceIDs{},
		Authorize:      app.Authorization(),
		Idempotency:    policy,
	}

	limit := admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}
	ctrl, err := admission.New(admission.Config{Limits: kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), limit), MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	server, _, err := kernelgrpc.NewServer(kernelgrpc.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{rpc.ServiceName},
		UnaryInterceptors:          kernelgrpc.ServerInterceptors(rpc.ServiceName, ctrl, nil, nil, kernelgrpc.WithCommands(rpc.Commands()...)),
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
	return conn
}

var keys atomic.Int64

func call(t *testing.T, tenant string) context.Context {
	t.Helper()
	return callWithKey(t, tenant, fmt.Sprintf("e2e-k-%d", keys.Add(1)))
}

func callWithKey(t *testing.T, tenant, key string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if key != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.IdempotencyKey, key)
	}
	if tenant == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, tenant)
}

func outboxRows(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM outbox").Scan(&count); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return count
}

func TestReserveBookingEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool)

	var reserved servicev1.ReserveBookingResponse
	if err := conn.Invoke(call(t, "acme"), rpc.FullMethod("ReserveBooking"),
		&servicev1.ReserveBookingRequest{BookingId: "e2e-b-001", ResourceId: "e2e-r-001", Quantity: 3}, &reserved); err != nil {
		t.Fatalf("ReserveBooking() = %v", err)
	}
	if reserved.GetReserved().GetBookingId() != "e2e-b-001" {
		t.Fatalf("ReserveBooking() = %v, want Reserved e2e-b-001", &reserved)
	}
	if n := outboxRows(t, pool); n != 1 {
		t.Fatalf("outbox rows = %d, want 1", n)
	}

	var found servicev1.FindBookingResponse
	if err := conn.Invoke(call(t, "acme"), rpc.FullMethod("FindBooking"), &servicev1.FindBookingRequest{BookingId: "e2e-b-001"}, &found); err != nil {
		t.Fatalf("FindBooking() = %v", err)
	}
	if found.GetBooking().GetStatus() != servicev1.BookingStatus_BOOKING_STATUS_RESERVED {
		t.Fatalf("FindBooking() = %v, want RESERVED", found.GetBooking())
	}

	var cancelled servicev1.CancelBookingResponse
	if err := conn.Invoke(call(t, "acme"), rpc.FullMethod("CancelBooking"), &servicev1.CancelBookingRequest{BookingId: "e2e-b-001"}, &cancelled); err != nil {
		t.Fatalf("CancelBooking() = %v", err)
	}
	if cancelled.GetCancelled().GetBookingId() != "e2e-b-001" {
		t.Fatalf("CancelBooking() = %v, want Cancelled e2e-b-001", &cancelled)
	}

	var byResource servicev1.FindBookingsByResourceResponse
	if err := conn.Invoke(call(t, "acme"), rpc.FullMethod("FindBookingsByResource"), &servicev1.FindBookingsByResourceRequest{ResourceId: "e2e-r-001"}, &byResource); err != nil {
		t.Fatalf("FindBookingsByResource() = %v", err)
	}
	if b := byResource.GetBookings(); len(b) != 1 || b[0].GetStatus() != servicev1.BookingStatus_BOOKING_STATUS_CANCELLED {
		t.Fatalf("FindBookingsByResource() = %v, want the one booking, cancelled", b)
	}
}

func TestAnInvalidQuantityIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool)

	var resp servicev1.ReserveBookingResponse
	err := conn.Invoke(call(t, "acme"), rpc.FullMethod("ReserveBooking"),
		&servicev1.ReserveBookingRequest{BookingId: "e2e-b-002", ResourceId: "e2e-r-002", Quantity: 0}, &resp)

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ReserveBooking(quantity 0) = %v, want InvalidArgument", err)
	}
	if n := outboxRows(t, pool); n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}
}

// IDN-15 over the hop: a call the edge sent without a tenant resolves no scope,
// and persistence refuses it rather than writing under an invented one.
func TestACallWithoutATenantIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool)

	var resp servicev1.ReserveBookingResponse
	err := conn.Invoke(call(t, ""), rpc.FullMethod("ReserveBooking"),
		&servicev1.ReserveBookingRequest{BookingId: "e2e-b-003", ResourceId: "e2e-r-003", Quantity: 1}, &resp)

	if err == nil {
		t.Fatalf("ReserveBooking() without a tenant = %v, want the call refused", &resp)
	}
	if n := outboxRows(t, pool); n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}
}

func TestARepeatedReserveOverGRPCReplaysAndAnotherKeyAlreadyExists(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool)
	req := &servicev1.ReserveBookingRequest{BookingId: "e2e-b-004", ResourceId: "e2e-r-004", Quantity: 2}

	var first, again servicev1.ReserveBookingResponse
	var header metadata.MD
	if err := conn.Invoke(callWithKey(t, "acme", "e2e-replay"), rpc.FullMethod("ReserveBooking"), req, &first); err != nil {
		t.Fatalf("ReserveBooking() = %v", err)
	}
	if err := conn.Invoke(callWithKey(t, "acme", "e2e-replay"), rpc.FullMethod("ReserveBooking"), req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated ReserveBooking() = %v, want the stored response", err)
	}
	if again.GetReserved().GetBookingId() != "e2e-b-004" || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want Reserved e2e-b-004 marked as replay", &again, header)
	}

	var other servicev1.ReserveBookingResponse
	err := conn.Invoke(call(t, "acme"), rpc.FullMethod("ReserveBooking"), req, &other)
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("ReserveBooking(another key) = %v, want AlreadyExists", err)
	}
	if n := outboxRows(t, pool); n != 1 {
		t.Fatalf("outbox rows = %d, want 1: one effect per booking", n)
	}
}

func TestACommandWithoutAKeyIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool)

	var resp servicev1.ReserveBookingResponse
	err := conn.Invoke(callWithKey(t, "acme", ""), rpc.FullMethod("ReserveBooking"),
		&servicev1.ReserveBookingRequest{BookingId: "e2e-b-005", ResourceId: "e2e-r-005", Quantity: 1}, &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("ReserveBooking() without a key = %v, want InvalidArgument with %s", err, kernelgrpc.ReasonMissingIdempotencyKey)
	}
	if n := outboxRows(t, pool); n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}
}
