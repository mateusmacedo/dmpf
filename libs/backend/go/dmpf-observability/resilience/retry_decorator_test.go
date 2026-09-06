package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

func retryConfig() resilience.RetryConfig {
	return resilience.RetryConfig{
		Dependency:  "payments",
		Classifier:  func(error) retry.Retryability { return retry.Retryable },
		MaxAttempts: 3,
		Backoff:     resilience.BackoffPolicy{Base: 100 * time.Millisecond, Factor: 2, Cap: 5 * time.Second},
		Rand:        func() float64 { return 0 },
	}
}

// instant is a sleeper that returns at once, for the tests that are about the
// decision and not about the wait.
func instant(context.Context, time.Duration) error { return nil }

func retrying(t *testing.T, config resilience.RetryConfig, c clock.Clock, sleeper clock.Sleeper, instruments *metrics.Instruments, inner resilience.Call) resilience.Call {
	t.Helper()

	decorator, err := resilience.NewRetry(config, c, sleeper, instruments)
	if err != nil {
		t.Fatalf("NewRetry() = %v, want nil", err)
	}
	return decorator(inner)
}

func idempotent(deadline time.Duration) resilience.Operation {
	op := remoteOp(deadline)
	op.Idempotent = true
	return op
}

func TestARetryDecoratorRefusesToWrapAUnitOfWork(t *testing.T) {
	fake := clock.NewFake(start)
	invoked := 0
	call := retrying(t, retryConfig(), fake, instant, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		return nil
	})

	op := idempotent(time.Second)
	op.Kind = resilience.UnitOfWork
	err := call(context.Background(), op, nil)

	if !errors.Is(err, resilience.ErrWrapsUnitOfWork) {
		t.Fatalf("call() = %v, want ErrWrapsUnitOfWork (RES-25)", err)
	}
	if invoked != 0 {
		t.Errorf("the call ran %d times, want 0 — the refusal comes before the work", invoked)
	}
}

func TestASuccessOnTheFirstAttemptIsNotRetried(t *testing.T) {
	fake := clock.NewFake(start)
	instruments, read := meter(t)
	invoked := 0
	call := retrying(t, retryConfig(), fake, instant, instruments, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		return nil
	})

	if err := call(context.Background(), idempotent(time.Second), nil); err != nil {
		t.Fatalf("call() = %v, want nil", err)
	}
	if invoked != 1 {
		t.Errorf("the call ran %d times, want 1", invoked)
	}
	if got := read(metrics.RetriesTotal); got != 0 {
		t.Errorf("%s = %d, want 0", metrics.RetriesTotal, got)
	}
}

func TestATransientFailureIsRetriedUntilItSucceeds(t *testing.T) {
	fake := clock.NewFake(start)
	instruments, read := meter(t)
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(time.Minute))

	invoked := 0
	call := retrying(t, retryConfig(), fake, instant, instruments, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		if invoked < 3 {
			return errDependency
		}
		return nil
	})

	if err := call(ctx, idempotent(time.Minute), nil); err != nil {
		t.Fatalf("call() = %v, want nil on the third attempt", err)
	}
	if invoked != 3 {
		t.Errorf("the call ran %d times, want 3", invoked)
	}
	if got := read(metrics.RetriesTotal); got != 2 {
		t.Errorf("%s = %d, want 2 — one per repeated attempt (RES-23)", metrics.RetriesTotal, got)
	}
}

func TestTheAttemptCeilingStopsTheLoop(t *testing.T) {
	fake := clock.NewFake(start)
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(time.Hour))

	invoked := 0
	call := retrying(t, retryConfig(), fake, instant, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		return errDependency
	})

	err := call(ctx, idempotent(time.Hour), nil)

	if !errors.Is(err, errDependency) {
		t.Fatalf("call() = %v, want the dependency error", err)
	}
	if invoked != retryConfig().MaxAttempts {
		t.Fatalf("the call ran %d times, want %d — the ceiling counts the original (RES-33)", invoked, retryConfig().MaxAttempts)
	}
}

func TestANotRetryableFailureIsNotRepeated(t *testing.T) {
	fake := clock.NewFake(start)
	config := retryConfig()
	config.Classifier = func(error) retry.Retryability { return retry.NotRetryable }

	invoked := 0
	call := retrying(t, config, fake, instant, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		return errDependency
	})

	_ = call(retry.WithBudget(context.Background(), retry.WithTotal(time.Hour)), idempotent(time.Hour), nil)

	if invoked != 1 {
		t.Fatalf("the call ran %d times, want 1", invoked)
	}
}

func TestWithoutABudgetNothingIsRetried(t *testing.T) {
	fake := clock.NewFake(start)

	invoked := 0
	call := retrying(t, retryConfig(), fake, instant, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		return errDependency
	})

	_ = call(context.Background(), idempotent(time.Hour), nil)

	if invoked != 1 {
		t.Fatalf("the call ran %d times, want 1 — no budget means factor 3 is false", invoked)
	}
}

func TestTheWaitIsDebitedBeforeSleeping(t *testing.T) {
	fake := clock.NewFake(start)
	config := retryConfig()
	config.Rand = func() float64 { return 1 }

	// Every observation is kept, not just the last: the sequence shows that each
	// wait was already debited when the process went to sleep for it.
	var balances []time.Duration
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(time.Second))
	budget, _ := retry.BudgetFrom(ctx)

	sleeper := func(context.Context, time.Duration) error {
		balances = append(balances, budget.Remaining())
		return nil
	}
	call := retrying(t, config, fake, sleeper, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})

	// A small estimate on purpose: remoteOp derives it from the deadline, and an
	// hour-long deadline would make the estimate alone exceed the budget, so the
	// budget factor would deny before the sleeper ever ran.
	op := idempotent(time.Hour)
	op.EstimatedDuration = 50 * time.Millisecond
	_ = call(ctx, op, nil)

	// A budget of 1s, waits of 100ms and 200ms, an estimate of 50ms and a ceiling
	// of 3 attempts. What is claimed before each sleep is the whole attempt —
	// the wait plus the estimate — because reserving is what stops two
	// dependencies from spending the same balance (RES-30). The 50ms of estimate
	// comes back when the attempt returns faster than that, which is why the
	// second balance starts from 900ms and not from 850ms.
	want := []time.Duration{850 * time.Millisecond, 650 * time.Millisecond}
	if len(balances) != len(want) {
		t.Fatalf("slept %d times with balances %v, want %d", len(balances), balances, len(want))
	}
	for i := range want {
		if balances[i] != want[i] {
			t.Fatalf("balances = %v, want %v — the attempt is claimed before sleeping for its wait (RES-31, RES-32)", balances, want)
		}
	}

	// And the settlement gave back what the attempts did not spend.
	if got := budget.Remaining(); got != 700*time.Millisecond {
		t.Errorf("Remaining() = %v, want 700ms: each attempt returned 50ms of unspent estimate", got)
	}
}

func TestTheDecoratorWaitsOnTheInjectedSleeper(t *testing.T) {
	fake := clock.NewFake(start)
	config := retryConfig()
	config.Rand = func() float64 { return 1 }
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(time.Minute))

	invoked := 0
	call := retrying(t, config, fake, clock.NewSleeper(fake), nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		if invoked < 2 {
			return errDependency
		}
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- call(ctx, idempotent(time.Minute), nil) }()
	awaitAlarm(t, fake, 1)

	fake.Advance(100 * time.Millisecond)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("call() = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the decorator never returned after the wait elapsed")
	}
	if invoked != 2 {
		t.Errorf("the call ran %d times, want 2", invoked)
	}
}

func TestEachRepeatedAttemptLeavesAnEventOnTheSpan(t *testing.T) {
	fake := clock.NewFake(start)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	ctx, span := provider.Tracer("dmpf-observability").Start(
		retry.WithBudget(context.Background(), retry.WithTotal(time.Hour)), "under.test")

	call := retrying(t, retryConfig(), fake, instant, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	_ = call(ctx, idempotent(time.Hour), nil)
	span.End()

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	attempts := 0
	for _, event := range ended[0].Events() {
		if event.Name == tracing.EventAttempt {
			attempts++
		}
	}
	if attempts != retryConfig().MaxAttempts-1 {
		t.Fatalf("the span carries %d attempt events, want %d (TRC-11)", attempts, retryConfig().MaxAttempts-1)
	}
}

func TestAnExhaustedBudgetMarksTheSpanAndIsCountedOnce(t *testing.T) {
	fake := clock.NewFake(start)
	instruments, read := meter(t)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	// A budget that covers one estimate and one wait, and no more.
	op := idempotent(time.Hour)
	op.EstimatedDuration = 50 * time.Millisecond
	config := retryConfig()
	config.MaxAttempts = 10
	config.Rand = func() float64 { return 1 }

	ctx, span := provider.Tracer("dmpf-observability").Start(
		retry.WithBudget(context.Background(), retry.WithTotal(150*time.Millisecond)), "under.test")

	call := retrying(t, config, fake, instant, instruments, func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	_ = call(ctx, op, nil)
	span.End()

	if got := read(metrics.BudgetExhaustedTotal); got != 1 {
		t.Fatalf("%s = %d, want exactly 1 per execution (RES-36)", metrics.BudgetExhaustedTotal, got)
	}

	marked := false
	for _, kv := range recorder.Ended()[0].Attributes() {
		if string(kv.Key) == resilience.AttrBudgetExhausted && kv.Value.AsBool() {
			marked = true
		}
	}
	if !marked {
		t.Fatalf("the span carries no %s attribute (RES-36)", resilience.AttrBudgetExhausted)
	}
}

func TestARetriedDependencyNeverReopensTheUnitOfWork(t *testing.T) {
	fake := clock.NewFake(start)
	store := memory.New()
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) *memory.Tx { return tx })

	// The retry decorator guards a dependency called from inside the
	// transaction, which is the arrangement RES-25 is about.
	attempts := 0
	guarded := retrying(t, retryConfig(), fake, instant, nil, func(context.Context, resilience.Operation, func(context.Context) error) error {
		attempts++
		if attempts < 3 {
			return errDependency
		}
		return nil
	})

	ctx := retry.WithBudget(context.Background(), retry.WithTotal(time.Minute))
	err := uow.Within(ctx, func(ctx context.Context, tx *memory.Tx) error {
		if err := guarded(ctx, idempotent(time.Minute), nil); err != nil {
			return err
		}
		return tx.Orders().Save(ctx, orders.OrderID("P-100"), openSnapshotFor("P-100"), dmpfports.Version(0))
	})

	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}
	if attempts != 3 {
		t.Errorf("the dependency ran %d times, want 3 — the retry is the dependency's", attempts)
	}
	if got := store.WithinCalls(); got != 1 {
		t.Fatalf("WithinCalls() = %d, want 1 — retry never reopens the transaction (RES-25, UOW-09)", got)
	}
	if got := store.Commits(); got != 1 {
		t.Errorf("Commits() = %d, want 1", got)
	}
}

func openSnapshotFor(id orders.OrderID) orders.Snapshot {
	return orders.Snapshot{
		ID:        id,
		Status:    orders.Open,
		ItemLimit: 3,
		Items:     []orders.Item{{SKU: "A", Quantity: 1}},
	}
}

func TestNewRetryRefusesAnAbsentCeilingOrClock(t *testing.T) {
	fake := clock.NewFake(start)

	noCeiling := retryConfig()
	noCeiling.MaxAttempts = 0
	if _, err := resilience.NewRetry(noCeiling, fake, instant, nil); !errors.Is(err, resilience.ErrBlankField) {
		t.Errorf("NewRetry() with no ceiling = %v, want a refusal (RES-33)", err)
	}
	if _, err := resilience.NewRetry(retryConfig(), nil, instant, nil); !errors.Is(err, resilience.ErrBlankField) {
		t.Errorf("NewRetry() with no clock = %v, want a refusal", err)
	}
	if _, err := resilience.NewRetry(retryConfig(), fake, nil, nil); !errors.Is(err, resilience.ErrBlankField) {
		t.Errorf("NewRetry() with no sleeper = %v, want a refusal", err)
	}
}

// The budget belongs to the execution, not to the call: three dependencies
// failing in turn draw on the same balance, and the one that finds it spent is
// denied without attempting again. The exhaustion is counted once for the
// execution, not once per dependency (RES-30, RES-36).
//
// The scenario is the one the spec states: a remaining deadline of 2s at the
// first failure, which sizes the budget at 1s, and a backoff base of 100ms.
func TestThreeDependenciesShareOneBudgetUntilItIsSpent(t *testing.T) {
	fake := clock.NewFake(start)
	instruments, read := meter(t)

	config := retryConfig()
	config.MaxAttempts = 10 // high enough that the budget, not the ceiling, decides
	config.Rand = func() float64 { return 1 }

	// A deadline of 2s on the context is what arms the budget at half of it.
	ctx, cancel := fake.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx)
	budget, _ := retry.BudgetFrom(ctx)

	op := idempotent(time.Hour)
	op.EstimatedDuration = 300 * time.Millisecond

	attempts := make([]int, 3)
	for dependency := range attempts {
		config.Dependency = "dep-" + string(rune('a'+dependency))
		call := retrying(t, config, fake, instant, instruments,
			func(context.Context, resilience.Operation, func(context.Context) error) error {
				attempts[dependency]++
				return errDependency
			})
		_ = call(ctx, op, nil)
	}

	if got := budget.Remaining(); got >= 400*time.Millisecond {
		t.Errorf("Remaining() = %v, want less than one attempt's claim of 400ms", got)
	}
	if attempts[2] != 1 {
		t.Errorf("attempts per dependency = %v; the third one attempted %d times, want 1 — "+
			"the balance the first two spent is the same balance it draws on",
			attempts, attempts[2])
	}
	if got := read(metrics.BudgetExhaustedTotal); got != 1 {
		t.Errorf("%s = %d, want exactly 1 for the execution (RES-36)", metrics.BudgetExhaustedTotal, got)
	}
}
