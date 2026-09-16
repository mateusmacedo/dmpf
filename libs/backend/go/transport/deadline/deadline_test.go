package deadline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func budget(limit, slack time.Duration) deadline.Budget {
	return deadline.Budget{
		Dependency:        "orders",
		Method:            "Reserve",
		Limit:             limit,
		Slack:             slack,
		EstimatedDuration: limit / 4,
	}
}

func withDeadline(t *testing.T, c *clock.Fake, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := c.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func TestRequire(t *testing.T) {
	t.Run("context without deadline is refused", func(t *testing.T) {
		_, err := deadline.Require(context.Background())
		if !errors.Is(err, deadline.ErrNoDeadline) {
			t.Fatalf("Require() = %v, want ErrNoDeadline", err)
		}
	})

	t.Run("context with deadline returns it", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, time.Second)

		got, err := deadline.Require(ctx)
		if err != nil {
			t.Fatalf("Require() = %v, want nil", err)
		}
		if want := start.Add(time.Second); !got.Equal(want) {
			t.Fatalf("Require() = %v, want %v", got, want)
		}
	})
}

func TestBudgetOperation(t *testing.T) {
	op := budget(time.Second, 100*time.Millisecond).Operation()

	if op.Kind != resilience.Remote {
		t.Errorf("Operation().Kind = %q, want %q", op.Kind, resilience.Remote)
	}
	if op.Deadline != time.Second {
		t.Errorf("Operation().Deadline = %v, want %v", op.Deadline, time.Second)
	}
	if op.Dependency != "orders" || op.Method != "Reserve" {
		t.Errorf("Operation() = %s.%s, want orders.Reserve", op.Dependency, op.Method)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("Operation().Validate() = %v, want nil", err)
	}
}

func TestBudgetValidate(t *testing.T) {
	cases := map[string]struct {
		budget deadline.Budget
		wantOK bool
	}{
		"complete":                  {budget(time.Second, 100*time.Millisecond), true},
		"estimated duration absent": {deadline.Budget{Dependency: "orders", Method: "Reserve", Limit: time.Second, Slack: time.Millisecond}, false},
		"slack absent":              {deadline.Budget{Dependency: "orders", Method: "Reserve", Limit: time.Second, EstimatedDuration: time.Millisecond}, false},
		"slack negative":            {budget(time.Second, -time.Millisecond), false},
		"limit absent":              {deadline.Budget{Dependency: "orders", Method: "Reserve", Slack: time.Millisecond, EstimatedDuration: time.Millisecond}, false},
		"method absent":             {deadline.Budget{Dependency: "orders", Limit: time.Second, Slack: time.Millisecond, EstimatedDuration: time.Millisecond}, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.budget.Validate()
			if tc.wantOK && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
		})
	}
}

func TestOutgoing(t *testing.T) {
	t.Run("context without deadline is refused before any derivation", func(t *testing.T) {
		c := clock.NewFake(start)

		_, err := deadline.Outgoing(context.Background(), c, budget(time.Second, 100*time.Millisecond))
		if !errors.Is(err, deadline.ErrNoDeadline) {
			t.Fatalf("Outgoing() = %v, want ErrNoDeadline", err)
		}
	})

	t.Run("invalid budget is refused", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, time.Second)

		_, err := deadline.Outgoing(ctx, c, budget(time.Second, 0))
		if err == nil {
			t.Fatal("Outgoing() = nil, want error for slack absent")
		}
	})

	t.Run("a larger limit never extends the caller's deadline", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, time.Second)

		got, err := deadline.Outgoing(ctx, c, budget(10*time.Second, 100*time.Millisecond))
		if err != nil {
			t.Fatalf("Outgoing() = %v, want nil", err)
		}
		if want := start.Add(900 * time.Millisecond); !got.Equal(want) {
			t.Fatalf("Outgoing() = %v, want %v", got, want)
		}
	})

	t.Run("a smaller limit bounds the hop below the caller's deadline", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, 10*time.Second)

		got, err := deadline.Outgoing(ctx, c, budget(time.Second, 100*time.Millisecond))
		if err != nil {
			t.Fatalf("Outgoing() = %v, want nil", err)
		}
		if want := start.Add(900 * time.Millisecond); !got.Equal(want) {
			t.Fatalf("Outgoing() = %v, want %v", got, want)
		}
	})

	t.Run("the result never exceeds the caller's deadline as time passes", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, time.Second)
		callerDeadline, _ := ctx.Deadline()

		for _, step := range []time.Duration{0, 300 * time.Millisecond, 300 * time.Millisecond} {
			c.Advance(step)
			elapsed := c.Now().Sub(start)
			got, err := deadline.Outgoing(ctx, c, budget(5*time.Second, 100*time.Millisecond))
			if err != nil {
				t.Fatalf("Outgoing() after %v = %v, want nil", elapsed, err)
			}
			if got.After(callerDeadline) {
				t.Fatalf("Outgoing() after %v = %v, exceeds caller deadline %v", elapsed, got, callerDeadline)
			}
		}
	})

	t.Run("remaining time equal to the slack is exhausted", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, 100*time.Millisecond)

		_, err := deadline.Outgoing(ctx, c, budget(time.Second, 100*time.Millisecond))
		if !errors.Is(err, deadline.ErrDeadlineExhausted) {
			t.Fatalf("Outgoing() = %v, want ErrDeadlineExhausted", err)
		}
	})

	t.Run("remaining time below the slack is exhausted", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, 50*time.Millisecond)

		_, err := deadline.Outgoing(ctx, c, budget(time.Second, 100*time.Millisecond))
		if !errors.Is(err, deadline.ErrDeadlineExhausted) {
			t.Fatalf("Outgoing() = %v, want ErrDeadlineExhausted", err)
		}
	})

	t.Run("an expired deadline is exhausted", func(t *testing.T) {
		c := clock.NewFake(start)
		ctx := withDeadline(t, c, time.Second)
		c.Advance(2 * time.Second)

		_, err := deadline.Outgoing(ctx, c, budget(time.Second, 100*time.Millisecond))
		if !errors.Is(err, deadline.ErrDeadlineExhausted) {
			t.Fatalf("Outgoing() = %v, want ErrDeadlineExhausted", err)
		}
	})
}
