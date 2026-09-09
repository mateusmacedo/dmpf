package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/apps/backend/dmpf-reference/api"
	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	obsclock "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfhttp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-http"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/admission"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
)

const (
	orderID   = "o-100"
	itemLimit = 2
)

var budget = deadline.Budget{Dependency: "postgres", Method: "orders", Limit: 2 * time.Second, Slack: 200 * time.Millisecond, EstimatedDuration: 100 * time.Millisecond}

// failingSave wraps the in-memory repository so a test reaches the 409 branch:
// the store never produces a conflict on its own.
type failingSave struct {
	dmpfports.Repository[orders.OrderID, orders.Snapshot]
	err error
}

func (f failingSave) Save(context.Context, orders.OrderID, orders.Snapshot, dmpfports.Version) error {
	return f.err
}

type fixture struct {
	store   *memory.Store
	handler http.Handler
	spans   *tracetest.InMemoryExporter
}

type option func(*setup)

type setup struct {
	saveErr error
	limit   admission.Limit
	openAPI []byte
	cors    []string
}

func withSaveError(err error) option { return func(s *setup) { s.saveErr = err } }

func withLimit(limit admission.Limit) option { return func(s *setup) { s.limit = limit } }

func withOpenAPI(document string) option { return func(s *setup) { s.openAPI = []byte(document) } }

func withCORS(origins ...string) option { return func(s *setup) { s.cors = origins } }

func newFixture(t *testing.T, options ...option) fixture {
	t.Helper()

	cfg := &setup{limit: admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 100}}
	for _, apply := range options {
		apply(cfg)
	}

	store := memory.New()
	bind := func(tx *memory.Tx) ordersapp.Resources {
		res := ordersapp.Resources{Orders: tx.Orders(), Outbox: tx.Outbox()}
		if cfg.saveErr != nil {
			res.Orders = failingSave{Repository: res.Orders, err: cfg.saveErr}
		}
		return res
	}
	service := ordersapp.Service{
		UoW:       memory.NewUnitOfWork(store, bind),
		Reader:    store.Reader(),
		Clock:     memory.FixedClock{At: 1_755_432_000_000_000_000},
		IDs:       &memory.SequenceIDs{Prefix: "m-"},
		Authorize: dmpfapplication.AllowAll[ordersapp.Command](),
		ItemLimit: itemLimit,
	}

	tenants, err := metrics.DeclareTenants(api.Tenant)
	if err != nil {
		t.Fatalf("DeclareTenants() = %v", err)
	}
	ctrl, err := admission.New(admission.Config{Limits: api.Limits(cfg.limit), Tenants: tenants, MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}

	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	handler, err := api.NewHandler(service, ctrl, provider.Tracer("api_test"), nil, api.Options{Budget: budget, OpenAPI: cfg.openAPI, CORSOrigins: cfg.cors})
	if err != nil {
		t.Fatalf("NewHandler() = %v", err)
	}
	return fixture{store: store, handler: handler, spans: spans}
}

func (f fixture) do(t *testing.T, method, path string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

func (f fixture) addItem(t *testing.T, order, sku string, quantity int) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"sku":"` + sku + `","quantity":` + itoa(quantity) + `}`)
	return f.do(t, http.MethodPost, "/orders/"+order+"/items", body, "Idempotency-Key", "k-"+sku, "Content-Type", "application/json")
}

func (f fixture) place(t *testing.T, order string) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, http.MethodPost, "/orders/"+order+"/place", nil, "Idempotency-Key", "k-place")
}

func itoa(n int) string { return string(rune('0' + n)) }

func decode(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
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

func TestRoutesReferenceTheContractAndDeclareIdempotencyOnPosts(t *testing.T) {
	routes := api.Routes(budget)
	if len(routes) != 3 {
		t.Fatalf("Routes() has %d routes, want 3", len(routes))
	}
	for _, route := range routes {
		if err := route.Validate(); err != nil {
			t.Fatalf("%s: Validate() = %v", route.Name, err)
		}
		if !strings.HasPrefix(route.ContractRef, "contracts/openapi/orders/v1/openapi.yaml#/paths/") {
			t.Fatalf("%s: ContractRef = %q, want a pointer into the published OpenAPI (RST-04)", route.Name, route.ContractRef)
		}
		if route.Method == http.MethodPost && route.IdempotencyKey != "Idempotency-Key" {
			t.Fatalf("%s: IdempotencyKey = %q, want Idempotency-Key on every POST (RST-02)", route.Name, route.IdempotencyKey)
		}
		if route.Method == http.MethodPost && !route.Idempotent() {
			t.Fatalf("%s: a POST with a declared key must be idempotent", route.Name)
		}
	}

	stripped := routes[0]
	stripped.ContractRef = ""
	if err := stripped.Validate(); !errors.Is(err, dmpfhttp.ErrContractRequired) {
		t.Fatalf("Validate() without ContractRef = %v, want ErrContractRequired", err)
	}
}

func TestLimitsCoverEveryRouteByItsPattern(t *testing.T) {
	limit := admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1}
	limits := api.Limits(limit)
	for _, route := range api.Routes(budget) {
		got, ok := limits[route.Method+" "+route.Path]
		if !ok || got != limit {
			t.Fatalf("Limits() lacks %q: %v", route.Method+" "+route.Path, limits)
		}
	}
	if len(limits) != 3 {
		t.Fatalf("Limits() has %d entries, want 3", len(limits))
	}
}

func TestPostWithoutIdempotencyKeyIsRefusedBeforeTheService(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodPost, "/orders/"+orderID+"/items", strings.NewReader(`{"sku":"A","quantity":1}`))

	requireRejection(t, rec, http.StatusBadRequest, "missing-idempotency-key")
	if got := f.store.WithinCalls(); got != 0 {
		t.Fatalf("the service opened %d transactions, want 0 — the refusal happens at the edge", got)
	}
}

func TestAddItemAcceptedIs201(t *testing.T) {
	f := newFixture(t)

	rec := f.addItem(t, orderID, "A", 1)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Order string `json:"order"`
		Items int    `json:"items"`
	}
	decode(t, rec, &body)
	if body.Order != orderID || body.Items != 1 {
		t.Fatalf("body = %+v, want {order: %s, items: 1}", body, orderID)
	}
}

func TestAddItemRejectedByTheDomainIs422(t *testing.T) {
	f := newFixture(t)
	for _, sku := range []string{"A", "B"} {
		if rec := f.addItem(t, orderID, sku, 1); rec.Code != http.StatusCreated {
			t.Fatalf("setup: status = %d", rec.Code)
		}
	}

	rec := f.addItem(t, orderID, "C", 1)

	requireRejection(t, rec, http.StatusUnprocessableEntity, string(orders.CodeItemLimitExceeded))
}

func TestPlaceOrderIs200ThenNotOpenIs422(t *testing.T) {
	f := newFixture(t)
	if rec := f.addItem(t, orderID, "A", 1); rec.Code != http.StatusCreated {
		t.Fatalf("setup: status = %d", rec.Code)
	}

	placed := f.place(t, orderID)
	if placed.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", placed.Code, placed.Body.String())
	}
	var body struct {
		Order string `json:"order"`
	}
	decode(t, placed, &body)
	if body.Order != orderID {
		t.Fatalf("body = %+v, want {order: %s}", body, orderID)
	}

	requireRejection(t, f.place(t, orderID), http.StatusUnprocessableEntity, string(orders.CodeOrderNotOpen))
}

func TestUnknownOrderIs404(t *testing.T) {
	f := newFixture(t)

	requireRejection(t, f.place(t, "o-absent"), http.StatusNotFound, "not-found")
	requireRejection(t, f.do(t, http.MethodGet, "/orders/o-absent", nil), http.StatusNotFound, "not-found")
}

func TestVersionConflictIs409(t *testing.T) {
	f := newFixture(t, withSaveError(dmpfports.ErrVersionConflict))

	requireRejection(t, f.addItem(t, orderID, "A", 1), http.StatusConflict, "version-conflict")
}

func TestMalformedBodyIs400(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodPost, "/orders/"+orderID+"/items", strings.NewReader(`{"sku":`), "Idempotency-Key", "k")

	requireRejection(t, rec, http.StatusBadRequest, "malformed-body")
}

func TestFindOrderReadsTheSnapshot(t *testing.T) {
	f := newFixture(t)
	if rec := f.addItem(t, orderID, "A", 2); rec.Code != http.StatusCreated {
		t.Fatalf("setup: status = %d", rec.Code)
	}

	rec := f.do(t, http.MethodGet, "/orders/"+orderID, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		ItemLimit int    `json:"itemLimit"`
		Items     []struct {
			SKU      string `json:"sku"`
			Quantity int    `json:"quantity"`
		} `json:"items"`
	}
	decode(t, rec, &body)
	if body.ID != orderID || body.Status != "open" || body.ItemLimit != itemLimit || len(body.Items) != 1 || body.Items[0].SKU != "A" || body.Items[0].Quantity != 2 {
		t.Fatalf("body = %+v", body)
	}
}

// unreadable fails the test if anything reads it: admission decides before
// the body is touched (RES-17).
type unreadable struct{ t *testing.T }

func (u unreadable) Read([]byte) (int, error) {
	u.t.Fatal("the body was read before admission decided (RES-17)")
	return 0, io.EOF
}

func TestAdmissionRefusesWith429BeforeReadingTheBody(t *testing.T) {
	f := newFixture(t, withLimit(admission.Limit{PerSecond: 0.001, Burst: 1, Concurrency: 1}))
	if rec := f.addItem(t, orderID, "A", 1); rec.Code != http.StatusCreated {
		t.Fatalf("first request: status = %d, want 201 (the burst admits one)", rec.Code)
	}

	rec := f.do(t, http.MethodPost, "/orders/"+orderID+"/items", unreadable{t: t}, "Idempotency-Key", "k-2")

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestTheServiceReceivesAMessageContextAuthoredAtTheEdge(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodPost, "/orders/"+orderID+"/items", strings.NewReader(`{"sku":"A","quantity":1}`),
		"Idempotency-Key", "k", "X-Correlation-ID", "corr-42")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body.String())
	}

	entries := f.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() = %d, want 1", len(entries))
	}
	mc := entries[0].Context
	if mc.CorrelationID != "corr-42" {
		t.Fatalf("correlationid = %q, want the X-Correlation-ID header", mc.CorrelationID)
	}
	if mc.CausationID != "m-000001" {
		t.Fatalf("causationid = %q, want the message's own id (the chain starts here)", mc.CausationID)
	}
	if !strings.HasPrefix(mc.Traceparent, "00-") {
		t.Fatalf("traceparent = %q, want a W3C traceparent of the server span", mc.Traceparent)
	}
	spans := f.spans.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no span was recorded for the request")
	}
	var traceID trace.TraceID
	for _, span := range spans {
		if span.SpanKind == trace.SpanKindServer {
			traceID = span.SpanContext.TraceID()
		}
	}
	if !strings.Contains(mc.Traceparent, traceID.String()) {
		t.Fatalf("traceparent %q does not carry the server span's trace %s", mc.Traceparent, traceID)
	}
}

func TestACorrelationIsMintedWhenTheClientSendsNone(t *testing.T) {
	f := newFixture(t)

	if rec := f.addItem(t, orderID, "A", 1); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}

	mc := f.store.Entries()[0].Context
	if len(mc.CorrelationID) != 32 {
		t.Fatalf("correlationid = %q, want 16 random bytes in hex", mc.CorrelationID)
	}
}

// The client's correlation is persisted in outbox metadata and travels in every
// envelope of the chain: what does not fit the published shape is replaced,
// never propagated.
func TestAMalformedCorrelationIsReplacedNotPropagated(t *testing.T) {
	cases := map[string]string{
		"too long":      strings.Repeat("c", 129),
		"bad character": "corr 42",
		"control byte":  "corr\x7f",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			rec := f.do(t, http.MethodPost, "/orders/"+orderID+"/items", strings.NewReader(`{"sku":"A","quantity":1}`),
				"Idempotency-Key", "k", "X-Correlation-ID", header)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d (body %s)", rec.Code, rec.Body.String())
			}

			mc := f.store.Entries()[0].Context
			if mc.CorrelationID == header || len(mc.CorrelationID) != 32 {
				t.Fatalf("correlationid = %q, want a minted one in place of %q", mc.CorrelationID, header)
			}
			if got := rec.Header().Get("X-Correlation-ID"); got != mc.CorrelationID {
				t.Fatalf("response header = %q, want the correlation actually used, %q", got, mc.CorrelationID)
			}
		})
	}
}

// What the contract does not publish is refused before the service runs: the
// domain does not validate quantity or SKU, so the edge holds the line the
// OpenAPI draws (minimum 1, minLength 1, maxLength 128).
func TestARequestOutsideTheContractIs400(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"quantity zero", `{"sku":"A","quantity":0}`, "invalid-request"},
		{"quantity negative", `{"sku":"A","quantity":-1}`, "invalid-request"},
		{"empty sku", `{"sku":"","quantity":1}`, "invalid-request"},
		{"sku too long", `{"sku":"` + strings.Repeat("A", 129) + `","quantity":1}`, "invalid-request"},
		{"unknown field", `{"sku":"A","quantity":1,"price":9}`, "malformed-body"},
		{"trailing data", `{"sku":"A","quantity":1} garbage`, "malformed-body"},
		{"two objects", `{"sku":"A","quantity":1}{"sku":"B","quantity":1}`, "malformed-body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)

			rec := f.do(t, http.MethodPost, "/orders/"+orderID+"/items", strings.NewReader(tc.body), "Idempotency-Key", "k")

			requireRejection(t, rec, http.StatusBadRequest, tc.code)
			if got := f.store.WithinCalls(); got != 0 {
				t.Fatalf("the service opened %d transactions, want 0", got)
			}
		})
	}
}

func TestAnOrderIDOutsideTheContractIs400(t *testing.T) {
	cases := map[string]string{
		"space":    "a%20b",
		"too long": strings.Repeat("x", 129),
		"slash":    "a%2Fb",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			requireRejection(t, f.do(t, http.MethodGet, "/orders/"+id, nil), http.StatusBadRequest, "invalid-request")
			requireRejection(t, f.do(t, http.MethodPost, "/orders/"+id+"/place", nil, "Idempotency-Key", "k"), http.StatusBadRequest, "invalid-request")
			if got := f.store.WithinCalls(); got != 0 {
				t.Fatalf("the service opened %d transactions, want 0", got)
			}
		})
	}
}

func TestJSONResponsesForbidSniffing(t *testing.T) {
	f := newFixture(t)

	for _, rec := range []*httptest.ResponseRecorder{
		f.addItem(t, orderID, "A", 1),
		f.do(t, http.MethodGet, "/orders/o-absent", nil),
		f.do(t, http.MethodPost, "/orders/"+orderID+"/place", nil),
	} {
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("status %d: X-Content-Type-Options = %q, want nosniff", rec.Code, got)
		}
	}
}

func TestTheContractIsServedOnlyWhenProvided(t *testing.T) {
	const document = "openapi: 3.1.0\ninfo:\n  title: t\n"

	with := newFixture(t, withOpenAPI(document))
	rec := with.do(t, http.MethodGet, api.OpenAPIPath, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != document {
		t.Fatalf("GET %s = %d %q, want 200 with the document", api.OpenAPIPath, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Fatalf("Content-Type = %q, want application/yaml", ct)
	}

	without := newFixture(t)
	if rec := without.do(t, http.MethodGet, api.OpenAPIPath, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("GET %s without a document = %d, want 404", api.OpenAPIPath, rec.Code)
	}
}

// CORS is off unless the composition root names origins; with them, only the
// named origin gets the headers and the preflight, and it never widens to "*".
func TestCORSIsOffByDefaultAndScopedToTheDeclaredOrigins(t *testing.T) {
	const allowed = "http://localhost:8082"

	off := newFixture(t)
	rec := off.do(t, http.MethodOptions, "/orders/"+orderID+"/items", nil, "Origin", allowed, "Access-Control-Request-Method", "POST")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("off: preflight = %d with ACAO %q, want 405 and no CORS header", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}

	on := newFixture(t, withCORS(allowed))
	rec = on.do(t, http.MethodOptions, "/orders/"+orderID+"/items", nil, "Origin", allowed, "Access-Control-Request-Method", "POST")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowed {
		t.Fatalf("ACAO = %q, want %q", got, allowed)
	}
	for _, header := range []string{"Content-Type", "Idempotency-Key", "X-Correlation-ID"} {
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), header) {
			t.Fatalf("Access-Control-Allow-Headers = %q lacks %s", rec.Header().Get("Access-Control-Allow-Headers"), header)
		}
	}
	if got := on.store.WithinCalls(); got != 0 {
		t.Fatalf("the preflight reached the service (%d transactions)", got)
	}

	rec = on.do(t, http.MethodPost, "/orders/"+orderID+"/items", strings.NewReader(`{"sku":"A","quantity":1}`),
		"Origin", allowed, "Idempotency-Key", "k")
	if rec.Code != http.StatusCreated || rec.Header().Get("Access-Control-Allow-Origin") != allowed {
		t.Fatalf("actual request = %d with ACAO %q, want 201 and the origin echoed", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Expose-Headers"), "X-Correlation-ID") {
		t.Fatalf("Expose-Headers = %q, want the correlation exposed to the browser", rec.Header().Get("Access-Control-Expose-Headers"))
	}

	rec = on.do(t, http.MethodGet, "/orders/"+orderID, nil, "Origin", "http://evil.example")
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("an undeclared origin received ACAO %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
