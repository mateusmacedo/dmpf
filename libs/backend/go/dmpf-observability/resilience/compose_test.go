package resilience_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
)

// probe records that a position ran, which is how the order becomes observable
// from outside the composition.
func probe(observed *[]string, position string) resilience.Decorator {
	return func(next resilience.Call) resilience.Call {
		return func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
			*observed = append(*observed, position)
			return next(ctx, op, do)
		}
	}
}

// fullSlots fills every position with a probe, so no position is empty and the
// declared-absence check does not interfere with the order assertion.
func fullSlots(observed *[]string) resilience.Slots {
	return resilience.Slots{
		Tracing:   probe(observed, resilience.SlotTracing),
		Metrics:   probe(observed, resilience.SlotMetrics),
		Logging:   probe(observed, resilience.SlotLogging),
		Bulkhead:  probe(observed, resilience.SlotBulkhead),
		Breaker:   probe(observed, resilience.SlotBreaker),
		RateLimit: probe(observed, resilience.SlotRateLimit),
		Retry:     probe(observed, resilience.SlotRetry),
		Timeout:   probe(observed, resilience.SlotTimeout),
	}
}

// fullSheet declares every field, so Validate passes and every position may be
// filled.
func fullSheet() resilience.Sheet {
	sheet := resilience.Defaults("payments")
	sheet.RateLimit = resilience.Declare(resilience.RateLimitPolicy{PerSecond: 10, Burst: 20})
	return sheet
}

func TestComposeAppliesTheCanonicalOrderFromTheOutsideIn(t *testing.T) {
	var observed []string
	call, err := resilience.Compose(fullSheet(), fullSlots(&observed))
	if err != nil {
		t.Fatalf("Compose() = %v, want nil", err)
	}

	if err := call(context.Background(), remoteOp(time.Second), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("call() = %v, want nil", err)
	}

	want := resilience.CanonicalOrder()
	if !slices.Equal(observed, want) {
		t.Fatalf("observed order = %v, want %v (RES-22)", observed, want)
	}
}

func TestTheCanonicalOrderHasTheEightDecoratorPositions(t *testing.T) {
	order := resilience.CanonicalOrder()

	if len(order) != 8 {
		t.Fatalf("CanonicalOrder() has %d positions, want 8 plus the call itself", len(order))
	}
	if order[0] != resilience.SlotTracing {
		t.Errorf("the outermost position is %q, want tracing: a refusal below must still land on the span", order[0])
	}
	if order[len(order)-1] != resilience.SlotTimeout {
		t.Errorf("the innermost position is %q, want timeout: the deadline bounds the call, not the queueing", order[len(order)-1])
	}
}

func TestCanonicalOrderReturnsAFreshSlice(t *testing.T) {
	resilience.CanonicalOrder()[0] = "tampered"

	if got := resilience.CanonicalOrder()[0]; got != resilience.SlotTracing {
		t.Fatalf("CanonicalOrder()[0] = %q after a caller rewrote it, want %q", got, resilience.SlotTracing)
	}
}

func TestAnEmptyPositionIsAcceptedOnlyWhenTheSheetDeclaresTheAbsence(t *testing.T) {
	var observed []string
	slots := fullSlots(&observed)
	slots.RateLimit = nil

	sheet := resilience.Defaults("payments")
	if _, err := resilience.Compose(sheet, slots); err != nil {
		t.Fatalf("Compose() = %v, want nil — the default sheet declares rate limiting not applicable", err)
	}

	declared := fullSheet()
	if _, err := resilience.Compose(declared, slots); !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Compose() = %v, want a refusal: the sheet declares a rate limit and the position is empty", err)
	}
}

func TestAnObservabilityPositionIsNeverAllowedToBeEmpty(t *testing.T) {
	for _, drop := range []string{resilience.SlotTracing, resilience.SlotMetrics, resilience.SlotLogging} {
		t.Run(drop, func(t *testing.T) {
			var observed []string
			slots := fullSlots(&observed)
			switch drop {
			case resilience.SlotTracing:
				slots.Tracing = nil
			case resilience.SlotMetrics:
				slots.Metrics = nil
			case resilience.SlotLogging:
				slots.Logging = nil
			}

			_, err := resilience.Compose(fullSheet(), slots)

			if !errors.Is(err, resilience.ErrBlankField) {
				t.Fatalf("Compose() = %v, want a refusal — every decorator is observable (RES-23)", err)
			}
		})
	}
}

func TestComposeRefusesAnInvalidSheet(t *testing.T) {
	var observed []string

	_, err := resilience.Compose(resilience.Sheet{Dependency: "payments"}, fullSlots(&observed))

	if !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Compose() = %v, want the sheet's own refusal", err)
	}
}

func TestAnotherOrderRequiresADeclaredReason(t *testing.T) {
	var observed []string
	order := slices.Clone(resilience.CanonicalOrder())
	order[0], order[1] = order[1], order[0]

	_, err := resilience.ComposeWithOrder(fullSheet(), fullSlots(&observed), order, "")

	if !errors.Is(err, resilience.ErrOrderReasonRequired) {
		t.Fatalf("ComposeWithOrder() = %v, want ErrOrderReasonRequired (RES-22)", err)
	}
}

func TestAnotherOrderIsAppliedWhenJustified(t *testing.T) {
	var observed []string
	order := slices.Clone(resilience.CanonicalOrder())
	order[0], order[1] = order[1], order[0]

	call, err := resilience.ComposeWithOrder(fullSheet(), fullSlots(&observed), order, "métricas antes do tracing por exigência do coletor")
	if err != nil {
		t.Fatalf("ComposeWithOrder() = %v, want nil", err)
	}
	if err := call(context.Background(), remoteOp(time.Second), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("call() = %v, want nil", err)
	}

	if !slices.Equal(observed, order) {
		t.Fatalf("observed order = %v, want the declared %v", observed, order)
	}
}

func TestAnOrderThatIsNotAPermutationIsRefused(t *testing.T) {
	var observed []string
	canonical := resilience.CanonicalOrder()

	cases := map[string][]string{
		"a position missing":  canonical[:len(canonical)-1],
		"a position twice":    append(slices.Clone(canonical[:len(canonical)-1]), canonical[0]),
		"an unknown position": append(slices.Clone(canonical[:len(canonical)-1]), "cache"),
	}
	for name, order := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := resilience.ComposeWithOrder(fullSheet(), fullSlots(&observed), order, "motivo declarado")

			if !errors.Is(err, resilience.ErrBlankField) {
				t.Fatalf("ComposeWithOrder() = %v, want a refusal with %s", err, name)
			}
		})
	}
}

func TestTheComposedCallReachesTheWork(t *testing.T) {
	var observed []string
	call, err := resilience.Compose(fullSheet(), fullSlots(&observed))
	if err != nil {
		t.Fatalf("Compose() = %v, want nil", err)
	}

	reached := false
	if err := call(context.Background(), remoteOp(time.Second), func(context.Context) error {
		reached = true
		return nil
	}); err != nil {
		t.Fatalf("call() = %v, want nil", err)
	}

	if !reached {
		t.Fatal("the work never ran: the innermost position must be the call itself")
	}
}

func TestTheComposedCallPropagatesTheFailure(t *testing.T) {
	var observed []string
	call, _ := resilience.Compose(fullSheet(), fullSlots(&observed))

	err := call(context.Background(), remoteOp(time.Second), func(context.Context) error { return errDependency })

	if !errors.Is(err, errDependency) {
		t.Fatalf("call() = %v, want the work's error", err)
	}
}
