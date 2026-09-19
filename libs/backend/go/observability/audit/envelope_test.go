package audit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func envelopeIdentity() audit.Identity {
	return audit.Identity{Service: "orders", Version: "1.2.3", Instance: "pod-1", Tenant: "public"}
}

func emitOne(t *testing.T, ctx context.Context, event audit.Event) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := audit.NewEnvelopeSink(&out, envelopeIdentity()).Emit(ctx, event); err != nil {
		t.Fatalf("Emit() = %v, want nil", err)
	}
	var record map[string]any
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("the line is not one JSON object: %v", err)
	}
	return record
}

func TestTheEnvelopeCarriesTheProcessIdentityAndTheEvent(t *testing.T) {
	record := emitOne(t, context.Background(), audit.Event{
		Subject: "user-1", Object: "order-1", Action: "orders.PlaceOrder", Outcome: "accepted", At: ports.Instant(1_700_000_000_000_000_000),
	})

	want := map[string]string{
		"kind": "audit", "msg": "audit", "service": "orders", "version": "1.2.3",
		"instance": "pod-1", "tenant_id": "public", "subject": "user-1",
		"object": "order-1", "action": "orders.PlaceOrder", "outcome": "accepted",
	}
	for key, value := range want {
		if got, _ := record[key].(string); got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
	if got, _ := record["time"].(string); !strings.HasPrefix(got, "2023-11-14T") {
		t.Fatalf("time = %q, want the instant of the event in RFC3339Nano UTC", got)
	}
}

func TestTheEnvelopeCarriesTheCorrelationOfTheMessageInFlight(t *testing.T) {
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{CorrelationID: "corr-1"})

	record := emitOne(t, ctx, audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})

	if got, _ := record["correlation_id"].(string); got != "corr-1" {
		t.Fatalf("correlation_id = %q, want %q: the trail joins the logs beside it by this field", got, "corr-1")
	}
}

func TestTheEnvelopeIsEmptyOfTraceOutsideASpan(t *testing.T) {
	record := emitOne(t, context.Background(), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})

	if got, _ := record["trace_id"].(string); got != "" {
		t.Fatalf("trace_id = %q, want empty outside a span", got)
	}
}

func TestConcurrentEmitsKeepTheTrailParseable(t *testing.T) {
	var out bytes.Buffer
	sink := audit.NewEnvelopeSink(&out, envelopeIdentity())
	var wg sync.WaitGroup

	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sink.Emit(context.Background(), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 50 {
		t.Fatalf("lines = %d, want 50", len(lines))
	}
	for i, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("line %d is not one JSON object: %v", i, err)
		}
	}
}
