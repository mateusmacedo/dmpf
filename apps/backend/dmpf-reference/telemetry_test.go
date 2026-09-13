package dmpfreference_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	dmpfreference "github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/audit"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// The audit trail and the application log share one destination in the local
// stack, so one shape: the record carries the same envelope the log handler
// writes, the trace of the request and the correlation the edge authored.
func TestAuditRecordsShareTheLogEnvelopeAndCarryTheTrace(t *testing.T) {
	var out bytes.Buffer
	cfg := dmpfreference.Defaults(dmpfreference.RoleAPI)
	cfg.Version, cfg.Instance = "1.2.3", "api-1"
	sink := dmpfreference.NewAuditSink(&out, cfg)

	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := provider.Tracer("test").Start(context.Background(), "HTTP POST /orders/{id}/items")
	defer span.End()
	ctx = dmpfports.WithMessageContext(ctx, dmpfports.MessageContext{CorrelationID: "corr-42"})

	if err := sink.Emit(ctx, audit.Event{Subject: "", Object: "o-1", Action: "orders.AddItem", Outcome: "accepted", At: 1_788_920_138_943_455_781}); err != nil {
		t.Fatalf("Emit() = %v", err)
	}

	var record map[string]string
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("the record is not a flat JSON object: %v — %s", err, out.String())
	}
	want := map[string]string{
		"time":           "2026-09-09T02:15:38.943455781Z",
		"level":          "INFO",
		"msg":            "audit",
		"kind":           "audit",
		"service":        "dmpf-reference",
		"version":        "1.2.3",
		"instance":       "api-1",
		"correlation_id": "corr-42",
		"tenant_id":      "public",
		"object":         "o-1",
		"action":         "orders.AddItem",
		"outcome":        "accepted",
		"trace_id":       span.SpanContext().TraceID().String(),
		"span_id":        span.SpanContext().SpanID().String(),
	}
	for key, value := range want {
		if record[key] != value {
			t.Errorf("record[%q] = %q, want %q", key, record[key], value)
		}
	}
	if _, present := record["subject"]; !present {
		t.Error("subject is absent from the record; the trail records it as empty, never drops it")
	}
}

func TestAuditRecordsOutsideARequestHaveNoTrace(t *testing.T) {
	var out bytes.Buffer
	sink := dmpfreference.NewAuditSink(&out, dmpfreference.Defaults(dmpfreference.RoleAPI))

	if err := sink.Emit(context.Background(), audit.Event{Object: "o-1", Action: "orders.AddItem", Outcome: "accepted"}); err != nil {
		t.Fatalf("Emit() = %v", err)
	}

	var record map[string]string
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if record["trace_id"] != "" || record["span_id"] != "" || record["correlation_id"] != "" {
		t.Fatalf("record outside a request = %v, want empty trace and correlation, never invented", record)
	}
}
