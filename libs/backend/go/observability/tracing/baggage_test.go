package tracing_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/baggage"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestTheExecutionBaggageCarriesTheCommonIdentifiers(t *testing.T) {
	tenant := ports.TenantID("tenant-1")
	existing, err := baggage.NewMemberRaw("other", "kept")
	if err != nil {
		t.Fatalf("NewMemberRaw() = %v", err)
	}
	bag, _ := baggage.New(existing)
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	got := baggage.FromContext(tracing.WithExecutionBaggage(ctx, executionContext(t, &tenant)))

	for key, want := range map[string]string{
		tracing.KeyCorrelationID: "corr-1",
		tracing.KeyRequestID:     "req-1",
		tracing.KeyTenantID:      "tenant-1",
		"other":                  "kept",
	} {
		if value := got.Member(key).Value(); value != want {
			t.Errorf("baggage %s = %q, want %q", key, value, want)
		}
	}
}

func TestAnExecutionWithoutTenantPutsNoTenantInTheBaggage(t *testing.T) {
	got := baggage.FromContext(tracing.WithExecutionBaggage(context.Background(), executionContext(t, nil)))

	if member := got.Member(tracing.KeyTenantID); member.Key() != "" {
		t.Errorf("baggage carries %s = %q, want it absent", tracing.KeyTenantID, member.Value())
	}
	if got.Len() != 2 {
		t.Errorf("baggage = %v, want the correlation and the request only", got)
	}
}
