package app

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// IDN-20: the line names the tenant of the call, off the carrier, and no tenant
// when the call resolved none; a fixed value would put one tenancy's name on
// another tenant's lines.
func TestRequestFieldsNameTheTenantOfTheCall(t *testing.T) {
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Tenant: &tenant,
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}

	fields := requestFields(ports.WithExecutionContext(context.Background(), execution))

	if got := fields[logging.KeyTenantID]; got != "acme" {
		t.Fatalf("tenant = %v, want the tenant of the call", got)
	}
}

func TestRequestFieldsOmitTheTenantOutsideAScopedCall(t *testing.T) {
	if got, present := requestFields(context.Background())[logging.KeyTenantID]; present {
		t.Fatalf("tenant = %v, want absent: no value is invented for it", got)
	}
}
