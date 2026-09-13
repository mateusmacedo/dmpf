package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
)

func degrading(t *testing.T, mode resilience.Degradation, instruments *metrics.Instruments, outcome error) resilience.Call {
	t.Helper()

	decorator, err := resilience.NewDegradation(mode, "payments", instruments)
	if err != nil {
		t.Fatalf("NewDegradation(%q) = %v, want nil", mode, err)
	}
	return decorator(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return outcome
	})
}

func TestFailPropagatesTheFailureUntouched(t *testing.T) {
	call := degrading(t, resilience.Fail, nil, errDependency)

	err := call(context.Background(), remoteOp(time.Second), nil)

	if !errors.Is(err, errDependency) {
		t.Fatalf("call() = %v, want the dependency error", err)
	}
	var degraded *resilience.DegradedResult
	if errors.As(err, &degraded) {
		t.Fatal("the failure was wrapped as degraded under the fail mode")
	}
}

func TestAnUndeclaredModeBehavesAsFail(t *testing.T) {
	call := degrading(t, "", nil, errDependency)

	if err := call(context.Background(), remoteOp(time.Second), nil); !errors.Is(err, errDependency) {
		t.Fatalf("call() = %v, want the dependency error — the platform default is to fail", err)
	}
}

func TestDegradeAnswersWithADistinguishableResult(t *testing.T) {
	instruments, read := meter(t)
	call := degrading(t, resilience.Degrade, instruments, errDependency)

	err := call(context.Background(), remoteOp(time.Second), nil)

	var degraded *resilience.DegradedResult
	if !errors.As(err, &degraded) {
		t.Fatalf("call() = %v, want a *DegradedResult (RES-37)", err)
	}
	if degraded.Dependency != "payments" {
		t.Errorf("Dependency = %q, want \"payments\"", degraded.Dependency)
	}
	if !errors.Is(err, errDependency) {
		t.Error("the cause did not survive: a caller that wants the original failure must find it")
	}
	if got := read(metrics.DegradedTotal); got != 1 {
		t.Errorf("%s = %d, want 1 (RES-38)", metrics.DegradedTotal, got)
	}
}

func TestDegradingIsNotSilent(t *testing.T) {
	call := degrading(t, resilience.Degrade, nil, errDependency)

	err := call(context.Background(), remoteOp(time.Second), nil)

	if err == nil {
		t.Fatal("call() = nil under the degrade mode: degrading must not be silent (RES-38)")
	}
}

func TestIgnoreOmitsTheDependencyAndCountsTheOmission(t *testing.T) {
	instruments, read := meter(t)
	call := degrading(t, resilience.Ignore, instruments, errDependency)

	err := call(context.Background(), remoteOp(time.Second), nil)

	if err != nil {
		t.Fatalf("call() = %v, want nil — the ignore mode omits the dependency", err)
	}
	if got := read(metrics.OmittedTotal); got != 1 {
		t.Errorf("%s = %d, want 1: the omission is what keeps ignore from being silent (RES-38)", metrics.OmittedTotal, got)
	}
}

func TestDeferIsRefusedAtConstruction(t *testing.T) {
	decorator, err := resilience.NewDegradation(resilience.Defer, "payments", nil)

	if !errors.Is(err, resilience.ErrDeferIsOutbox) {
		t.Fatalf("NewDegradation(defer) = %v, want ErrDeferIsOutbox", err)
	}
	if decorator != nil {
		t.Error("NewDegradation(defer) returned a decorator alongside the refusal")
	}
}

func TestAnUnknownModeIsRefusedAtConstruction(t *testing.T) {
	if _, err := resilience.NewDegradation("liquidar", "payments", nil); err == nil {
		t.Fatal("NewDegradation() = nil error for an unknown mode, want a refusal")
	}
}

func TestASuccessIsNeverDegraded(t *testing.T) {
	instruments, read := meter(t)

	for _, mode := range []resilience.Degradation{resilience.Fail, resilience.Degrade, resilience.Ignore} {
		call := degrading(t, mode, instruments, nil)
		if err := call(context.Background(), remoteOp(time.Second), nil); err != nil {
			t.Fatalf("call() = %v under %q, want nil", err, mode)
		}
	}

	if got := read(metrics.DegradedTotal) + read(metrics.OmittedTotal); got != 0 {
		t.Fatalf("counted %d degradations for successful calls, want 0", got)
	}
}

func TestACancellationByTheCallerIsNotDegraded(t *testing.T) {
	instruments, read := meter(t)

	for _, mode := range []resilience.Degradation{resilience.Degrade, resilience.Ignore} {
		t.Run(string(mode), func(t *testing.T) {
			call := degrading(t, mode, instruments, resilience.ErrCancelled)

			err := call(context.Background(), remoteOp(time.Second), nil)

			if !errors.Is(err, resilience.ErrCancelled) {
				t.Fatalf("call() = %v, want the cancellation through: nobody is listening to degrade for", err)
			}
		})
	}

	if got := read(metrics.DegradedTotal) + read(metrics.OmittedTotal); got != 0 {
		t.Fatalf("counted %d degradations for cancellations, want 0 — the dependency is not to blame", got)
	}
}

func TestACancelledContextIsNotDegradedEitherWayAround(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	call := degrading(t, resilience.Ignore, nil, errDependency)

	err := call(ctx, remoteOp(time.Second), nil)

	if !errors.Is(err, errDependency) {
		t.Fatalf("call() = %v, want the failure through: the caller had already given up", err)
	}
}

func TestADegradedResultIsReadableByRedaction(t *testing.T) {
	degraded := &resilience.DegradedResult{Dependency: "payments", Cause: errDependency}

	var categorized redact.Categorized
	if !errors.As(error(degraded), &categorized) {
		t.Fatal("a DegradedResult is not readable by redaction: the category would not reach a log")
	}
	if got := categorized.ErrorCategory(); got != resilience.CategoryDegraded {
		t.Errorf("ErrorCategory() = %q, want %q", got, resilience.CategoryDegraded)
	}
	if categorized.ErrorCode() == "" {
		t.Error("ErrorCode() is empty")
	}
}
