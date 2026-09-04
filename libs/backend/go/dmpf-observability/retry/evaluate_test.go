package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
)

var errTransient = errors.New("connection reset by peer")

const estimate = 200 * time.Millisecond

func always(verdict retry.Retryability) retry.Classifier {
	return func(error) retry.Retryability { return verdict }
}

func noJitter() float64 { return 0 }

func armedBudget(t *testing.T, total time.Duration) *retry.Budget {
	t.Helper()
	ctx := retry.WithBudget(context.Background(), retry.WithTotal(total))
	budget, ok := retry.BudgetFrom(ctx)
	if !ok {
		t.Fatal("BudgetFrom() reported no budget right after WithBudget")
	}
	return budget
}

func idempotentRemote() resilience.Operation {
	return resilience.Operation{
		Dependency:        "payments",
		Method:            "Authorize",
		Kind:              resilience.Remote,
		Idempotent:        true,
		Deadline:          2 * time.Second,
		EstimatedDuration: estimate,
	}
}

// allowingInput is the shape where every factor holds, so each case below turns
// exactly one of them false and names the expected denial.
func allowingInput(t *testing.T) retry.Input {
	t.Helper()
	return retry.Input{
		Err:         errTransient,
		Classifier:  always(retry.Retryable),
		Operation:   idempotentRemote(),
		Attempt:     0,
		MaxAttempts: 3,
		Budget:      armedBudget(t, time.Second),
		Remaining:   time.Second,
		Backoff:     retry.DefaultBackoff(),
		Rand:        noJitter,
	}
}

func TestEvaluateAllowsWhenEveryFactorHolds(t *testing.T) {
	got := retry.Evaluate(allowingInput(t))

	if !got.Allowed {
		t.Fatalf("Verdict = %+v, want allowed", got)
	}
	if got.Denied != retry.FactorNone {
		t.Fatalf("Denied = %v, want none on an allowed verdict", got.Denied)
	}
}

func TestEvaluateNamesTheFirstFalseFactor(t *testing.T) {
	cases := []struct {
		name  string
		mutar func(*retry.Input)
		want  retry.Factor
	}{
		{
			name:  "ceiling reached is checked before the four factors",
			mutar: func(in *retry.Input) { in.Attempt = 3 },
			want:  retry.FactorAttempts,
		},
		{
			name:  "classifier says not retryable",
			mutar: func(in *retry.Input) { in.Classifier = always(retry.NotRetryable) },
			want:  retry.FactorRetryable,
		},
		{
			name:  "classifier says unknown",
			mutar: func(in *retry.Input) { in.Classifier = always(retry.Unknown) },
			want:  retry.FactorRetryable,
		},
		{
			name:  "no classifier at all",
			mutar: func(in *retry.Input) { in.Classifier = nil },
			want:  retry.FactorRetryable,
		},
		{
			name: "not idempotent and no effect-absent predicate",
			mutar: func(in *retry.Input) {
				in.Operation.Idempotent = false
				in.Operation.EffectAbsent = nil
			},
			want: retry.FactorIdempotent,
		},
		{
			name: "not idempotent and the predicate denies this error",
			mutar: func(in *retry.Input) {
				in.Operation.Idempotent = false
				in.Operation.EffectAbsent = func(error) bool { return false }
			},
			want: retry.FactorIdempotent,
		},
		{
			name:  "no budget in the execution",
			mutar: func(in *retry.Input) { in.Budget = nil },
			want:  retry.FactorBudget,
		},
		{
			name:  "remaining deadline below the need",
			mutar: func(in *retry.Input) { in.Remaining = estimate - time.Nanosecond },
			want:  retry.FactorDeadline,
		},
	}

	for _, caso := range cases {
		t.Run(caso.name, func(t *testing.T) {
			in := allowingInput(t)
			caso.mutar(&in)

			got := retry.Evaluate(in)

			if got.Allowed {
				t.Fatalf("Verdict = %+v, want denied by %v", got, caso.want)
			}
			if got.Denied != caso.want {
				t.Fatalf("Denied = %v, want %v", got.Denied, caso.want)
			}
		})
	}
}

func TestANotIdempotentMethodIsRepeatableWhenTheAttemptLeftNoEffect(t *testing.T) {
	in := allowingInput(t)
	in.Operation.Idempotent = false
	in.Operation.EffectAbsent = func(err error) bool { return errors.Is(err, errTransient) }

	if got := retry.Evaluate(in); !got.Allowed {
		t.Fatalf("Verdict = %+v, want allowed: the attempt left no effect", got)
	}
}

func TestTheBudgetFactorIsAFrontier(t *testing.T) {
	t.Run("balance equal to the need authorises", func(t *testing.T) {
		in := allowingInput(t)
		in.Budget = armedBudget(t, estimate)

		if got := retry.Evaluate(in); !got.Allowed {
			t.Fatalf("Verdict = %+v, want allowed at balance == need", got)
		}
	})

	t.Run("balance one nanosecond short denies", func(t *testing.T) {
		in := allowingInput(t)
		in.Budget = armedBudget(t, estimate-time.Nanosecond)

		got := retry.Evaluate(in)
		if got.Allowed || got.Denied != retry.FactorBudget {
			t.Fatalf("Verdict = %+v, want denied by budget at need-1ns", got)
		}
	})
}

func TestTheDeadlineFactorIsAFrontier(t *testing.T) {
	t.Run("remaining equal to the need authorises", func(t *testing.T) {
		in := allowingInput(t)
		in.Remaining = estimate

		if got := retry.Evaluate(in); !got.Allowed {
			t.Fatalf("Verdict = %+v, want allowed at remaining == need", got)
		}
	})

	t.Run("remaining one nanosecond short denies", func(t *testing.T) {
		in := allowingInput(t)
		in.Remaining = estimate - time.Nanosecond

		got := retry.Evaluate(in)
		if got.Allowed || got.Denied != retry.FactorDeadline {
			t.Fatalf("Verdict = %+v, want denied by deadline at need-1ns", got)
		}
	})
}

func TestTheNeedIncludesTheBackoffWait(t *testing.T) {
	in := allowingInput(t)
	in.Rand = func() float64 { return 1 }
	in.Remaining = estimate + retry.DefaultBase - time.Nanosecond
	in.Budget = armedBudget(t, time.Hour)

	got := retry.Evaluate(in)

	if got.Allowed || got.Denied != retry.FactorDeadline {
		t.Fatalf("Verdict = %+v, want denied by deadline: the need is the estimate plus the wait", got)
	}
}

func TestAnAllowedVerdictCarriesTheWait(t *testing.T) {
	in := allowingInput(t)
	in.Rand = func() float64 { return 1 }
	in.Remaining = time.Hour
	in.Budget = armedBudget(t, time.Hour)

	got := retry.Evaluate(in)

	if !got.Allowed {
		t.Fatalf("Verdict = %+v, want allowed", got)
	}
	if got.Wait != retry.DefaultBase {
		t.Fatalf("Wait = %v, want %v — the whole interval at draw 1", got.Wait, retry.DefaultBase)
	}
}

// TestEvaluateCompilesWithEveryFieldSet is the literal instantiation the plan
// requires: it fails at build time if a field of Input is renamed or dropped.
func TestEvaluateCompilesWithEveryFieldSet(t *testing.T) {
	verdict := retry.Evaluate(retry.Input{
		Err:        errTransient,
		Classifier: always(retry.Retryable),
		Operation: resilience.Operation{
			Dependency:        "payments",
			Method:            "Authorize",
			Kind:              resilience.Remote,
			Idempotent:        true,
			EffectAbsent:      func(error) bool { return false },
			Deadline:          2 * time.Second,
			EstimatedDuration: estimate,
		},
		Attempt:     1,
		MaxAttempts: 3,
		Budget:      armedBudget(t, time.Minute),
		Remaining:   time.Minute,
		Backoff:     retry.Backoff{Base: retry.DefaultBase, Factor: retry.DefaultFactor, Cap: retry.DefaultCap},
		Rand:        noJitter,
	})

	if !verdict.Allowed {
		t.Fatalf("Verdict = %+v, want allowed", verdict)
	}
}
