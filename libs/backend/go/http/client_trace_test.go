package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

func tracedConfig(t *testing.T) (provider func() tracetest.SpanStubs, cfgTracer trace.Tracer) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(otelboot.NewPrivacyExporter(exporter)))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exporter.GetSpans, tp.Tracer("t")
}

func clientSpans(spans tracetest.SpanStubs) []tracetest.SpanStub {
	var found []tracetest.SpanStub
	for _, span := range spans {
		if span.SpanKind == trace.SpanKindClient {
			found = append(found, span)
		}
	}
	return found
}

func attrsOf(span tracetest.SpanStub) map[attribute.Key]attribute.Value {
	attrs := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes {
		attrs[kv.Key] = kv.Value
	}
	return attrs
}

func TestEveryAttemptIsAClientSpanWithItsResendCount(t *testing.T) {
	up := newUpstream(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK)
	spans, tracer := tracedConfig(t)
	cfg := clientConfig(route(http.MethodGet))
	cfg.Tracer = tracer
	client := newClient(t, cfg)

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, up.URL+"/realms/x/certs?token=s", ""))
	if err != nil {
		t.Fatalf("Do() = %v", err)
	}
	_ = resp.Body.Close()

	clients := clientSpans(spans())
	if len(clients) != 3 {
		t.Fatalf("%d CLIENT spans, want one per attempt (TRC-11)", len(clients))
	}
	var resilience tracetest.SpanStub
	for _, span := range spans() {
		if span.Name == "dmpf.resilience orders" {
			resilience = span
		}
	}
	if !resilience.SpanContext.IsValid() {
		t.Fatalf("no dmpf.resilience orders span among %d", len(spans()))
	}
	for i, span := range clients {
		attrs := attrsOf(span)
		count, present := attrs["http.request.resend_count"]
		switch {
		case i == 0 && present:
			t.Errorf("first attempt carries http.request.resend_count = %v, want it absent", count)
		case i > 0 && count.AsInt64() != int64(i):
			t.Errorf("attempt %d: http.request.resend_count = %v, want %d", i, count, i)
		}
		if span.Name != http.MethodGet {
			t.Errorf("attempt %d: span name = %q, want %q (RF-B1)", i, span.Name, http.MethodGet)
		}
		if span.Parent.SpanID() != resilience.SpanContext.SpanID() {
			t.Errorf("attempt %d is not a child of the resilience span", i)
		}
		full := attrs["url.full"].AsString()
		if full != up.URL+"/REDACTED" || strings.Contains(full, "token") {
			t.Errorf("attempt %d: url.full = %q, want the origin and /REDACTED (RF-B3)", i, full)
		}
	}
}
