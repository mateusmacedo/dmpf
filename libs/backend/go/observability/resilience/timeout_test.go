package resilience_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
)

var start = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

func remoteOp(deadline time.Duration) resilience.Operation {
	return resilience.Operation{
		Dependency:        "payments",
		Method:            "Authorize",
		Kind:              resilience.Remote,
		Deadline:          deadline,
		EstimatedDuration: deadline / 4,
	}
}

// meter returns a set of instruments over a manual reader, plus a lookup of how
// many times a series was incremented for a dependency.
func meter(t *testing.T) (*metrics.Instruments, func(name string) int64) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	instruments, err := metrics.New(provider.Meter("observability"))
	if err != nil {
		t.Fatalf("metrics.New() = %v, want nil", err)
	}

	// The reader is returned as a lookup that handles both shapes: a counter
	// arrives as a Sum and the breaker state as a Gauge, and a lookup that knew
	// only Sum would silently report zero for the gauge.
	return instruments, func(name string) int64 {
		var collected metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &collected); err != nil {
			t.Fatalf("Collect() = %v, want nil", err)
		}

		var value int64
		for _, scope := range collected.ScopeMetrics {
			for _, series := range scope.Metrics {
				if series.Name != name {
					continue
				}
				switch data := series.Data.(type) {
				case metricdata.Sum[int64]:
					for _, point := range data.DataPoints {
						value += point.Value
					}
				case metricdata.Gauge[int64]:
					for _, point := range data.DataPoints {
						value = point.Value
					}
				default:
					t.Fatalf("series %q has an unexpected shape %T", name, series.Data)
				}
			}
		}
		return value
	}
}

func TestTheMethodDeadlineDecidesWhenTheContextHasNone(t *testing.T) {
	fake := clock.NewFake(start)

	got := resilience.EffectiveDeadline(context.Background(), fake, remoteOp(2*time.Second), 0)

	if got != 2*time.Second {
		t.Fatalf("EffectiveDeadline() = %v, want the method's 2s (RES-05)", got)
	}
}

func TestTheShorterOfTheCallerDeadlineAndTheMethodDecides(t *testing.T) {
	fake := clock.NewFake(start)
	ctx, cancel := fake.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	got := resilience.EffectiveDeadline(ctx, fake, remoteOp(2*time.Second), 0)

	if got != 500*time.Millisecond {
		t.Fatalf("EffectiveDeadline() = %v, want the caller's 500ms", got)
	}
}

func TestTheMethodDeadlineWinsWhenTheCallerHasMoreTime(t *testing.T) {
	fake := clock.NewFake(start)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	got := resilience.EffectiveDeadline(ctx, fake, remoteOp(2*time.Second), 0)

	if got != 2*time.Second {
		t.Fatalf("EffectiveDeadline() = %v, want the method's 2s", got)
	}
}

func TestTheCallerDeadlineShrinksAsTheClockAdvances(t *testing.T) {
	fake := clock.NewFake(start)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Second)
	defer cancel()

	fake.Advance(700 * time.Millisecond)
	got := resilience.EffectiveDeadline(ctx, fake, remoteOp(2*time.Second), 0)

	if got != 300*time.Millisecond {
		t.Fatalf("EffectiveDeadline() = %v, want the 300ms that remain", got)
	}
}

func TestTheBudgetMinusTheReserveCanDecide(t *testing.T) {
	fake := clock.NewFake(start)
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(600*time.Millisecond))

	got := resilience.EffectiveDeadline(ctx, fake, remoteOp(2*time.Second), 100*time.Millisecond)

	if got != 500*time.Millisecond {
		t.Fatalf("EffectiveDeadline() = %v, want the budget of 600ms minus the 100ms reserve", got)
	}
}

func TestAnUndimensionedBudgetDoesNotRestrictTheAttemptInFlight(t *testing.T) {
	fake := clock.NewFake(start)
	ctx := retry.WithBudget(context.Background())

	got := resilience.EffectiveDeadline(ctx, fake, remoteOp(2*time.Second), 100*time.Millisecond)

	if got != 2*time.Second {
		t.Fatalf("EffectiveDeadline() = %v, want the method's 2s — an unsized budget restricts retries, not the attempt", got)
	}
}

func TestAnExhaustedBudgetDoesNotRestrictTheAttemptInFlight(t *testing.T) {
	fake := clock.NewFake(start)
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(time.Second))
	budget, _ := retry.BudgetFrom(ctx)
	budget.Debit(time.Second)

	got := resilience.EffectiveDeadline(ctx, fake, remoteOp(2*time.Second), 100*time.Millisecond)

	if got != 2*time.Second {
		t.Fatalf("EffectiveDeadline() = %v, want the method's 2s: with no balance the retry is already denied", got)
	}
}

func TestTheDecoratorBoundsTheCallByTheEffectiveDeadline(t *testing.T) {
	fake := clock.NewFake(start)
	var seen time.Duration
	inner := resilience.Call(func(ctx context.Context, _ resilience.Operation, _ func(context.Context) error) error {
		deadline, declared := ctx.Deadline()
		if !declared {
			t.Fatal("the inner call received a context with no deadline")
		}
		seen = deadline.Sub(fake.Now())
		return nil
	})

	decorated := resilience.Timeout(fake, nil, 0)(inner)
	if err := decorated(context.Background(), remoteOp(2*time.Second), nil); err != nil {
		t.Fatalf("decorated() = %v, want nil", err)
	}

	if seen != 2*time.Second {
		t.Fatalf("the inner deadline was %v away, want 2s", seen)
	}
}

func TestACallWithNoTimeLeftIsRefusedWithoutRunning(t *testing.T) {
	fake := clock.NewFake(start)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fake.Advance(time.Second)

	invoked := false
	instruments, total := meter(t)
	decorated := resilience.Timeout(fake, instruments, 0)(func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked = true
		return nil
	})

	err := decorated(ctx, remoteOp(2*time.Second), nil)

	if !errors.Is(err, resilience.ErrDeadlineExceeded) {
		t.Fatalf("decorated() = %v, want ErrDeadlineExceeded", err)
	}
	if invoked {
		t.Error("the inner call ran with no time left")
	}
	if got := total(metrics.DeadlineExceededTotal); got != 1 {
		t.Errorf("%s = %d, want 1 (RES-23)", metrics.DeadlineExceededTotal, got)
	}
}

func TestADeadlineAndACancellationAreCountedApart(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		fake := clock.NewFake(start)
		instruments, total := meter(t)
		decorated := resilience.Timeout(fake, instruments, 0)(func(ctx context.Context, _ resilience.Operation, _ func(context.Context) error) error {
			<-ctx.Done()
			return ctx.Err()
		})

		done := make(chan error, 1)
		go func() { done <- decorated(context.Background(), remoteOp(time.Second), nil) }()
		awaitAlarm(t, fake, 1)
		fake.Advance(time.Second)

		err := <-done
		if !errors.Is(err, resilience.ErrDeadlineExceeded) {
			t.Fatalf("decorated() = %v, want ErrDeadlineExceeded", err)
		}
		if got := total(metrics.DeadlineExceededTotal); got != 1 {
			t.Errorf("%s = %d, want 1", metrics.DeadlineExceededTotal, got)
		}
		if got := total(metrics.CancellationsTotal); got != 0 {
			t.Errorf("%s = %d, want 0 — running late is not the caller giving up", metrics.CancellationsTotal, got)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		fake := clock.NewFake(start)
		instruments, total := meter(t)
		caller, cancel := context.WithCancel(context.Background())
		decorated := resilience.Timeout(fake, instruments, 0)(func(ctx context.Context, _ resilience.Operation, _ func(context.Context) error) error {
			<-ctx.Done()
			return ctx.Err()
		})

		done := make(chan error, 1)
		go func() { done <- decorated(caller, remoteOp(time.Hour), nil) }()
		awaitAlarm(t, fake, 1)
		cancel()

		err := <-done
		if !errors.Is(err, resilience.ErrCancelled) {
			t.Fatalf("decorated() = %v, want ErrCancelled", err)
		}
		if got := total(metrics.CancellationsTotal); got != 1 {
			t.Errorf("%s = %d, want 1", metrics.CancellationsTotal, got)
		}
		if got := total(metrics.DeadlineExceededTotal); got != 0 {
			t.Errorf("%s = %d, want 0 — the caller gave up, we did not run late (CTX-28)", metrics.DeadlineExceededTotal, got)
		}
	})
}

func TestAFailureOfTheCallItselfIsNotReclassified(t *testing.T) {
	fake := clock.NewFake(start)
	broken := errors.New("connection reset by peer")
	instruments, total := meter(t)

	decorated := resilience.Timeout(fake, instruments, 0)(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return broken
	})

	err := decorated(context.Background(), remoteOp(time.Second), nil)

	if !errors.Is(err, broken) {
		t.Fatalf("decorated() = %v, want the call's own error", err)
	}
	if errors.Is(err, resilience.ErrDeadlineExceeded) || errors.Is(err, resilience.ErrCancelled) {
		t.Fatal("a failure of the call was reclassified as a deadline or a cancellation")
	}
	if got := total(metrics.DeadlineExceededTotal) + total(metrics.CancellationsTotal); got != 0 {
		t.Errorf("counted %d deadline or cancellation events for a plain failure, want 0", got)
	}
}

func TestTheDecoratorToleratesAbsentInstruments(t *testing.T) {
	fake := clock.NewFake(start)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fake.Advance(time.Second)

	decorated := resilience.Timeout(fake, nil, 0)(resilience.Direct)

	if err := decorated(ctx, remoteOp(time.Second), func(context.Context) error { return nil }); !errors.Is(err, resilience.ErrDeadlineExceeded) {
		t.Fatalf("decorated() = %v, want ErrDeadlineExceeded without a meter", err)
	}
}

// awaitAlarm waits until the fake clock holds the expected number of alarms, so
// the test never advances before the goroutine under test has armed its timer.
// The wall-clock bound is only a safety net against a hung test.
func awaitAlarm(t *testing.T, fake *clock.Fake, want int) {
	t.Helper()
	limit := time.Now().Add(2 * time.Second)
	for fake.Pending() != want {
		if time.Now().After(limit) {
			t.Fatalf("Pending() = %d, want %d", fake.Pending(), want)
		}
		runtime.Gosched()
	}
}
