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
	return audit.Identity{Service: "orders", Version: "1.2.3", Instance: "pod-1"}
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
		"instance": "pod-1", "subject": "user-1",
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

// IDN-20: a process serves every tenant it authenticates, so the line names the
// tenant of the call from the carrier, and none when the call resolved none.
func TestTheEnvelopeNamesTheTenantOfTheCall(t *testing.T) {
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Tenant: &tenant,
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}

	record := emitOne(t, ports.WithExecutionContext(context.Background(), execution), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})
	if got, _ := record["tenant_id"].(string); got != "acme" {
		t.Fatalf("tenant_id = %q, want the tenant of the call", got)
	}

	bare := emitOne(t, context.Background(), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})
	if got, _ := bare["tenant_id"].(string); got != "" {
		t.Fatalf("tenant_id = %q, want absent outside a scoped call: no value is invented for it", got)
	}
}

func TestTheEnvelopeOfASecurityEventCarriesBothTenants(t *testing.T) {
	record := emitOne(t, context.Background(), audit.Event{
		Subject: "user-1", Object: "orders/o-1", Action: "security.cross_tenant_access", Outcome: "denied",
		Tenant: "globex", DataTenant: "acme",
	})

	if got, _ := record["tenant_id"].(string); got != "globex" {
		t.Fatalf("tenant_id = %q, want the tenant of the context, %q, over the process identity", got, "globex")
	}
	if got, _ := record["data_tenant_id"].(string); got != "acme" {
		t.Fatalf("data_tenant_id = %q, want the tenant of the data reached, %q (IDN-12)", got, "acme")
	}
}

func TestTheEnvelopeOfAnOrdinaryEventOmitsTheDataTenant(t *testing.T) {
	record := emitOne(t, context.Background(), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})

	if _, present := record["data_tenant_id"]; present {
		t.Fatalf("data_tenant_id present in %v, want it absent outside a cross-tenant access", record)
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
