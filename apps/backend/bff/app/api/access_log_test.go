package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const severityKey = "severity"

func accessRecords(t *testing.T, f fixture) []sdklog.Record {
	t.Helper()
	if err := f.logProvider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	var records []sdklog.Record
	for _, record := range f.records.all() {
		if record.Body().AsString() == "http request" {
			records = append(records, record)
		}
	}
	return records
}

func accessLogs(t *testing.T, f fixture) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, record := range accessRecords(t, f) {
		fields := map[string]any{severityKey: record.SeverityText()}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			fields[string(kv.Key)] = kv.Value.AsInterface()
			return true
		})
		records = append(records, fields)
	}
	return records
}

func onlyAccessLog(t *testing.T, f fixture) map[string]any {
	t.Helper()
	records := accessLogs(t, f)
	if len(records) != 1 {
		t.Fatalf("access log = %v, want one record", records)
	}
	return records[0]
}

func accessLogBeforeExport(t *testing.T, f fixture) map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(f.logs.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err == nil && record["msg"] == "http request" {
			records = append(records, record)
		}
	}
	if len(records) != 1 {
		t.Fatalf("emitted access log = %v, want one record", f.logs.String())
	}
	return records[0]
}

func TestAnAcceptedRequestIsLoggedAtInfoWithTheExecutionFromTheBaggage(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodPost, "/orders/o-1/items", strings.NewReader(`{"sku":"A","quantity":1}`),
		"Idempotency-Key", "k-1", "Content-Type", "application/json", api.CorrelationHeader, "corr-1")

	record := onlyAccessLog(t, f)
	want := map[string]any{
		severityKey:                 "INFO",
		"http.request.method":       "POST",
		"http.route":                "/orders/{id}/items",
		"http.response.status_code": int64(http.StatusCreated),
		"dmpf.outcome_category":     "accepted",
		"dmpf.correlation_id":       "corr-1",
		"dmpf.tenant_id":            testTenant,
	}
	for key, value := range want {
		if record[key] != value {
			t.Fatalf("access log %s = %v, want %v (record %v)", key, record[key], value, record)
		}
	}
	if requestID, _ := record["dmpf.request_id"].(string); requestID == "" {
		t.Fatalf("access log = %v, want the dmpf.request_id of the execution", record)
	}
	emitted := accessLogBeforeExport(t, f)
	for _, gone := range []string{"route", "method", "pattern", "status", "duration_ms", "idempotency_key", "derived_idempotency_key"} {
		if _, logged := record[gone]; logged {
			t.Fatalf("access log = %v, want no %q (RF-A5)", record, gone)
		}
		if _, logged := emitted[gone]; logged {
			t.Fatalf("emitted access log = %v, want no %q (RF-A5)", emitted, gone)
		}
	}
}

func TestARefusedRequestIsLoggedByItsOutcome(t *testing.T) {
	for name, c := range map[string]struct {
		do      func(f fixture)
		status  int64
		outcome string
		level   string
	}{
		"denied": {
			do:      func(f fixture) { f.do(t, http.MethodGet, "/orders/o-1", nil, "Authorization", "") },
			status:  http.StatusUnauthorized,
			outcome: "denied",
			level:   "WARN",
		},
		"rejected": {
			do:      func(f fixture) { f.do(t, http.MethodGet, "/orders/bad%20id", nil) },
			status:  http.StatusBadRequest,
			outcome: "rejected",
			level:   "INFO",
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{})

			c.do(f)

			record := onlyAccessLog(t, f)
			if record["http.response.status_code"] != c.status || record["dmpf.outcome_category"] != c.outcome || record[severityKey] != c.level {
				t.Fatalf("access log = %v, want %d %s at %s", record, c.status, c.outcome, c.level)
			}
		})
	}
}

func TestADeniedRequestCarriesNoExecution(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil, "Authorization", "", api.CorrelationHeader, "corr-1")

	record := onlyAccessLog(t, f)
	for _, key := range []string{"dmpf.correlation_id", "dmpf.request_id", "dmpf.tenant_id"} {
		if _, logged := record[key]; logged {
			t.Fatalf("access log = %v, want no %s without an execution (CTX-26)", record, key)
		}
	}
}

func TestBaggageFromTheWireNeverReachesTheAccessLog(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil, "Authorization", "", "baggage", "dmpf.correlation_id=forged,dmpf.tenant_id=evil")

	record := onlyAccessLog(t, f)
	for _, key := range []string{"dmpf.correlation_id", "dmpf.tenant_id"} {
		if _, logged := record[key]; logged {
			t.Fatalf("access log = %v, want no %s taken from the wire (RF-B8)", record, key)
		}
	}
}

type panickingAuthenticator struct{}

func (panickingAuthenticator) Authenticate(context.Context, ports.Credential) (ports.Identity, error) {
	panic("authenticator bug")
}

func TestAPanickingRequestIsLoggedAsAServerFailure(t *testing.T) {
	f := newFixture(t, &fakeContexts{}, withAuthenticator(panickingAuthenticator{}))

	response := f.do(t, "GET", "/orders/o-1", nil)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	record := onlyAccessLog(t, f)
	if record["http.response.status_code"] != int64(http.StatusInternalServerError) || record["dmpf.outcome_category"] != "failed" || record[severityKey] != "ERROR" {
		t.Fatalf("access log = %v, want one 500 failed at ERROR", record)
	}
}

func TestTheAccessLogNamesTheMethodAsTheServerSpanDoes(t *testing.T) {
	for _, c := range []struct{ method, want, span string }{
		{"PROBEMETHODXYZ", "_OTHER", "HTTP"},
		{"get", http.MethodGet, http.MethodGet},
	} {
		t.Run(c.method, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{})

			if rec := f.do(t, c.method, "/orders/o-1", nil); rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s /orders/o-1 = %d, want 405", c.method, rec.Code)
			}

			record := onlyAccessLog(t, f)
			spanMethod := attributesOf(serverSpan(t, f, c.span))["http.request.method"].AsString()
			if record["http.request.method"] != c.want || spanMethod != c.want {
				t.Fatalf("access log http.request.method = %v, SERVER = %q, want both %q (RF-A3, RF-B2)", record["http.request.method"], spanMethod, c.want)
			}
			if c.want == "_OTHER" && strings.Contains(f.logs.String(), c.method) {
				t.Fatalf("log = %q, want no raw method of the client", f.logs.String())
			}
		})
	}
}

func TestEveryResponseWithAServerSpanIsLoggedOnceInItsTrace(t *testing.T) {
	const origin = "http://localhost:8082"
	ready := func(context.Context) error { return nil }
	execution := []string{"dmpf.correlation_id", "dmpf.request_id", "dmpf.tenant_id"}
	for _, c := range []struct {
		method, path string
		headers      []string
		status       int
		route        string
		outcome      string
		execution    []string
		admin        bool
	}{
		{http.MethodOptions, "/orders/o-1/place", []string{"Origin", origin, "Access-Control-Request-Method", "POST"}, http.StatusNoContent, "", "accepted", nil, false},
		{http.MethodOptions, api.LivenessPath, []string{"Origin", origin, "Access-Control-Request-Method", "GET"}, http.StatusNoContent, "", "accepted", nil, false},
		{http.MethodOptions, api.ReadinessPath, []string{"Origin", origin, "Access-Control-Request-Method", "GET"}, http.StatusNoContent, "", "accepted", nil, false},
		{http.MethodOptions, api.LivenessPath, []string{"Origin", origin, "Access-Control-Request-Method", "GET"}, http.StatusMethodNotAllowed, "", "rejected", nil, true},
		{http.MethodPost, api.LivenessPath, nil, http.StatusMethodNotAllowed, "", "rejected", nil, true},
		{http.MethodPost, api.ReadinessPath, nil, http.StatusMethodNotAllowed, "", "rejected", nil, true},
		{http.MethodGet, "/nowhere/42", nil, http.StatusNotFound, "", "rejected", nil, false},
		{http.MethodGet, "/nowhere/42", nil, http.StatusNotFound, "", "rejected", nil, true},
		{http.MethodDelete, "/orders/o-1", nil, http.StatusMethodNotAllowed, "", "rejected", nil, false},
		{http.MethodGet, api.OrdersContractPath, nil, http.StatusOK, api.OrdersContractPath, "accepted", nil, false},
		{http.MethodGet, "/orders/o-1", nil, http.StatusOK, "/orders/{id}", "accepted", execution, false},
		{http.MethodGet, "/" + api.LivenessPath, nil, http.StatusTemporaryRedirect, api.LivenessPath, "accepted", nil, true},
		{http.MethodGet, "/x/.." + api.ReadinessPath, nil, http.StatusTemporaryRedirect, api.ReadinessPath, "accepted", nil, true},
		{http.MethodGet, "/orders/./o-1", nil, http.StatusTemporaryRedirect, "/orders/{id}", "accepted", nil, false},
		{http.MethodPost, "/orders//o-1/place", nil, http.StatusTemporaryRedirect, "/orders/{id}/place", "accepted", nil, false},
	} {
		port := map[bool]string{false: "public", true: "admin"}[c.admin]
		t.Run(port+" "+c.method+" "+c.path, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{}, withContracts("orders: yes", "reservations: yes"), withReady(ready), withCORS(origin))

			if rec := f.doOn(t, c.admin, c.method, c.path, c.headers...); rec.Code != c.status {
				t.Fatalf("status = %d, want %d", rec.Code, c.status)
			}

			records := accessRecords(t, f)
			if len(records) != 1 {
				t.Fatalf("access logs = %d, want exactly one for a response with a SERVER span (RF-A5)", len(records))
			}
			record := records[0]
			span := serverSpan(t, f, strings.TrimSpace(c.method+" "+c.route))
			if !record.TraceID().IsValid() || record.TraceID() != span.SpanContext.TraceID() || record.SpanID() != span.SpanContext.SpanID() {
				t.Fatalf("access log in %s/%s, want the SERVER span %s/%s", record.TraceID(), record.SpanID(), span.SpanContext.TraceID(), span.SpanContext.SpanID())
			}
			if record.SeverityText() != "INFO" {
				t.Fatalf("access log severity = %s, want INFO for %s by the severity table", record.SeverityText(), c.outcome)
			}

			want := map[string]any{
				"http.request.method":       c.method,
				"http.response.status_code": int64(c.status),
				"dmpf.outcome_category":     c.outcome,
			}
			if c.route != "" {
				want["http.route"] = c.route
			}
			got := map[string]any{}
			record.WalkAttributes(func(kv attribute.KeyValue) bool {
				got[string(kv.Key)] = kv.Value.AsInterface()
				return true
			})
			for _, key := range c.execution {
				if value, _ := got[key].(string); value == "" {
					t.Fatalf("access log = %v, want the %s of the execution (RF-A4)", got, key)
				}
				delete(got, key)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("access log = %v, want %v (RF-A5)", got, want)
			}
		})
	}
}
