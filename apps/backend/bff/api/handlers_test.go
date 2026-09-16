package api_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/api"
)

func TestRoutesReferenceThePublishedContracts(t *testing.T) {
	routes := api.Routes(routeBudget)
	if len(routes) != 6 {
		t.Fatalf("Routes() has %d routes, want 6", len(routes))
	}
	for _, route := range routes {
		if err := route.Validate(); err != nil {
			t.Fatalf("%s: Validate() = %v", route.Name, err)
		}
		context := "orders"
		if strings.HasPrefix(route.Path, "/reservations/") {
			context = "reservations"
		}
		if !strings.HasPrefix(route.ContractRef, "contracts/openapi/"+context+"/v1/openapi.yaml#/paths/") {
			t.Fatalf("%s: ContractRef = %q, want the %s contract (RST-04)", route.Name, route.ContractRef, context)
		}
		if route.Method == http.MethodPost && (route.IdempotencyKey != api.IdempotencyHeader || !route.Idempotent()) {
			t.Fatalf("%s: a POST must declare %s (RST-02)", route.Name, api.IdempotencyHeader)
		}
	}
	stripped := routes[0]
	stripped.ContractRef = ""
	if err := stripped.Validate(); !errors.Is(err, provider.ErrContractRequired) {
		t.Fatalf("Validate() without ContractRef = %v, want ErrContractRequired", err)
	}
}

func TestAPostWithoutIdempotencyKeyNeverCallsAContext(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodPost, "/reservations/o-1/cancel", nil)

	requireRejection(t, rec, http.StatusBadRequest, "missing-idempotency-key")
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}

func TestAddItemAcceptedIs201(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.post(t, "/orders/o-1/items", `{"sku":"A","quantity":1}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Order string `json:"order"`
		Items int    `json:"items"`
	}
	decode(t, rec, &body)
	if body.Order != "o-1" || body.Items != 1 {
		t.Fatalf("body = %+v, want order o-1 with 1 item", body)
	}
	calls := f.fake.callsTo("AddItem")
	req, ok := calls[0].request.(*ordersv1.AddItemRequest)
	if len(calls) != 1 || !ok || req.GetOrderId() != "o-1" || req.GetSku() != "A" || req.GetQuantity() != 1 {
		t.Fatalf("AddItem request = %v, want order o-1, sku A, quantity 1", calls)
	}
}

func TestEveryRouteAnswersItsSuccess(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	cases := []struct {
		method, path, body string
		status             int
		want               string
	}{
		{http.MethodPost, "/orders/o-1/place", "", http.StatusOK, `{"order":"o-1"}`},
		{http.MethodGet, "/orders/o-1", "", http.StatusOK, `{"id":"o-1","status":"open","itemLimit":10,"items":[{"sku":"A","quantity":1}]}`},
		{http.MethodPost, "/reservations/o-1/reserve", `{"items":2}`, http.StatusOK, `{"order":"o-1","items":2}`},
		{http.MethodPost, "/reservations/o-1/cancel", "", http.StatusOK, `{"order":"o-1"}`},
		{http.MethodGet, "/reservations/o-1", "", http.StatusOK, `{"order":"o-1","status":"confirmed","items":2}`},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var rec = f.do(t, tc.method, tc.path, strings.NewReader(tc.body), "Idempotency-Key", "k-1")
			if rec.Code != tc.status || strings.TrimSpace(rec.Body.String()) != tc.want {
				t.Fatalf("%d %s, want %d %s", rec.Code, rec.Body.String(), tc.status, tc.want)
			}
		})
	}
}

func TestACanceledReservationReadsAsCanceled(t *testing.T) {
	fake := (&fakeContexts{}).on("FindReservation", func(int) (any, error) {
		return &reservationsv1.FindReservationResponse{Reservation: &reservationsv1.Reservation{
			OrderId: "o-1", Status: reservationsv1.ReservationStatus_RESERVATION_STATUS_CANCELED,
		}}, nil
	})
	f := newFixture(t, fake)

	rec := f.do(t, http.MethodGet, "/reservations/o-1", nil)

	if strings.TrimSpace(rec.Body.String()) != `{"order":"o-1","status":"canceled","items":0}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestARejectionIs422WithTheDomainCode(t *testing.T) {
	fake := (&fakeContexts{}).on("Cancel", func(int) (any, error) {
		return &reservationsv1.CancelResponse{Result: &reservationsv1.CancelResponse_Rejection{Rejection: &reservationsv1.Rejection{
			Code: "reservations/already-reserved", Message: "reservation is already confirmed",
		}}}, nil
	})
	f := newFixture(t, fake)

	rec := f.post(t, "/reservations/o-1/cancel", "")

	requireRejection(t, rec, http.StatusUnprocessableEntity, "reservations/already-reserved")
}

func TestContextStatusesMapToHTTP(t *testing.T) {
	cases := []struct {
		code   codes.Code
		status int
		want   string
	}{
		{codes.NotFound, http.StatusNotFound, "not-found"},
		{codes.Aborted, http.StatusConflict, "version-conflict"},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout, "deadline-exceeded"},
		{codes.Unavailable, http.StatusServiceUnavailable, "unavailable"},
		{codes.Internal, http.StatusInternalServerError, "internal-failure"},
	}
	for _, tc := range cases {
		t.Run(tc.code.String(), func(t *testing.T) {
			fake := (&fakeContexts{}).on("FindOrder", func(int) (any, error) {
				return nil, status.Error(tc.code, "detail the edge must not echo")
			})
			f := newFixture(t, fake)

			rec := f.do(t, http.MethodGet, "/orders/o-1", nil)

			requireRejection(t, rec, tc.status, tc.want)
			if strings.Contains(rec.Body.String(), "must not echo") {
				t.Fatalf("body %s leaked the context detail", rec.Body.String())
			}
		})
	}
}

func TestTheEdgeGrantsTheRetryBudgetOfARead(t *testing.T) {
	fake := (&fakeContexts{}).on("FindOrder", func(n int) (any, error) {
		if n == 1 {
			return nil, status.Error(codes.Unavailable, "restarting")
		}
		return &ordersv1.FindOrderResponse{Order: &ordersv1.Order{OrderId: "o-1", Status: ordersv1.OrderStatus_ORDER_STATUS_PLACED}}, nil
	})
	f := newFixture(t, fake)

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after one retry (body %s)", rec.Code, rec.Body.String())
	}
	if n := len(fake.callsTo("FindOrder")); n != 2 {
		t.Fatalf("FindOrder reached the context %d times, want 2 (RES-31)", n)
	}
}

func TestAnUnavailableReserveIs503WithoutRetry(t *testing.T) {
	fake := (&fakeContexts{}).on("Reserve", func(int) (any, error) {
		return nil, status.Error(codes.Unavailable, "restarting")
	})
	f := newFixture(t, fake)

	rec := f.post(t, "/reservations/o-1/reserve", `{"items":1}`)

	requireRejection(t, rec, http.StatusServiceUnavailable, "unavailable")
	if n := len(fake.callsTo("Reserve")); n != 1 {
		t.Fatalf("Reserve reached the context %d times, want 1 (GRP-09)", n)
	}
}

func TestALocallyExhaustedDeadlineIs504WithoutACall(t *testing.T) {
	tight := deadline.Budget{Dependency: "edge", Method: "route", Limit: 50 * time.Millisecond, Slack: 5 * time.Millisecond, EstimatedDuration: 10 * time.Millisecond}
	f := newFixture(t, &fakeContexts{}, withBudget(tight))

	rec := f.post(t, "/reservations/o-1/reserve", `{"items":1}`)

	requireRejection(t, rec, http.StatusGatewayTimeout, "deadline-exceeded")
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0 — the method slack does not fit the route deadline", n)
	}
}

func TestARequestWithoutDeadlineReachesTheContextBelowTheRouteBudget(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	started := time.Now()

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body.String())
	}
	calls := f.fake.callsTo("FindOrder")
	if len(calls) != 1 || !calls[0].hasDeadline {
		t.Fatalf("the context received no deadline (GRP-04): %+v", calls)
	}
	if budget := calls[0].deadline.Sub(started); budget >= routeBudget.Limit {
		t.Fatalf("the context got %v, want less than the route budget %v (GRP-17)", budget, routeBudget.Limit)
	}
}

func TestInvalidRequestsNeverCallAContext(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	cases := []struct {
		path, body, code string
	}{
		{"/orders/o-1/items", `{"sku":"A"`, "malformed-body"},
		{"/orders/o-1/items", `{"sku":"A","quantity":0}`, "invalid-request"},
		{"/orders/o-1/items", `{"sku":"A","quantity":1,"extra":true}`, "malformed-body"},
		{"/reservations/o-1/reserve", `{"items":0}`, "invalid-request"},
		{"/reservations/o%20o/cancel", "", "invalid-request"},
	}
	for _, tc := range cases {
		t.Run(tc.path+" "+tc.body, func(t *testing.T) {
			requireRejection(t, f.post(t, tc.path, tc.body), http.StatusBadRequest, tc.code)
		})
	}
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}

type unreadable struct{}

func (unreadable) Read([]byte) (int, error) { panic("the body was read before admission") }

func TestAdmissionRefusesWith429BeforeReadingTheBody(t *testing.T) {
	f := newFixture(t, &fakeContexts{}, withLimit(admission.Limit{PerSecond: 0.001, Burst: 1, Concurrency: 10}))

	if rec := f.do(t, http.MethodGet, "/reservations/o-1", nil); rec.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", rec.Code)
	}
	if rec := f.do(t, http.MethodGet, "/reservations/o-1", unreadable{}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429 (RES-17)", rec.Code)
	}
}

func TestTheEdgeSpanParentsTheClientSpanAndTheMetadata(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil, api.CorrelationHeader, "corr-1")

	if rec.Header().Get(api.CorrelationHeader) != "corr-1" {
		t.Fatalf("%s = %q, want corr-1 echoed", api.CorrelationHeader, rec.Header().Get(api.CorrelationHeader))
	}
	md := f.fake.callsTo("FindOrder")[0].md
	if got := md.Get("x-correlation-id"); len(got) != 1 || got[0] != "corr-1" {
		t.Fatalf("x-correlation-id = %v, want corr-1", got)
	}
	if got := md.Get("x-causation-id"); len(got) != 1 || got[0] == "" || got[0] == "corr-1" {
		t.Fatalf("x-causation-id = %v, want the edge request id", got)
	}

	var edge tracetest.SpanStub
	byID := map[string]tracetest.SpanStub{}
	for _, s := range f.spans.GetSpans() {
		byID[s.SpanContext.SpanID().String()] = s
		if s.Name == "HTTP GET /orders/{id}" {
			edge = s
		}
	}
	if !edge.SpanContext.IsValid() {
		t.Fatalf("no edge span among %v", f.spans.GetSpans())
	}
	traceparent := md.Get("traceparent")
	parts := strings.Split(strings.Join(traceparent, ""), "-")
	if len(parts) != 4 || parts[1] != edge.SpanContext.TraceID().String() {
		t.Fatalf("traceparent %v does not continue the edge trace", traceparent)
	}
	client, sent := byID[parts[2]]
	if !sent || !strings.HasPrefix(client.Name, "dmpf.grpc.client") {
		t.Fatalf("traceparent names %q, want a client span", client.Name)
	}
	for span := client; span.Parent.SpanID() != edge.SpanContext.SpanID(); {
		parent, known := byID[span.Parent.SpanID().String()]
		if !known {
			t.Fatalf("client span %q does not descend from the edge span (TRC-05)", client.Name)
		}
		span = parent
	}
}

func TestAnIncomingTraceparentIsContinued(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	const incoming = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	f.do(t, http.MethodGet, "/reservations/o-1", nil, "traceparent", incoming)

	for _, s := range f.spans.GetSpans() {
		if s.Name == "HTTP GET /reservations/{order_id}" {
			if s.SpanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
				t.Fatalf("edge trace = %s, want the incoming trace", s.SpanContext.TraceID())
			}
			return
		}
	}
	t.Fatal("no edge span recorded")
}

func TestAMalformedCorrelationIsReplaced(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil, api.CorrelationHeader, "bad correlation;")

	minted := rec.Header().Get(api.CorrelationHeader)
	if minted == "" || minted == "bad correlation;" {
		t.Fatalf("%s = %q, want a minted correlation", api.CorrelationHeader, minted)
	}
	if got := f.fake.callsTo("FindOrder")[0].md.Get("x-correlation-id"); len(got) != 1 || got[0] != minted {
		t.Fatalf("x-correlation-id = %v, want %q", got, minted)
	}
}

func TestTheContractsAreServedOnlyWhenProvided(t *testing.T) {
	silent := newFixture(t, &fakeContexts{})
	if rec := silent.do(t, http.MethodGet, api.OrdersContractPath, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("contract without document = %d, want 404", rec.Code)
	}

	f := newFixture(t, &fakeContexts{}, withContracts("orders: yes", "reservations: yes"))
	for path, want := range map[string]string{api.OrdersContractPath: "orders: yes", api.ReservationsContractPath: "reservations: yes"} {
		rec := f.do(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Fatalf("%s = %d %q, want 200 %q", path, rec.Code, rec.Body.String(), want)
		}
	}
}

func TestCORSAnswersThePreflightOfDeclaredOrigins(t *testing.T) {
	f := newFixture(t, &fakeContexts{}, withCORS("http://localhost:8082"))

	rec := f.do(t, http.MethodOptions, "/reservations/o-1/reserve", nil, "Origin", "http://localhost:8082", "Access-Control-Request-Method", "POST")

	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8082" {
		t.Fatalf("preflight = %d %v, want 204 with the origin allowed", rec.Code, rec.Header())
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), api.IdempotencyHeader) {
		t.Fatalf("Access-Control-Allow-Headers = %q, want %s", rec.Header().Get("Access-Control-Allow-Headers"), api.IdempotencyHeader)
	}

	other := f.do(t, http.MethodOptions, "/reservations/o-1/reserve", nil, "Origin", "http://evil.example")
	if other.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("an undeclared origin was allowed")
	}
}

func TestAnIdempotencyKeyTheWireRejectsNeverReachesAContext(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	// WHY: gRPC refuses metadata outside %x20-%x7E (grpc internal/metadata),
	// and net/http accepts those bytes, so an unvalidated header turns into a
	// permanent 500 the caller can trigger at will.
	for _, key := range []string{"abcé", "abc\tdef", strings.Repeat("k", 200), " "} {
		t.Run(fmt.Sprintf("%q", key), func(t *testing.T) {
			rec := f.do(t, http.MethodPost, "/reservations/o-1/cancel", nil,
				"Idempotency-Key", key, "Content-Type", "application/json")

			requireRejection(t, rec, http.StatusBadRequest, "invalid-idempotency-key")
		})
	}
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}
