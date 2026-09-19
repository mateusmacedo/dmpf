package bff

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/api"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
)

func TestRequestFieldsAlwaysCarryTheTenant(t *testing.T) {
	fields := requestFields(context.Background())

	if got := fields[logging.KeyTenantID]; got != api.Tenant {
		t.Fatalf("tenant = %v, want %q: every line is attributed even outside a call", got, api.Tenant)
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
