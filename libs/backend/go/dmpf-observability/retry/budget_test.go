package retry_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
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
