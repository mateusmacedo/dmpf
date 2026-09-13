package resilience_test

import (
	"context"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
)

// Every constructor of a decorator accepts a nil set of instruments — a service
// that boots telemetry without a meter still gets its resilience. This asserts
// that none of them panics on the path, which is what a nil dereference would do
// on the first call rather than at construction.
func TestNilInstrumentsNeverPanicOnThePath(t *testing.T) {
	fake := clock.NewFake(time.Unix(0, 0))
	sheet := resilience.Defaults("payments")
	policy, _ := sheet.Breaker.Get()
	pool, _ := sheet.Bulkhead.Get()

	call := func(context.Context, resilience.Operation, func(context.Context) error) error { return nil }
	op := resilience.Operation{
		Dependency: "payments", Method: "Charge", Kind: resilience.Remote,
		Deadline: time.Second, EstimatedDuration: time.Millisecond, Idempotent: true,
	}

	degradation, err := resilience.NewDegradation(resilience.Fail, "payments", nil)
	if err != nil {
		t.Fatalf("NewDegradation() = %v", err)
	}

	for name, decorator := range map[string]resilience.Decorator{
		"breaker":  resilience.NewBreaker("payments", policy, fake, nil).Decorate(),
		"bulkhead": resilience.NewBulkhead("payments", pool, fake, nil).Decorate(),
		"timeout":  resilience.Timeout(fake, nil, 0),
		"degrade":  degradation,
	} {
		t.Run(name, func(t *testing.T) {
			if err := decorator(call)(context.Background(), op, nil); err != nil {
				t.Errorf("%s with nil instruments = %v, want nil", name, err)
			}
		})
	}
}
