package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

// countingBody counts the bytes the handler side reads, so a refusal proves it
// never touched the body.
type countingBody struct {
	io.Reader
	read atomic.Int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read.Add(int64(n))
	return n, err
}

func (b *countingBody) Close() error { return nil }

func admissionController(t *testing.T, limit admission.Limit) *admission.Controller {
	t.Helper()
	tenants, err := metrics.DeclareTenants("acme")
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := admission.New(admission.Config{
		Limits:  map[string]admission.Limit{"POST /v1/orders": limit},
		Tenants: tenants,
		MaxKeys: 10,
		Clock:   clock.NewFake(start),
	})
	if err != nil {
		t.Fatal(err)
	}
	return ctrl
}

func tenantHeader(r *http.Request) string { return r.Header.Get("X-Tenant") }

func post(tenant string, body *countingBody) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", nil)
	if body != nil {
		req.Body = body
	}
	if tenant != "" {
		req.Header.Set("X-Tenant", tenant)
	}
	return req
}

func TestAdmissionRefusesWith429BeforeReadingTheBody(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}

	handled := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handled++
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	})
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 10})
	middleware := provider.Admission(ctrl, nil, tenantHeader, instruments)(handler)

	first := &countingBody{Reader: strings.NewReader(strings.Repeat("x", 1<<20))}
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, post("initech", first))
	if rec.Code != http.StatusCreated || handled != 1 {
		t.Fatalf("first request = %d (handled %d), want 201 handled once", rec.Code, handled)
	}

	second := &countingBody{Reader: strings.NewReader(strings.Repeat("x", 1<<20))}
	rec = httptest.NewRecorder()
	middleware.ServeHTTP(rec, post("initech", second))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429 (RES-17)", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	if handled != 1 {
		t.Fatalf("handler ran %d times, want 1: the refusal comes before the handler", handled)
	}
	if second.read.Load() != 0 {
		t.Fatalf("the refused request had %d body bytes read, want 0 (RES-17)", second.read.Load())
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metrics.AdmissionRejectionsTotal {
				continue
			}
			found = true
			sum := m.Data.(metricdata.Sum[int64])
			if len(sum.DataPoints) != 1 {
				t.Fatalf("data points = %d, want 1", len(sum.DataPoints))
			}
			route, _ := sum.DataPoints[0].Attributes.Value(metrics.KeyRoute)
			tenant, _ := sum.DataPoints[0].Attributes.Value(metrics.KeyTenant)
			if route.AsString() != "POST /v1/orders" || tenant.AsString() != metrics.OtherTenant {
				t.Fatalf("labels route=%q tenant=%q, want the route key and %q (MET-12)", route.AsString(), tenant.AsString(), metrics.OtherTenant)
			}
		}
	}
	if !found {
		t.Fatalf("series %q not recorded", metrics.AdmissionRejectionsTotal)
	}
}

// RES-16: each tenant owns its bucket, declared in the label allowlist or not;
// the allowlist only collapses the metric label (MET-07).
func TestAdmissionKeepsABucketPerTenantWhateverTheAllowlist(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 10})
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	middleware := provider.Admission(ctrl, nil, tenantHeader, nil)(ok)

	serve := func(tenant string) int {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, post(tenant, nil))
		return rec.Code
	}
	if serve("acme") != http.StatusNoContent {
		t.Fatal("acme refused on its first request")
	}
	if serve("initech") != http.StatusNoContent {
		t.Fatal("initech refused on its first request: an undeclared tenant still owns its bucket")
	}
	if serve("umbrella") != http.StatusNoContent {
		t.Fatal("umbrella refused because initech used its bucket: the buckets are per tenant")
	}
	if serve("acme") != http.StatusTooManyRequests {
		t.Fatal("acme admitted twice within the burst")
	}
}

func TestAdmissionAnswers404ForARouteWithoutLimit(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1})
	handled := false
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { handled = true })
	middleware := provider.Admission(ctrl, nil, nil, nil)(handler)

	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/unknown", nil))
	if rec.Code != http.StatusNotFound || handled {
		t.Fatalf("code = %d handled = %v, want 404 and no handler (RES-16)", rec.Code, handled)
	}
}

func TestAdmissionReleasesTheSlotAfterTheHandler(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 1})
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	middleware := provider.Admission(ctrl, nil, nil, nil)(ok)

	for range 3 {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, post("", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("sequential request = %d, want 204: the slot is released after the handler", rec.Code)
		}
	}
}

func TestAdmissionUsesTheInjectedRouteFunc(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1})
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	byPattern := func(*http.Request) string { return "POST /v1/orders" }
	middleware := provider.Admission(ctrl, byPattern, nil, nil)(ok)

	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/orders/12345/items", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204: the route func maps the concrete path to the declared key", rec.Code)
	}
}

func TestAnUndeclaredRouteIsCountedUnderOneLabelNotItsPath(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("t"))
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := admission.New(admission.Config{
		Limits: map[string]admission.Limit{"POST /v1/orders": {PerSecond: 1, Burst: 1, Concurrency: 1}}, MaxKeys: 8, Clock: clock.NewFake(start),
	})
	if err != nil {
		t.Fatal(err)
	}
	middleware := provider.Admission(ctrl, nil, nil, instruments)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for _, path := range []string{"/aaa1", "/aaa2", "/aaa3"} {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", path, rec.Code)
		}
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metrics.AdmissionRejectionsTotal {
				continue
			}
			sum := m.Data.(metricdata.Sum[int64])
			if len(sum.DataPoints) != 1 || sum.DataPoints[0].Value != 3 {
				t.Fatalf("data points = %d, want one series with 3 refusals (MET-07)", len(sum.DataPoints))
			}
			route, _ := sum.DataPoints[0].Attributes.Value(metrics.KeyRoute)
			if route.AsString() != provider.UndeclaredRouteLabel {
				t.Fatalf("route = %q, want %q", route.AsString(), provider.UndeclaredRouteLabel)
			}
			return
		}
	}
	t.Fatalf("series %q not recorded", metrics.AdmissionRejectionsTotal)
}
