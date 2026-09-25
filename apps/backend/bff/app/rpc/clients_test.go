package rpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
)

type harness struct {
	fake         *fakeContexts
	orders       rpc.Orders
	reservations rpc.Reservations
	spans        *tracetest.InMemoryExporter
	tracer       trace.Tracer
}

func newHarness(t *testing.T, fake *fakeContexts) harness {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	dialer := fake.serveBuffered(t)
	opts := rpc.Options{Insecure: true, Clock: obsclock.System(), Tracer: provider.Tracer("rpc_test"), Service: "bff-test"}
	extra := grpc.WithContextDialer(dialer)

	ordersConn, err := rpc.Dial("passthrough:///orders", rpc.OrdersConfig(opts), extra)
	if err != nil {
		t.Fatalf("Dial(orders) = %v", err)
	}
	t.Cleanup(func() { _ = ordersConn.Close() })
	reservationsConn, err := rpc.Dial("passthrough:///reservations", rpc.ReservationsConfig(opts), extra)
	if err != nil {
		t.Fatalf("Dial(reservations) = %v", err)
	}
	t.Cleanup(func() { _ = reservationsConn.Close() })

	return harness{
		fake:         fake,
		orders:       rpc.NewOrders(ordersConn),
		reservations: rpc.NewReservations(reservationsConn),
		spans:        spans,
		tracer:       provider.Tracer("edge"),
	}
}

func withDeadline(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func TestMethodNamesComeFromTheGeneratedDescriptor(t *testing.T) {
	want := map[string]string{
		rpc.MethodAddItem:         "/company.orders.service.v1.OrdersService/AddItem",
		rpc.MethodPlaceOrder:      "/company.orders.service.v1.OrdersService/PlaceOrder",
		rpc.MethodFindOrder:       "/company.orders.service.v1.OrdersService/FindOrder",
		rpc.MethodReserve:         "/company.reservations.service.v1.ReservationsService/Reserve",
		rpc.MethodCancel:          "/company.reservations.service.v1.ReservationsService/Cancel",
		rpc.MethodFindReservation: "/company.reservations.service.v1.ReservationsService/FindReservation",
	}
	for got, expected := range want {
		if got != expected {
			t.Fatalf("method name = %q, want %q", got, expected)
		}
	}
}

func TestOnlyTheReadsAreDeclaredIdempotent(t *testing.T) {
	opts := rpc.Options{Insecure: true, Clock: obsclock.System()}
	policies := map[string]kernel.MethodPolicy{}
	for method, policy := range rpc.OrdersConfig(opts).Methods {
		policies[method] = policy
	}
	for method, policy := range rpc.ReservationsConfig(opts).Methods {
		policies[method] = policy
	}
	if len(policies) != 6 {
		t.Fatalf("declared %d methods, want 6", len(policies))
	}
	for method, policy := range policies {
		read := method == rpc.MethodFindOrder || method == rpc.MethodFindReservation
		if policy.Idempotent != read {
			t.Fatalf("%s: Idempotent = %v, want %v (GRP-09)", method, policy.Idempotent, read)
		}
		if read && (len(policy.RetryableCodes) != 1 || policy.RetryableCodes[0] != codes.Unavailable) {
			t.Fatalf("%s: RetryableCodes = %v, want [Unavailable] (GRP-08)", method, policy.RetryableCodes)
		}
		if policy.Budget.Limit >= 2*time.Second || policy.Budget.Slack <= 0 {
			t.Fatalf("%s: budget %+v must stay below the route budget with a slack (GRP-17)", method, policy.Budget)
		}
	}
}

func TestFindOrderRetriesAnUnavailableContext(t *testing.T) {
	fake := &fakeContexts{findOrder: func(n int) (*ordersv1.FindOrderResponse, error) {
		if n == 1 {
			return nil, status.Error(codes.Unavailable, "restarting")
		}
		return &ordersv1.FindOrderResponse{Order: &ordersv1.Order{OrderId: "o-1"}}, nil
	}}
	h := newHarness(t, fake)
	ctx := retry.WithBudget(withDeadline(t, 2*time.Second))

	resp, err := h.orders.FindOrder(ctx, &ordersv1.FindOrderRequest{OrderId: "o-1"})

	if err != nil {
		t.Fatalf("FindOrder() = %v, want success after one retry", err)
	}
	if resp.GetOrder().GetOrderId() != "o-1" {
		t.Fatalf("FindOrder() = %v", resp)
	}
	if got := len(fake.callsTo("FindOrder")); got != 2 {
		t.Fatalf("FindOrder reached the context %d times, want 2", got)
	}
}

func TestReserveIsNeverRetried(t *testing.T) {
	fake := &fakeContexts{reserve: func(int) (*reservationsv1.ReserveResponse, error) {
		return nil, status.Error(codes.Unavailable, "restarting")
	}}
	h := newHarness(t, fake)
	ctx := retry.WithBudget(withDeadline(t, 2*time.Second))

	_, err := h.reservations.Reserve(ctx, &reservationsv1.ReserveRequest{OrderId: "o-1", ItemCount: 1})

	if status.Code(err) != codes.Unavailable {
		t.Fatalf("Reserve() = %v, want Unavailable", err)
	}
	if got := len(fake.callsTo("Reserve")); got != 1 {
		t.Fatalf("Reserve reached the context %d times, want 1 (GRP-09)", got)
	}
}

func TestMetadataCarriesTheCallAndTheClientSpan(t *testing.T) {
	fake := &fakeContexts{}
	h := newHarness(t, fake)
	ctx, root := h.tracer.Start(withDeadline(t, 2*time.Second), "HTTP GET /orders/{id}", trace.WithSpanKind(trace.SpanKindServer))
	ctx = rpc.WithCall(ctx, rpc.Call{CorrelationID: "corr-1", RequestID: "req-1", IdempotencyKey: "k-1"})

	if _, err := h.orders.FindOrder(ctx, &ordersv1.FindOrderRequest{OrderId: "o-1"}); err != nil {
		t.Fatalf("FindOrder() = %v", err)
	}
	root.End()

	calls := fake.callsTo("FindOrder")
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	md := calls[0].md
	for key, want := range map[string]string{rpc.CorrelationIDKey: "corr-1", rpc.CausationIDKey: "req-1", rpc.IdempotencyKeyKey: "k-1"} {
		if got := md.Get(key); len(got) != 1 || got[0] != want {
			t.Fatalf("metadata %s = %v, want %q", key, got, want)
		}
	}
	traceparent := md.Get("traceparent")
	if len(traceparent) != 1 {
		t.Fatalf("metadata traceparent = %v, want one W3C value", traceparent)
	}
	parts := strings.Split(traceparent[0], "-")
	if len(parts) != 4 || parts[1] != root.SpanContext().TraceID().String() {
		t.Fatalf("traceparent %q does not continue the edge trace %s", traceparent[0], root.SpanContext().TraceID())
	}
	byID := map[string]tracetest.SpanStub{}
	for _, s := range h.spans.GetSpans() {
		byID[s.SpanContext.SpanID().String()] = s
	}
	sent, ok := byID[parts[2]]
	if !ok {
		t.Fatalf("traceparent span %s was not exported; spans: %v", parts[2], h.spans.GetSpans())
	}
	if sent.SpanContext.SpanID() == root.SpanContext().SpanID() || !strings.HasPrefix(sent.Name, "dmpf.grpc.client") {
		t.Fatalf("traceparent names span %q, want a client span of the call, not the edge span", sent.Name)
	}
	for span := sent; span.Parent.SpanID() != root.SpanContext().SpanID(); {
		parent, known := byID[span.Parent.SpanID().String()]
		if !known {
			t.Fatalf("client span %q does not descend from the edge span", sent.Name)
		}
		span = parent
	}
}

func TestTheContextReceivesADeadlineBelowTheCaller(t *testing.T) {
	fake := &fakeContexts{}
	h := newHarness(t, fake)
	ctx := withDeadline(t, 2*time.Second)
	callerDeadline, _ := ctx.Deadline()

	if _, err := h.orders.FindOrder(ctx, &ordersv1.FindOrderRequest{OrderId: "o-1"}); err != nil {
		t.Fatalf("FindOrder() = %v", err)
	}

	calls := fake.callsTo("FindOrder")
	if len(calls) != 1 || !calls[0].hasDeadline {
		t.Fatalf("the context received no deadline (GRP-04): %+v", calls)
	}
	if gap := callerDeadline.Sub(calls[0].deadline); gap < 50*time.Millisecond {
		t.Fatalf("the context deadline is %v before the caller's, want the method slack kept (GRP-05, GRP-17)", gap)
	}
}

func TestACallWithoutDeadlineNeverReachesTheWire(t *testing.T) {
	fake := &fakeContexts{}
	h := newHarness(t, fake)

	_, err := h.orders.FindOrder(context.Background(), &ordersv1.FindOrderRequest{OrderId: "o-1"})

	if !errors.Is(err, deadline.ErrNoDeadline) {
		t.Fatalf("FindOrder() = %v, want ErrNoDeadline (GRP-04)", err)
	}
	if got := len(fake.callsTo("FindOrder")); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
}

func TestAnExhaustedDeadlineNeverReachesTheWire(t *testing.T) {
	fake := &fakeContexts{}
	h := newHarness(t, fake)

	_, err := h.reservations.Reserve(withDeadline(t, 20*time.Millisecond), &reservationsv1.ReserveRequest{OrderId: "o-1", ItemCount: 1})

	if !errors.Is(err, deadline.ErrDeadlineExhausted) {
		t.Fatalf("Reserve() = %v, want ErrDeadlineExhausted (GRP-17)", err)
	}
	if got := len(fake.callsTo("Reserve")); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
}
