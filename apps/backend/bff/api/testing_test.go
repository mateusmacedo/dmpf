package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/api"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
)

var routeBudget = deadline.Budget{Dependency: "edge", Method: "route", Limit: 2 * time.Second, Slack: 200 * time.Millisecond, EstimatedDuration: 100 * time.Millisecond}

type received struct {
	method      string
	request     proto.Message
	md          metadata.MD
	deadline    time.Time
	hasDeadline bool
	at          time.Time
}

type fakeContexts struct {
	mu      sync.Mutex
	calls   []received
	respond map[string]func(n int) (any, error)
}

func (f *fakeContexts) on(method string, respond func(n int) (any, error)) *fakeContexts {
	if f.respond == nil {
		f.respond = map[string]func(int) (any, error){}
	}
	f.respond[method] = respond
	return f
}

func (f *fakeContexts) record(ctx context.Context, method string, req proto.Message) int {
	md, _ := metadata.FromIncomingContext(ctx)
	d, ok := ctx.Deadline()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, received{method, req, md.Copy(), d, ok, time.Now()})
	n := 0
	for _, c := range f.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

func (f *fakeContexts) callsTo(method string) []received {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []received
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeContexts) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func unary[Req any, PReq interface {
	*Req
	proto.Message
}](f *fakeContexts, name string, fallback any) grpc.MethodDesc {
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			req := PReq(new(Req))
			if err := dec(req); err != nil {
				return nil, err
			}
			n := f.record(ctx, name, req)
			if respond, declared := f.respond[name]; declared {
				return respond(n)
			}
			return fallback, nil
		},
	}
}

func (f *fakeContexts) serve(t *testing.T) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	srv := grpc.NewServer()
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.OrdersServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			unary[ordersv1.AddItemRequest](f, "AddItem", &ordersv1.AddItemResponse{Result: &ordersv1.AddItemResponse_Accepted{Accepted: &ordersv1.ItemAccepted{OrderId: "o-1", ItemCount: 1}}}),
			unary[ordersv1.PlaceOrderRequest](f, "PlaceOrder", &ordersv1.PlaceOrderResponse{Result: &ordersv1.PlaceOrderResponse_Placed{Placed: &ordersv1.Placed{OrderId: "o-1"}}}),
			unary[ordersv1.FindOrderRequest](f, "FindOrder", &ordersv1.FindOrderResponse{Order: &ordersv1.Order{
				OrderId: "o-1", Status: ordersv1.OrderStatus_ORDER_STATUS_OPEN, ItemLimit: 10,
				Items: []*ordersv1.Item{{Sku: "A", Quantity: 1}},
			}}),
		},
	}, f)
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.ReservationsServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			unary[reservationsv1.ReserveRequest](f, "Reserve", &reservationsv1.ReserveResponse{Result: &reservationsv1.ReserveResponse_Reserved{Reserved: &reservationsv1.Reserved{OrderId: "o-1", ItemCount: 2}}}),
			unary[reservationsv1.CancelRequest](f, "Cancel", &reservationsv1.CancelResponse{Result: &reservationsv1.CancelResponse_Canceled{Canceled: &reservationsv1.Canceled{OrderId: "o-1"}}}),
			unary[reservationsv1.FindReservationRequest](f, "FindReservation", &reservationsv1.FindReservationResponse{Reservation: &reservationsv1.Reservation{
				OrderId: "o-1", Status: reservationsv1.ReservationStatus_RESERVATION_STATUS_CONFIRMED, ItemCount: 2,
			}}),
		},
	}, f)
	h := health.NewServer()
	h.SetServingStatus(rpc.OrdersServiceName, healthpb.HealthCheckResponse_SERVING)
	h.SetServingStatus(rpc.ReservationsServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, h)

	listener := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	return func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }
}

type fixture struct {
	fake    *fakeContexts
	handler http.Handler
	spans   *tracetest.InMemoryExporter
}

type setup struct {
	budget               deadline.Budget
	limit                admission.Limit
	ordersContract       []byte
	reservationsContract []byte
	cors                 []string
}

type option func(*setup)

func withBudget(b deadline.Budget) option { return func(s *setup) { s.budget = b } }

func withLimit(l admission.Limit) option { return func(s *setup) { s.limit = l } }

func withContracts(orders, reservations string) option {
	return func(s *setup) { s.ordersContract, s.reservationsContract = []byte(orders), []byte(reservations) }
}

func withCORS(origins ...string) option { return func(s *setup) { s.cors = origins } }

func newFixture(t *testing.T, fake *fakeContexts, options ...option) fixture {
	t.Helper()
	cfg := &setup{budget: routeBudget, limit: admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 100}}
	for _, apply := range options {
		apply(cfg)
	}

	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	tracer := provider.Tracer("bff")

	dialer := fake.serve(t)
	opts := rpc.Options{Insecure: true, Clock: obsclock.System(), Tracer: tracer, Service: "bff-test"}
	ordersConn, err := rpc.Dial("passthrough:///orders", rpc.OrdersConfig(opts), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial(orders) = %v", err)
	}
	t.Cleanup(func() { _ = ordersConn.Close() })
	reservationsConn, err := rpc.Dial("passthrough:///reservations", rpc.ReservationsConfig(opts), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial(reservations) = %v", err)
	}
	t.Cleanup(func() { _ = reservationsConn.Close() })

	tenants, err := metrics.DeclareTenants(api.Tenant)
	if err != nil {
		t.Fatalf("DeclareTenants() = %v", err)
	}
	ctrl, err := admission.New(admission.Config{Limits: api.Limits(cfg.limit), Tenants: tenants, MaxKeys: 32, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}

	handler, err := api.NewHandler(rpc.NewOrders(ordersConn), rpc.NewReservations(reservationsConn), ctrl, tracer, nil, api.Options{
		Budget:               cfg.budget,
		Authenticator:        authn.DevAuthenticator{},
		OrdersContract:       cfg.ordersContract,
		ReservationsContract: cfg.reservationsContract,
		CORSOrigins:          cfg.cors,
	})
	if err != nil {
		t.Fatalf("NewHandler() = %v", err)
	}
	return fixture{fake: fake, handler: handler, spans: spans}
}

// testCredential is what the development authenticator reads back as identity.
// Every route declares RequireSubjectAndTenant, so a request without it is
// denied before reaching a context — which is what the 401 cases assert by
// passing an empty Authorization explicitly.
const testCredential = `Bearer {"sub":"tester","tenant":"public","permissions":["orders:write","orders:read"]}`

func (f fixture) do(t *testing.T, method, path string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if _, declared := req.Header["Authorization"]; !declared {
		req.Header.Set("Authorization", testCredential)
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

func (f fixture) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	return f.do(t, http.MethodPost, path, reader, "Idempotency-Key", "k-1", "Content-Type", "application/json")
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json (body %s)", ct, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("body %s is not the expected JSON: %v", rec.Body.String(), err)
	}
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func requireRejection(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	var body rejection
	decode(t, rec, &body)
	if body.Code != code || body.Message == "" {
		t.Fatalf("rejection = %+v, want code %q with a message", body, code)
	}
}
