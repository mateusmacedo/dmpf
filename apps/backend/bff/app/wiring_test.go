package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

type edge struct {
	addr     string
	admin    string
	logs     *memoryLogs
	provider *sdklog.LoggerProvider
	spans    *tracetest.InMemoryExporter
	metrics  *sdkmetric.ManualReader
	runtime  *otelboot.Runtime
	cancel   context.CancelFunc
	done     chan error
}

func startEdge(t *testing.T, env ...string) edge {
	t.Helper()
	base := append(slices.Clone(targets), "GRPC_INSECURE", "true", "HTTP_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0")
	cfg, err := app.FromEnv(lookup(append(base, env...)...))
	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	telemetry := app.TelemetryOf(cfg)
	e := edge{logs: &memoryLogs{}, spans: tracetest.NewInMemoryExporter(), metrics: sdkmetric.NewManualReader(), done: make(chan error, 1)}
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: telemetry.Service, ServiceVersion: "test",
			ServiceInstanceID: "bff-1", Role: telemetry.Role},
		TraceExporter: e.spans,
		MetricReader:  e.metrics,
	}
	e.provider = otelboot.NewLoggerProvider(config, e.logs)
	config.LoggerProvider = e.provider
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	if e.runtime, err = otelboot.Start(ctx, config); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = e.runtime.Shutdown(context.Background())
	})

	go func() { e.done <- app.RunWith(ctx, cfg, e.runtime) }()
	e.addr = listeningAddr(t, e, "http listening")
	e.admin = listeningAddr(t, e, "admin listening")
	return e
}

func (e edge) stop(t *testing.T) {
	t.Helper()
	e.cancel()
	if err := <-e.done; err != nil {
		t.Fatalf("RunWith() = %v, want nil on shutdown", err)
	}
	if err := e.provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
}

func (e edge) records(t *testing.T) []sdklog.Record {
	t.Helper()
	if err := e.provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	e.logs.mu.Lock()
	defer e.logs.mu.Unlock()
	return slices.Clone(e.logs.records)
}

func (e edge) request(t *testing.T, method, path string, headers ...string) int {
	t.Helper()
	return requestAt(t, e.addr, method, path, headers...)
}

func (e edge) probe(t *testing.T, method, path string) int {
	t.Helper()
	return requestAt(t, e.admin, method, path)
}

func requestAt(t *testing.T, addr, method, path string, headers ...string) int {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+addr+path, nil)
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if http.CanonicalHeaderKey(headers[i]) == "Host" {
			req.Host = headers[i+1]
			continue
		}
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s = %v", method, path, err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func attributesOf(record sdklog.Record) map[string]attribute.Value {
	attributes := map[string]attribute.Value{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		attributes[string(kv.Key)] = kv.Value
		return true
	})
	return attributes
}

func recordNamed(t *testing.T, records []sdklog.Record, body string) sdklog.Record {
	t.Helper()
	var bodies []string
	for _, record := range records {
		if record.Body().AsString() == body {
			return record
		}
		bodies = append(bodies, record.Body().AsString())
	}
	t.Fatalf("no %q record in %v", body, bodies)
	return sdklog.Record{}
}

func requireNoExecution(t *testing.T, body string, attributes map[string]attribute.Value) {
	t.Helper()
	for _, key := range []string{tracing.KeyCorrelationID, tracing.KeyRequestID, tracing.KeyTenantID} {
		if value, present := attributes[key]; present {
			t.Errorf("%q carries %s = %s, want no key of an execution on a start or shutdown record (RF-A4)", body, key, value.String())
		}
	}
}

func listeningAddr(t *testing.T, e edge, body string) string {
	t.Helper()
	var bodies []string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		bodies = bodies[:0]
		for _, record := range e.records(t) {
			bodies = append(bodies, record.Body().AsString())
			if record.Body().AsString() != body {
				continue
			}
			if scope := record.InstrumentationScope().Name; scope != reflect.TypeFor[app.Config]().PkgPath() {
				t.Fatalf("readiness line scope = %q, want the import path of the app package that emits it (RF-A1)", scope)
			}
			attributes := attributesOf(record)
			host, port := attributes[string(semconv.ServerAddressKey)], attributes[string(semconv.ServerPortKey)]
			if host.AsString() == "" || port.Type() != attribute.INT64 {
				t.Fatalf("readiness line = %v, want server.address and server.port surviving the processor (RF-A3)", attributes)
			}
			requireNoExecution(t, body, attributes)
			return net.JoinHostPort(host.AsString(), strconv.FormatInt(port.AsInt64(), 10))
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %q line in %v", body, bodies)
	return ""
}

func TestTheReadinessLineCarriesThePortTheKernelChose(t *testing.T) {
	e := startEdge(t, devMock, "true")

	if _, port, err := net.SplitHostPort(e.addr); err != nil || port == "0" || port == "" {
		t.Fatalf("addr = %q, want the address the listener resolved, not the configured one", e.addr)
	}
	conn, err := net.DialTimeout("tcp", e.addr, time.Second)
	if err != nil {
		t.Fatalf("dial %s = %v, want the edge accepting on the address it logged", e.addr, err)
	}
	_ = conn.Close()
	e.stop(t)
}

func TestTheShutdownLineCarriesTheDrainDelayUnderAPlatformKey(t *testing.T) {
	e := startEdge(t, devMock, "true", "DRAIN_DELAY", "10ms")

	e.stop(t)

	attributes := attributesOf(recordNamed(t, e.records(t), "http draining"))
	if got := attributes["dmpf.drain.delay"].AsString(); got != "10ms" {
		t.Fatalf("\"http draining\" = %v, want dmpf.drain.delay=10ms surviving the processor (RF-A3)", attributes)
	}
	requireNoExecution(t, "http draining", attributes)
}

func TestTheAdministrationPortRefusesReadinessWhileThePublicOneDrains(t *testing.T) {
	orders := serveOrders(t)
	e := startEdge(t, devMock, "true", "DRAIN_DELAY", "1s",
		"ORDERS_GRPC_TARGET", orders.target, "RESERVATIONS_GRPC_TARGET", orders.target, "BOOKINGS_GRPC_TARGET", orders.target)
	readiness := func(want int) bool {
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if e.probe(t, http.MethodGet, "/readyz") == want {
				return true
			}
		}
		return false
	}

	if !readiness(http.StatusNoContent) {
		t.Fatal("/readyz on the administration port never answered 204 with the contexts serving")
	}
	e.cancel()
	if !readiness(http.StatusServiceUnavailable) {
		t.Fatal("/readyz on the administration port never answered 503 during the drain")
	}
	if status := e.request(t, http.MethodGet, "/orders/o-1"); status != http.StatusUnauthorized {
		t.Fatalf("public GET /orders/o-1 during the drain = %d, want it still served (401)", status)
	}
	e.stop(t)
}

func TestTheAdministrationPortClosesOnlyAfterThePublicOneHasDrained(t *testing.T) {
	arrived, release := make(chan struct{}, 1), make(chan struct{})
	orders := serveOrdersHeldBy(t, func(ctx context.Context) {
		select {
		case arrived <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
	})
	e := startEdge(t, devMock, "true",
		"ORDERS_GRPC_TARGET", orders.target, "RESERVATIONS_GRPC_TARGET", orders.target, "BOOKINGS_GRPC_TARGET", orders.target)
	req, err := http.NewRequest(http.MethodGet, "http://"+e.addr+"/orders/o-1", nil)
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	req.Header.Set("Authorization", degradedCredential)
	answered := make(chan error, 1)
	go func() {
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				err = errors.New(res.Status)
			}
		}
		answered <- err
	}()
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the public GET /orders/o-1 never reached the orders context")
	}

	e.cancel()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		conn, err := net.DialTimeout("tcp", e.addr, time.Second)
		if err != nil {
			break
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("the public port still accepts connections 5s after the shutdown began")
		}
	}
	res, err := http.Get("http://" + e.admin + "/readyz")
	if err != nil {
		t.Fatalf("/readyz on the administration port while the public one drains its request in flight = %v, want 503: the administration port closes only after the public one", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("/readyz on the administration port while the public one drains = %d, want 503", res.StatusCode)
	}
	close(release)
	if err := <-answered; err != nil {
		t.Fatalf("public GET /orders/o-1 in flight when the shutdown began = %v, want 200: the public port drains it before closing", err)
	}
	e.stop(t)
}

func TestARefusedTokenReachesTheLoggerProviderOfTheRuntime(t *testing.T) {
	issuer := httptest.NewServer(nil)
	t.Cleanup(issuer.Close)
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": issuer.URL, "jwks_uri": issuer.URL + "/jwks",
			"authorization_endpoint": issuer.URL + "/auth", "token_endpoint": issuer.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	issuer.Config.Handler = mux
	e := startEdge(t, "OIDC_ISSUER", issuer.URL, "OIDC_AUDIENCE", "dmpf-bff", "OIDC_TENANT_CLAIM", "tenant_id")

	if status := e.request(t, http.MethodGet, "/orders/o-1", "Authorization", "Bearer not-a-jwt"); status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	e.stop(t)

	record := recordNamed(t, e.records(t), "auth: token rejected")
	if scope := record.InstrumentationScope().Name; scope != "github.com/mateusmacedo/dmpf/libs/backend/go/authn" {
		t.Fatalf("refusal scope = %q, want the import path of authn (RF-A1)", scope)
	}
	attributes := attributesOf(record)
	if got := attributes[string(semconv.ErrorTypeKey)].AsString(); got != "Unauthenticated" {
		t.Fatalf("refusal error.type = %q, want the FND-07 category Unauthenticated (RF-B1)", got)
	}
	if got := attributes["dmpf.auth.refusal_reason"].AsString(); got != "malformed" {
		t.Fatalf("refusal dmpf.auth.refusal_reason = %q, want malformed (RF-A6)", got)
	}
}

func TestEveryRequestOfTheEdgeIsOneServerSpanAndOnePointOfItsDuration(t *testing.T) {
	contract := filepath.Join(t.TempDir(), "orders.yaml")
	if err := os.WriteFile(contract, []byte("openapi: 3.1.0\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	e := startEdge(t, devMock, "true", "CORS_ORIGINS", "http://app.example", "OPENAPI_ORDERS_PATH", contract)
	requests := []struct {
		method, path string
		headers      []string
		status       int
		span         string
		admin        bool
	}{
		{http.MethodGet, "/orders/o-1", nil, http.StatusUnauthorized, "GET /orders/{id}", false},
		{http.MethodGet, "/openapi/orders/v1/openapi.yaml", nil, http.StatusOK, "GET /openapi/orders/v1/openapi.yaml", false},
		{http.MethodGet, "/livez", nil, http.StatusNotFound, "GET", false},
		{http.MethodGet, "/readyz", nil, http.StatusNotFound, "GET", false},
		{http.MethodGet, "/livez", nil, http.StatusNoContent, "", true},
		{http.MethodGet, "/readyz", nil, http.StatusServiceUnavailable, "", true},
		{http.MethodGet, "/nowhere", nil, http.StatusNotFound, "GET", true},
		{http.MethodOptions, "/orders/o-1/place", []string{"Origin", "http://app.example", "Access-Control-Request-Method", "POST"}, http.StatusNoContent, "OPTIONS", false},
		{http.MethodGet, "/nowhere", nil, http.StatusNotFound, "GET", false},
		{http.MethodDelete, "/orders/o-1", nil, http.StatusMethodNotAllowed, "DELETE", false},
	}
	var want []string
	for _, r := range requests {
		addr := map[bool]string{false: e.addr, true: e.admin}[r.admin]
		if status := requestAt(t, addr, r.method, r.path, r.headers...); status != r.status {
			t.Fatalf("%s %s on %s = %d, want %d", r.method, r.path, addr, status, r.status)
		}
		if r.span != "" {
			want = append(want, r.span)
		}
	}
	if err := e.runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	var got []string
	for _, span := range e.spans.GetSpans() {
		if span.SpanKind == trace.SpanKindServer {
			got = append(got, span.Name)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("SERVER spans = %v, want exactly one per request outside the health routes %v (RF-B2)", got, want)
	}
	if points := durationPoints(t, e.metrics); points != uint64(len(want)) {
		t.Fatalf("http.server.request.duration counted %d requests, want %d, none of the health routes (RF-B2, RF-D2)", points, len(want))
	}
	e.stop(t)
}

func TestAnEdgeSeriesDoesNotSplitByTheHostTheClientSends(t *testing.T) {
	e := startEdge(t, devMock, "true")
	for _, host := range []string{"a.example", "b.example", "c.example:1234", "c.example:4321"} {
		if status := e.request(t, http.MethodGet, "/orders/o-1", "Host", host); status != http.StatusUnauthorized {
			t.Fatalf("GET /orders/o-1 with Host %s = %d, want 401", host, status)
		}
	}
	if err := e.runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	for _, name := range []string{
		httpconv.ServerRequestDuration{}.Name(),
		httpconv.ServerRequestBodySize{}.Name(),
		httpconv.ServerResponseBodySize{}.Name(),
	} {
		series := routeSeries(t, e.metrics, name, "/orders/{id}")
		if len(series) != 1 || series[0].HasValue(semconv.ServerAddressKey) || series[0].HasValue(semconv.ServerPortKey) {
			encoded := make([]string, len(series))
			for i, set := range series {
				encoded[i] = set.Encoded(attribute.DefaultEncoder())
			}
			t.Errorf("%s for /orders/{id} = %q, want one series without server.address or server.port, "+
				"which otelhttp takes from the Host the client sends (RF-D3)", name, encoded)
		}
	}
	e.stop(t)
}

func routeSeries(t *testing.T, reader *sdkmetric.ManualReader, name, route string) []attribute.Set {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var sets []attribute.Set
	keep := func(set attribute.Set) {
		if value, ok := set.Value(semconv.HTTPRouteKey); ok && value.AsString() == route {
			sets = append(sets, set)
		}
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					keep(point.Attributes)
				}
			case metricdata.Histogram[int64]:
				for _, point := range data.DataPoints {
					keep(point.Attributes)
				}
			default:
				t.Fatalf("%s is %T, want a histogram", name, m.Data)
			}
		}
	}
	return sets
}

func durationPoints(t *testing.T, reader *sdkmetric.ManualReader) uint64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var count uint64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if histogram, ok := m.Data.(metricdata.Histogram[float64]); ok && m.Name == "http.server.request.duration" {
				for _, point := range histogram.DataPoints {
					count += point.Count
				}
			}
		}
	}
	return count
}
