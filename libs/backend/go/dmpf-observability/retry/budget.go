package retry

import (
	"context"
	"sync/atomic"
	"time"
)

// Budget is the retry budget of one execution, shared by every dependency the
// execution touches (RES-30, RES-31). It is the only value the kernel admits in
// a context, and its absence only restricts: no budget means no retry.
//
// Safe for concurrent use: several dependencies of the same execution debit it
// at once (RES-36).
type Budget struct {
	remaining atomic.Int64
	armed     atomic.Bool
	reported  atomic.Bool
}

type budgetKey struct{}

// Option configures the budget at the edge.
type Option func(*Budget)

// WithTotal declares the budget explicitly. Without it the budget is armed on
// the first failure with half of the remaining deadline (RES-31).
func WithTotal(total time.Duration) Option {
	return func(b *Budget) {
		if total > 0 {
			b.remaining.Store(int64(total))
			b.armed.Store(true)
		}
	}
}

// WithBudget puts a budget in the context. Only the edge calls it: the use case
// receives the context and never mints a policy of its own (RES-24).
func WithBudget(ctx context.Context, options ...Option) context.Context {
	budget := &Budget{}
	for _, apply := range options {
		apply(budget)
	}
	return context.WithValue(ctx, budgetKey{}, budget)
}

// BudgetFrom reads the budget of the execution. The false result is what makes
// factor 3 false, so an execution without a budget never retries.
func BudgetFrom(ctx context.Context) (*Budget, bool) {
	budget, ok := ctx.Value(budgetKey{}).(*Budget)
	return budget, ok
}

// Arm sizes an undeclared budget as half of the remaining deadline, on the first
// failure. It is idempotent: only the first call sizes the budget, so a second
// dependency failing later does not reset what the first already spent.
func (b *Budget) Arm(remaining time.Duration) {
	if remaining <= 0 {
		return
	}
	if b.armed.CompareAndSwap(false, true) {
		b.remaining.Store(int64(remaining / 2))
	}
}

// Debit takes time out of the budget: both the backoff wait and the duration of
// the repeated attempt are debited (RES-31). It never goes below zero.
func (b *Budget) Debit(spent time.Duration) {
	if spent <= 0 {
		return
	}

	for {
		current := b.remaining.Load()
		next := current - int64(spent)
		if next < 0 {
			next = 0
		}
		if b.remaining.CompareAndSwap(current, next) {
			return
		}
	}
}

// Remaining is the balance. An unarmed budget reports zero, which denies the
// retry rather than allowing one against a balance nobody sized.
func (b *Budget) Remaining() time.Duration {
	if !b.armed.Load() {
		return 0
	}
	return time.Duration(b.remaining.Load())
}

// Exhausted reports a balance that reached zero, once. It latches: the first
// call after the balance is gone returns true and every later call returns
// false, so the caller counts the exhaustion once per execution instead of once
// per attempt.
func (b *Budget) Exhausted() bool {
	if !b.armed.Load() || b.remaining.Load() > 0 {
		return false
	}
	return b.reported.CompareAndSwap(false, true)
}

// ReportExhaustion latches the same report for a caller that already knows the
// budget ran out. It exists because a balance above zero can still be too small
// for another attempt, and only the caller knows what an attempt costs: the
// budget would report nothing while the retry was already denied for lack of
// funds (RES-36).
//
// It shares the latch with Exhausted, so an execution reports at most once
// whichever of the two observes it first.
func (b *Budget) ReportExhaustion() bool {
	if !b.armed.Load() {
		return false
	}
	return b.reported.CompareAndSwap(false, true)
}
