package retry_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
)

func TestBudgetFromReportsFalseWithoutABudget(t *testing.T) {
	budget, ok := retry.BudgetFrom(context.Background())

	if ok {
		t.Fatal("BudgetFrom() reported a budget on a bare context")
	}
	if budget != nil {
		t.Fatalf("BudgetFrom() = %v, want nil", budget)
	}
}

func TestADeclaredTotalIsTheBalance(t *testing.T) {
	budget := armedBudget(t, 2*time.Second)

	if got := budget.Remaining(); got != 2*time.Second {
		t.Fatalf("Remaining() = %v, want 2s", got)
	}
}

func TestAnUnarmedBudgetReportsNoBalance(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, ok := retry.BudgetFrom(ctx)
	if !ok {
		t.Fatal("BudgetFrom() reported no budget right after WithBudget")
	}

	if got := budget.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %v, want 0 — an unsized budget denies the retry", got)
	}
}

func TestArmTakesHalfOfTheRemainingDeadline(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)

	budget.Arm(2 * time.Second)

	if got := budget.Remaining(); got != time.Second {
		t.Fatalf("Remaining() = %v, want 1s — half of the remaining deadline (RES-31)", got)
	}
}

func TestArmSizesOnlyOnce(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)
	budget.Arm(2 * time.Second)
	budget.Debit(400 * time.Millisecond)

	budget.Arm(10 * time.Second)

	if got := budget.Remaining(); got != 600*time.Millisecond {
		t.Fatalf("Remaining() = %v, want 600ms — a second Arm must not reset what was spent", got)
	}
}

func TestArmIgnoresANonPositiveDeadline(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)

	budget.Arm(0)

	if got := budget.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %v, want 0", got)
	}
}

func TestADeclaredTotalSurvivesArm(t *testing.T) {
	budget := armedBudget(t, 2*time.Second)

	budget.Arm(10 * time.Second)

	if got := budget.Remaining(); got != 2*time.Second {
		t.Fatalf("Remaining() = %v, want the declared 2s — Arm only sizes an undeclared budget", got)
	}
}

func TestDebitNeverGoesBelowZero(t *testing.T) {
	budget := armedBudget(t, time.Second)

	budget.Debit(5 * time.Second)

	if got := budget.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %v, want 0", got)
	}
}

func TestExhaustedReportsOnlyOnce(t *testing.T) {
	budget := armedBudget(t, time.Second)

	if budget.Exhausted() {
		t.Fatal("Exhausted() reported exhaustion while the balance was positive")
	}

	budget.Debit(time.Second)

	if !budget.Exhausted() {
		t.Fatal("Exhausted() = false on the first call after the balance reached zero")
	}
	if budget.Exhausted() {
		t.Fatal("Exhausted() = true twice: the metric must be incremented once per execution (RES-36)")
	}
}

func TestAnUnarmedBudgetIsNotExhausted(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)

	if budget.Exhausted() {
		t.Fatal("Exhausted() = true on an unsized budget: nothing was spent")
	}
}

func TestTheBudgetIsSharedByEveryDependencyOfTheExecution(t *testing.T) {
	const dependencies = 64
	const each = 10 * time.Millisecond
	budget := armedBudget(t, dependencies*each)

	var wg sync.WaitGroup
	for range dependencies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			budget.Debit(each)
		}()
	}
	wg.Wait()

	if got := budget.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %v, want 0 — every debit must land (RES-36)", got)
	}
}

func TestExhaustedLatchesOnceUnderConcurrency(t *testing.T) {
	budget := armedBudget(t, time.Millisecond)
	budget.Debit(time.Millisecond)

	var reports int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if budget.Exhausted() {
				mu.Lock()
				reports++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if reports != 1 {
		t.Fatalf("Exhausted() reported %d times under concurrency, want exactly 1", reports)
	}
}

func TestReportExhaustionLatchesWithABalanceStillAboveZero(t *testing.T) {
	budget := armedBudget(t, time.Second)

	if !budget.ReportExhaustion() {
		t.Fatal("ReportExhaustion() = false on the first call, want true: a balance above zero can still be too small for another attempt")
	}
	if budget.ReportExhaustion() {
		t.Fatal("ReportExhaustion() = true twice, want the report to latch")
	}
}

func TestReportExhaustionSharesTheLatchWithExhausted(t *testing.T) {
	t.Run("report first", func(t *testing.T) {
		budget := armedBudget(t, time.Second)

		if !budget.ReportExhaustion() {
			t.Fatal("ReportExhaustion() = false on the first call")
		}
		budget.Debit(time.Second)
		if budget.Exhausted() {
			t.Fatal("Exhausted() = true after ReportExhaustion already reported: an execution must report at most once")
		}
	})

	t.Run("exhausted first", func(t *testing.T) {
		budget := armedBudget(t, time.Second)
		budget.Debit(time.Second)

		if !budget.Exhausted() {
			t.Fatal("Exhausted() = false with the balance gone")
		}
		if budget.ReportExhaustion() {
			t.Fatal("ReportExhaustion() = true after Exhausted already reported")
		}
	})
}

func TestAnUnarmedBudgetReportsNoExhaustion(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)

	if budget.ReportExhaustion() {
		t.Fatal("ReportExhaustion() = true on an unsized budget: nothing was spent")
	}
}

func TestReportExhaustionLatchesOnceUnderConcurrency(t *testing.T) {
	budget := armedBudget(t, time.Second)

	var reports int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if budget.ReportExhaustion() {
				mu.Lock()
				reports++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if reports != 1 {
		t.Fatalf("ReportExhaustion() reported %d times under concurrency, want exactly 1", reports)
	}
}

// The budget is shared by every dependency of one execution (RES-30). Reading
// the balance and debiting it afterwards lets two dependencies that fail at the
// same time both see enough funds and both retry, spending twice what the
// execution was allowed.
//
// This test states the invariant; it does not prove the absence of the race.
// The window between a load and a later store is a few instructions wide, and
// measured at roughly one round in a hundred — a test that depended on hitting
// it would be flaky, and the race detector does not see it, because atomics
// make it a logical race and not a data race. What rules it out is the
// construction: Reserve decides and spends in one compare-and-swap.
func TestTwoDependenciesCannotBothSpendTheSameBalance(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)
	budget.Arm(2 * time.Second) // balance: 1s

	const need = 600 * time.Millisecond
	const dependencies = 8

	granted := make(chan bool, dependencies)
	var racing sync.WaitGroup
	var start sync.WaitGroup
	start.Add(1)

	for range dependencies {
		racing.Add(1)
		go func() {
			defer racing.Done()
			start.Wait()
			granted <- budget.Reserve(need)
		}()
	}

	start.Done()
	racing.Wait()
	close(granted)

	allowed := 0
	for ok := range granted {
		if ok {
			allowed++
		}
	}

	// 1s of balance covers one attempt of 600ms, never two.
	if allowed != 1 {
		t.Errorf("Reserve(%v) succeeded %d times against a balance of %v, want 1",
			need, allowed, time.Second)
	}
	if got := budget.Remaining(); got != time.Second-need {
		t.Errorf("Remaining() = %v, want %v: exactly one reservation was paid for", got, time.Second-need)
	}
}

func TestReserveRefusesWhatTheBalanceCannotCover(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)
	budget.Arm(2 * time.Second) // balance: 1s

	if budget.Reserve(2 * time.Second) {
		t.Error("Reserve() granted more than the balance holds")
	}
	if got := budget.Remaining(); got != time.Second {
		t.Errorf("Remaining() = %v, want the balance untouched by a refused reservation", got)
	}
}

func TestAnUnarmedBudgetGrantsNothing(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)

	if budget.Reserve(time.Millisecond) {
		t.Error("Reserve() granted against a budget nobody sized")
	}
}

func TestSettleGivesBackWhatTheAttemptDidNotSpend(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)
	budget.Arm(2 * time.Second) // balance: 1s

	if !budget.Reserve(400 * time.Millisecond) {
		t.Fatal("Reserve() refused a claim the balance covers")
	}
	budget.Settle(400*time.Millisecond, 250*time.Millisecond)

	// The claim was 400ms and the attempt spent 250ms, so the balance is down by
	// what was spent and not by what was claimed.
	if got := budget.Remaining(); got != 750*time.Millisecond {
		t.Errorf("Remaining() = %v, want 750ms: only the 250ms actually spent leave the budget", got)
	}
}

func TestSettleChargesAnAttemptThatRanLong(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)
	budget.Arm(2 * time.Second) // balance: 1s

	if !budget.Reserve(400 * time.Millisecond) {
		t.Fatal("Reserve() refused a claim the balance covers")
	}
	budget.Settle(400*time.Millisecond, 700*time.Millisecond)

	if got := budget.Remaining(); got != 300*time.Millisecond {
		t.Errorf("Remaining() = %v, want 300ms: the attempt overran its claim by 300ms", got)
	}
}

func TestSettleOnAnExactClaimChangesNothing(t *testing.T) {
	ctx := retry.WithBudget(context.Background())
	budget, _ := retry.BudgetFrom(ctx)
	budget.Arm(2 * time.Second)

	if !budget.Reserve(400 * time.Millisecond) {
		t.Fatal("Reserve() refused a claim the balance covers")
	}
	budget.Settle(400*time.Millisecond, 400*time.Millisecond)

	if got := budget.Remaining(); got != 600*time.Millisecond {
		t.Errorf("Remaining() = %v, want 600ms untouched by an exact settlement", got)
	}
}
