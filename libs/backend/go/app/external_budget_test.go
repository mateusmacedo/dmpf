package app_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

func validBudget() app.ExternalBudget {
	return app.ExternalBudget{
		Route:      deadline.Budget{Dependency: "partner", Method: "submit", Limit: 2 * time.Second, Slack: 50 * time.Millisecond},
		Settle:     time.Second,
		Processing: 5 * time.Second,
		Margin:     app.MinExternalMargin,
	}
}

func TestExternalBudgetValidateAcceptsCoherentBudget(t *testing.T) {
	if err := validBudget().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestExternalBudgetValidateRefuses(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*app.ExternalBudget)
	}{
		{"route limit zero", func(b *app.ExternalBudget) { b.Route.Limit = 0 }},
		{"route limit negative", func(b *app.ExternalBudget) { b.Route.Limit = -time.Second }},
		{"settle zero", func(b *app.ExternalBudget) { b.Settle = 0 }},
		{"settle negative", func(b *app.ExternalBudget) { b.Settle = -time.Second }},
		{"processing zero", func(b *app.ExternalBudget) { b.Processing = 0 }},
		{"processing negative", func(b *app.ExternalBudget) { b.Processing = -time.Second }},
		{"margin zero", func(b *app.ExternalBudget) { b.Margin = 0 }},
		{"margin negative", func(b *app.ExternalBudget) { b.Margin = -time.Second }},
		{"margin just below the floor", func(b *app.ExternalBudget) { b.Margin = app.MinExternalMargin - time.Nanosecond }},
		{"limit plus settle overflows", func(b *app.ExternalBudget) { b.Route.Limit, b.Settle = math.MaxInt64, math.MaxInt64 }},
		{"settle overflows past processing", func(b *app.ExternalBudget) { b.Settle = math.MaxInt64 }},
		{"stale cut overflows", func(b *app.ExternalBudget) { b.Margin = math.MaxInt64 }},
		{"limit plus settle equals processing", func(b *app.ExternalBudget) { b.Processing = b.Route.Limit + b.Settle }},
		{"limit plus settle exceeds processing", func(b *app.ExternalBudget) { b.Processing = b.Route.Limit + b.Settle - time.Millisecond }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := validBudget()
			tc.mutate(&b)
			if err := b.Validate(); !errors.Is(err, app.ErrInvalidExternalBudget) {
				t.Fatalf("Validate() = %v, want ErrInvalidExternalBudget", err)
			}
		})
	}
}

func TestExternalBudgetValidateAcceptsTheMarginFloor(t *testing.T) {
	b := validBudget()
	b.Margin = app.MinExternalMargin
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() with margin %v = %v, want nil", b.Margin, err)
	}
}

func TestExternalBudgetStaleAfterSaturatesInsteadOfOverflowing(t *testing.T) {
	b := validBudget()
	b.Margin = math.MaxInt64
	if got := b.StaleAfter(); got != math.MaxInt64 {
		t.Fatalf("StaleAfter() = %v, want %v", got, time.Duration(math.MaxInt64))
	}
}

func TestExternalBudgetStaleAfterSumsLimitSettleAndMargin(t *testing.T) {
	b := validBudget()
	want := b.Route.Limit + b.Settle + b.Margin
	if got := b.StaleAfter(); got != want {
		t.Fatalf("StaleAfter() = %v, want %v", got, want)
	}
}

func TestExternalBudgetSettleContextBoundsBySettle(t *testing.T) {
	b := validBudget()
	before := time.Now()
	ctx, cancel := b.SettleContext()(context.Background())
	defer cancel()
	after := time.Now()

	got, ok := ctx.Deadline()
	if !ok {
		t.Fatal("SettleContext() context has no deadline")
	}
	if got.Before(before.Add(b.Settle)) || got.After(after.Add(b.Settle)) {
		t.Fatalf("deadline %v outside [%v, %v]", got, before.Add(b.Settle), after.Add(b.Settle))
	}
}

func TestExternalBudgetSettleContextIgnoresParentCancellation(t *testing.T) {
	b := validBudget()
	parent, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	ctx, cancel := b.SettleContext()(context.WithoutCancel(parent))
	defer cancel()
	if err := ctx.Err(); err != nil {
		t.Fatalf("settle context Err() = %v, want nil over a WithoutCancel parent", err)
	}
}
