package api_test

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
	ordersv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
)

func serverSpan(t *testing.T, f fixture, name string) tracetest.SpanStub {
	t.Helper()
	var found []tracetest.SpanStub
	for _, s := range f.spans.GetSpans() {
		if s.SpanKind == trace.SpanKindServer {
			found = append(found, s)
		}
	}
	if len(found) != 1 || found[0].Name != name {
		names := make([]string, len(found))
		for i, s := range found {
			names[i] = s.Name
		}
		t.Fatalf("SERVER spans = %v, want one %q", names, name)
	}
	return found[0]
}

func attributesOf(span tracetest.SpanStub) map[attribute.Key]attribute.Value {
	values := make(map[attribute.Key]attribute.Value, len(span.Attributes))
	for _, kv := range span.Attributes {
		values[kv.Key] = kv.Value
	}
	return values
}

func TestTheServerSpanIsTheOneOfOtelhttpWithTheExecution(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil, api.CorrelationHeader, "corr-1", "Accept", "application/json", "User-Agent", "probe/1.0", "Cookie", "session=s")

	span := serverSpan(t, f, "GET /orders/{id}")
	values := attributesOf(span)
	for key, want := range map[attribute.Key]string{
		"dmpf.correlation_id":   "corr-1",
		"dmpf.tenant_id":        testTenant,
		"dmpf.outcome_category": "accepted",
		"http.route":            "/orders/{id}",
		"url.path":              "/orders/{id}",
	} {
		if got := values[key].String(); got != want {
			t.Fatalf("SERVER %s = %q, want %q (attributes %v)", key, got, want, span.Attributes)
		}
	}
	if values["dmpf.request_id"].AsString() == "" {
		t.Fatalf("SERVER attributes = %v, want the dmpf.request_id of the execution", span.Attributes)
	}
	if got := values["http.request.header.accept"].AsStringSlice(); !slices.Equal(got, []string{"application/json"}) {
		t.Fatalf("SERVER http.request.header.accept = %v, want [application/json]", got)
	}
	for key := range values {
		switch {
		case key == "client.address", key == "user_agent.original", key == "network.peer.address":
			t.Fatalf("SERVER exported %s: the privacy exporter removes it (RF-B3)", key)
		case strings.HasPrefix(string(key), "http.request.header.") && key != "http.request.header.accept" && key != "http.request.header.content-type":
			t.Fatalf("SERVER exported %s: only content-type and accept are allowed (RF-B2)", key)
		}
	}
	if span.Status.Code != codes.Unset {
		t.Fatalf("SERVER status = %v, want Unset", span.Status)
	}
}

func TestARefusalLeavesTheServerSpanUnsetWithItsOutcome(t *testing.T) {
	notFound := (&fakeContexts{}).on("FindOrder", func(int) (any, error) { return nil, status.Error(grpccodes.NotFound, "no order") })
	for name, c := range map[string]struct {
		fake    *fakeContexts
		headers []string
		outcome string
	}{
		"denied":   {&fakeContexts{}, []string{"Authorization", ""}, "denied"},
		"rejected": {notFound, nil, "rejected"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, c.fake)

			rec := f.do(t, http.MethodGet, "/orders/o-1", nil, c.headers...)
			if rec.Code < http.StatusBadRequest || rec.Code >= http.StatusInternalServerError {
				t.Fatalf("status = %d, want a 4xx", rec.Code)
			}

			span := serverSpan(t, f, "GET /orders/{id}")
			values := attributesOf(span)
			if span.Status.Code != codes.Unset || values["dmpf.outcome_category"].AsString() != c.outcome {
				t.Fatalf("SERVER status = %v, outcome = %q; want Unset and %q", span.Status, values["dmpf.outcome_category"].AsString(), c.outcome)
			}
			if _, typed := values["error.type"]; typed {
				t.Fatalf("SERVER attributes = %v, want no error.type in a 4xx", span.Attributes)
			}
		})
	}
}

func TestAServerFailureMarksTheServerSpan(t *testing.T) {
	fake := (&fakeContexts{}).on("FindOrder", func(int) (any, error) { return nil, status.Error(grpccodes.Internal, "boom") })
	f := newFixture(t, fake)

	if rec := f.do(t, http.MethodGet, "/orders/o-1", nil); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	span := serverSpan(t, f, "GET /orders/{id}")
	if span.Status.Code != codes.Error || attributesOf(span)["dmpf.outcome_category"].AsString() != "failed" {
		t.Fatalf("SERVER status = %v, attributes %v; want Error and failed", span.Status, span.Attributes)
	}
}

func TestTheReplayedHeaderReachesNeitherTheSpanNorTheLog(t *testing.T) {
	fake := (&fakeContexts{}).replay("PlaceOrder").on("PlaceOrder", func(int) (any, error) {
		return &ordersv1.PlaceOrderResponse{Result: &ordersv1.PlaceOrderResponse_Placed{Placed: &ordersv1.Placed{OrderId: "o-1"}}}, nil
	})
	f := newFixture(t, fake)

	rec := f.post(t, "/orders/o-1/place", "")
	if rec.Header().Get(api.ReplayedHeader) != "true" {
		t.Fatalf("%s = %q, want the replay marked to the client", api.ReplayedHeader, rec.Header().Get(api.ReplayedHeader))
	}

	for key, value := range attributesOf(serverSpan(t, f, "POST /orders/{id}/place")) {
		if strings.Contains(strings.ToLower(string(key)), "replayed") || strings.Contains(strings.ToLower(value.String()), "replayed") {
			t.Fatalf("SERVER %s = %s: the replay header stays out of the span", key, value.String())
		}
	}
	for _, record := range []map[string]any{onlyAccessLog(t, f), accessLogBeforeExport(t, f)} {
		for key, value := range record {
			if strings.Contains(strings.ToLower(key), "replayed") || strings.Contains(strings.ToLower(fmt.Sprint(value)), "replayed") {
				t.Fatalf("access log %s = %v: the replay header stays out of the log", key, value)
			}
		}
	}
}

func TestTheServerDurationIsMeasuredPerRoute(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil)

	var collected metricdata.ResourceMetrics
	if err := f.metrics.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "http.server.request.duration" {
				continue
			}
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			if !ok || len(histogram.DataPoints) != 1 {
				t.Fatalf("http.server.request.duration = %#v, want one histogram point", m.Data)
			}
			route, _ := histogram.DataPoints[0].Attributes.Value("http.route")
			if route.AsString() != "/orders/{id}" || histogram.DataPoints[0].Count != 1 {
				t.Fatalf("point = %v, want one request of /orders/{id}", histogram.DataPoints[0])
			}
			return
		}
	}
	t.Fatalf("no http.server.request.duration among %v", collected.ScopeMetrics)
}

func TestTheServerSpanCarriesTheContentTypeOfTheRequest(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.post(t, "/orders/o-1/place", "")

	values := attributesOf(serverSpan(t, f, "POST /orders/{id}/place"))
	if got := values["http.request.header.content-type"].AsStringSlice(); !slices.Equal(got, []string{"application/json"}) {
		t.Fatalf("SERVER http.request.header.content-type = %v, want [application/json] (RF-B2)", got)
	}
}

func TestAServerFailureRecordsItsFND07CategoryOnTheServerSpan(t *testing.T) {
	for name, c := range map[string]struct {
		code grpccodes.Code
		want string
	}{
		"INTERNAL":          {grpccodes.Internal, "Unexpected"},
		"UNAVAILABLE":       {grpccodes.Unavailable, "TransientDependency"},
		"DEADLINE_EXCEEDED": {grpccodes.DeadlineExceeded, "DeadlineExceeded"},
		"UNKNOWN":           {grpccodes.Unknown, semconv.ErrorTypeOther.Value.AsString()},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, (&fakeContexts{}).on("FindOrder", func(int) (any, error) { return nil, status.Error(c.code, "boom") }))

			if rec := f.do(t, http.MethodGet, "/orders/o-1", nil); rec.Code < http.StatusInternalServerError {
				t.Fatalf("status = %d, want a 5xx", rec.Code)
			}

			if got := attributesOf(serverSpan(t, f, "GET /orders/{id}"))["error.type"].AsString(); got != c.want {
				t.Fatalf("SERVER error.type = %q, want %q (RF-B1)", got, c.want)
			}
		})
	}
}

func TestAPanicIsRecordedAsUnexpectedOnTheServerSpan(t *testing.T) {
	f := newFixture(t, &fakeContexts{}, withAuthenticator(panickingAuthenticator{}))

	f.do(t, http.MethodGet, "/orders/o-1", nil)

	if got := attributesOf(serverSpan(t, f, "GET /orders/{id}"))["error.type"].AsString(); got != "Unexpected" {
		t.Fatalf("SERVER error.type = %q, want Unexpected (ERR-22, RF-B1)", got)
	}
}

func TestEveryRequestOutsideTheMountedRoutesIsOneServerSpanAndOnePoint(t *testing.T) {
	ready := func(context.Context) error { return nil }
	for _, c := range []struct {
		method, path string
		headers      []string
		status       int
		span, route  string
		admin        bool
	}{
		{http.MethodGet, api.OrdersContractPath, nil, http.StatusOK, "GET " + api.OrdersContractPath, api.OrdersContractPath, false},
		{http.MethodOptions, "/orders/o-1/place", []string{"Origin", "http://localhost:8082", "Access-Control-Request-Method", "POST"}, http.StatusNoContent, "OPTIONS", "", false},
		{http.MethodOptions, api.ReadinessPath, []string{"Origin", "http://localhost:8082", "Access-Control-Request-Method", "GET"}, http.StatusNoContent, "OPTIONS", "", false},
		{http.MethodGet, api.LivenessPath, nil, http.StatusNotFound, "GET", "", false},
		{http.MethodPost, api.LivenessPath, nil, http.StatusMethodNotAllowed, "POST", "", true},
		{http.MethodGet, "/nowhere/42", nil, http.StatusNotFound, "GET", "", false},
		{http.MethodGet, "/nowhere/42", nil, http.StatusNotFound, "GET", "", true},
		{http.MethodDelete, "/orders/o-1", nil, http.StatusMethodNotAllowed, "DELETE", "", false},
	} {
		port := map[bool]string{false: "public", true: "admin"}[c.admin]
		t.Run(port+" "+c.method+" "+c.path, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{}, withContracts("orders: yes", "reservations: yes"), withReady(ready), withCORS("http://localhost:8082"))

			if rec := f.doOn(t, c.admin, c.method, c.path, c.headers...); rec.Code != c.status {
				t.Fatalf("status = %d, want %d", rec.Code, c.status)
			}

			values := attributesOf(serverSpan(t, f, c.span))
			route, routed := values["http.route"]
			switch {
			case c.route == "" && routed:
				t.Fatalf("SERVER http.route = %q, want none outside a pattern of the mux", route.AsString())
			case c.route != "" && route.AsString() != c.route:
				t.Fatalf("SERVER http.route = %q, want %q", route.AsString(), c.route)
			}
			if want := cmp.Or(c.route, "REDACTED"); values["url.path"].AsString() != want {
				t.Fatalf("SERVER url.path = %q, want %q (RF-B3)", values["url.path"].AsString(), want)
			}
			if count := durationCount(t, f); count != 1 {
				t.Fatalf("http.server.request.duration counted %d requests, want 1 (RF-B2)", count)
			}
		})
	}
}

func durationCount(t *testing.T, f fixture) uint64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := f.metrics.Collect(context.Background(), &collected); err != nil {
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
