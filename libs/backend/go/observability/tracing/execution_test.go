package tracing_test

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func executionContext(t *testing.T, tenant *ports.TenantID) ports.ExecutionContext {
	t.Helper()

	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		TraceContext:  "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		Tenant:        tenant,
		Deadline:      1,
		Locale:        "pt-BR",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return execution
}

func TestExecutionAttributesCarryTheCommonIdentifiers(t *testing.T) {
	tenant := ports.TenantID("tenant-1")

	got := tracing.ExecutionAttributes(executionContext(t, &tenant)).KeyValues()

	want := []attribute.KeyValue{
		attribute.String(tracing.KeyCorrelationID, "corr-1"),
		attribute.String(tracing.KeyRequestID, "req-1"),
		attribute.String(tracing.KeyTenantID, "tenant-1"),
	}
	if len(got) != len(want) {
		t.Fatalf("KeyValues() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("KeyValues()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestExecutionAttributesOmitAnAbsentTenant(t *testing.T) {
	got := tracing.ExecutionAttributes(executionContext(t, nil)).KeyValues()

	for _, kv := range got {
		if string(kv.Key) == tracing.KeyTenantID {
			t.Fatalf("KeyValues() = %v, want no tenant along a chain without one (CTX-26)", got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("KeyValues() = %v, want the correlation and the request only", got)
	}
}

func TestExecutionAttributesCompose(t *testing.T) {
	got := tracing.ExecutionAttributes(executionContext(t, nil)).TrafficClass("write").KeyValues()

	if last := got[len(got)-1]; string(last.Key) != tracing.KeyTrafficClass {
		t.Fatalf("KeyValues() = %v, want the builder to keep accumulating after the execution set", got)
	}
}
