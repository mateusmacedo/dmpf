package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

var errTransient = errors.New("transient")

func config(c *clock.Fake) compose.Config {
	sheet := resilience.Defaults("dep")
	sheet.Backoff = resilience.Declare(resilience.BackoffPolicy{Base: time.Millisecond, Factor: 2, Cap: 10 * time.Millisecond})
	return compose.Config{
		Sheet: sheet, Service: "svc", SpanPrefix: "dmpf.test ", Clock: c,
		Rand:       func() float64 { return 0 },
		Category:   func(error) string { return "transient" },
		Classifier: func(err error) retry.Retryability { return retry.Retryable },
	}
}

func TestBuildRetriesUnderTheConjunction(t *testing.T) {
	c := clock.NewFake(start)
	call, err := compose.Build(config(c))
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))

	attempts := 0
	err = call(ctx, compose.Operation(config(c), "op", true), func(context.Context) error {
		attempts++
		if attempts == 1 {
			return errTransient
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("call = %v after %d attempts, want nil after 2", err, attempts)
	}
}

func TestBuildWithoutBudgetMakesOneAttempt(t *testing.T) {
	c := clock.NewFake(start)
	call, _ := compose.Build(config(c))
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attempts := 0
	_ = call(ctx, compose.Operation(config(c), "op", true), func(context.Context) error { attempts++; return errTransient })
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 without retry.WithBudget (RES-31)", attempts)
	}
}

func TestRetryDeclaredOffIsTheIdentity(t *testing.T) {
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Sheet.Retry = resilience.Declare(false)
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))
	attempts := 0
	_ = call(ctx, compose.Operation(cfg, "op", true), func(context.Context) error { attempts++; return errTransient })
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 with retry declared off", attempts)
	}
}

func TestBuildRefusesADeclaredPositionWithoutDecorator(t *testing.T) {
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Sheet.RateLimit = resilience.Declare(resilience.RateLimitPolicy{PerSecond: 1, Burst: 1})
	if _, err := compose.Build(cfg); !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Build() = %v, want ErrBlankField (RES-21)", err)
	}
}

func TestOperationIsBoundedByTheSheet(t *testing.T) {
	c := clock.NewFake(start)
	op := compose.Operation(config(c), "publish x", true)
	if op.Deadline != resilience.DefaultDeadline || op.EstimatedDuration != resilience.DefaultDeadline/4 || !op.Idempotent || op.Kind != resilience.Remote {
		t.Fatalf("Operation = %+v", op)
	}
	if err := op.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}
