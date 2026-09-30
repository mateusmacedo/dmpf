package app

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// IDN-20: the line names the tenant the edge authenticated, off the carrier,
// and no tenant when the call resolved none.
func TestRequestFieldsNameTheTenantOfTheCall(t *testing.T) {
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Tenant: &tenant,
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}

	if got := requestFields(ports.WithExecutionContext(context.Background(), execution))[logging.KeyTenantID]; got != "acme" {
		t.Fatalf("tenant = %v, want the tenant of the call", got)
	}
	if got, present := requestFields(context.Background())[logging.KeyTenantID]; present {
		t.Fatalf("tenant = %v, want absent outside a scoped call", got)
	}
}

func TestRequestFieldsCarryTheCorrelationAndTheRequestOfTheCallInFlight(t *testing.T) {
	ctx := rpc.WithCall(context.Background(), rpc.Call{CorrelationID: "corr-1", RequestID: "req-1"})

	fields := requestFields(ctx)

	if got := fields[logging.KeyCorrelationID]; got != "corr-1" {
		t.Fatalf("correlation = %v, want %q", got, "corr-1")
	}
	if got := fields[logging.KeyRequestID]; got != "req-1" {
		t.Fatalf("request = %v, want %q: the border is the only place that mints a request id, and losing it breaks the trail back to the caller", got, "req-1")
	}
}

func TestRequestFieldsOmitTheCallFieldsOutsideACall(t *testing.T) {
	fields := requestFields(context.Background())

	for _, key := range []string{logging.KeyCorrelationID, logging.KeyRequestID} {
		if _, ok := fields[key]; ok {
			t.Fatalf("%s is present without a call; the field would carry an empty value", key)
		}
	}
}
