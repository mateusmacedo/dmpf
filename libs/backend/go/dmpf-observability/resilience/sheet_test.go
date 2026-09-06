package resilience_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
)

func TestTheDefaultSheetIsValid(t *testing.T) {
	if err := resilience.Defaults("payments").Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestTheDefaultsAreTheBaselineOfFND08(t *testing.T) {
	sheet := resilience.Defaults("payments")

	breaker, ok := sheet.Breaker.Get()
	if !ok {
		t.Fatal("the default sheet declares no breaker")
	}
	if breaker.Window != 30*time.Second || breaker.Threshold != 0.5 ||
		breaker.MinSamples != 20 || breaker.Cooldown != 30*time.Second || breaker.Probes != 1 {
		t.Errorf("breaker = %+v, want window 30s, threshold 0.5, 20 samples, cooldown 30s, 1 probe (RES-10)", breaker)
	}

	bulkhead, _ := sheet.Bulkhead.Get()
	if bulkhead.Queue != bulkhead.Pool {
		t.Errorf("queue = %d, pool = %d; the queue is the size of the pool (RES-13)", bulkhead.Queue, bulkhead.Pool)
	}
	if bulkhead.Acquisition != 100*time.Millisecond {
		t.Errorf("acquisition = %v, want 100ms (RES-13)", bulkhead.Acquisition)
	}

	attempts, _ := sheet.MaxAttempts.Get()
	if attempts != 3 {
		t.Errorf("max attempts = %d, want 3 for a synchronous path (RES-33)", attempts)
	}

	backoff, _ := sheet.Backoff.Get()
	if backoff.Base != retry.DefaultBase || backoff.Factor != retry.DefaultFactor || backoff.Cap != retry.DefaultCap {
		t.Errorf("backoff = %+v, want the platform values of RES-32", backoff)
	}

	mode, _ := sheet.Degradation.Get()
	if mode != resilience.Fail {
		t.Errorf("degradation = %q, want %q by default", mode, resilience.Fail)
	}
}

func TestTheEmptyPositionsAreDeclaredNotApplicableWithAReason(t *testing.T) {
	sheet := resilience.Defaults("payments")

	rateLimit, skipped := sheet.RateLimit.Skipped()
	if !skipped {
		t.Fatal("rate limit is not declared not applicable: an empty position needs a reason (RES-21)")
	}
	if strings.TrimSpace(rateLimit.Reason) == "" {
		t.Error("rate limit is not applicable with an empty reason")
	}

	cache, skipped := sheet.Cache.Skipped()
	if !skipped || strings.TrimSpace(cache.Reason) == "" {
		t.Error("cache is not declared not applicable with a reason")
	}
}

func TestValidateReportsEveryBlankAtOnce(t *testing.T) {
	sheet := resilience.Sheet{Dependency: "payments"}

	err := sheet.Validate()

	if !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Validate() = %v, want ErrBlankField", err)
	}
	for _, field := range []string{
		resilience.FieldDeadline, resilience.FieldRetry, resilience.FieldBudget,
		resilience.FieldBackoff, resilience.FieldMaxAttempts, resilience.FieldBreaker,
		resilience.FieldBulkhead, resilience.FieldRateLimit, resilience.FieldCache,
		resilience.FieldDegradation,
	} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("Validate() = %q, want it to name the blank field %q", err, field)
		}
	}
}

func TestValidateRefusesASheetWithoutADependency(t *testing.T) {
	sheet := resilience.Defaults("")

	if err := sheet.Validate(); !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Validate() = %v, want ErrBlankField", err)
	}
}

func TestValidateRefusesANonPositiveDeadline(t *testing.T) {
	sheet := resilience.Defaults("payments")
	sheet.Deadline = resilience.Declare(time.Duration(0))

	if err := sheet.Validate(); !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Validate() = %v, want a refusal: a call without a timeout does not exist (RES-05)", err)
	}
}

func TestValidateRefusesTheDeferMode(t *testing.T) {
	sheet := resilience.Defaults("payments")
	sheet.Degradation = resilience.Declare(resilience.Defer)

	if err := sheet.Validate(); !errors.Is(err, resilience.ErrDeferIsOutbox) {
		t.Fatalf("Validate() = %v, want ErrDeferIsOutbox", err)
	}
}

func TestASkippedFieldSatisfiesTheSheet(t *testing.T) {
	sheet := resilience.Defaults("payments")
	sheet.Breaker = resilience.Skip[resilience.BreakerPolicy]("dependência sem histórico de falha em rajada")

	if err := sheet.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: a declared absence is a decision", err)
	}
}

func TestAFieldSaysEitherAValueOrAnAbsenceButNotNothing(t *testing.T) {
	var blank resilience.Field[int]
	if blank.Declared() {
		t.Error("the zero field reports as declared, want not declared")
	}

	value := resilience.Declare(7)
	if got, ok := value.Get(); !ok || got != 7 {
		t.Errorf("Get() = %v, %v; want 7, true", got, ok)
	}
	if _, skipped := value.Skipped(); skipped {
		t.Error("a declared value also reports a declared absence")
	}

	skipped := resilience.Skip[int]("não se aplica aqui")
	if _, ok := skipped.Get(); ok {
		t.Error("a declared absence also reports a value")
	}
	if absence, isSkipped := skipped.Skipped(); !isSkipped || absence.Reason != "não se aplica aqui" {
		t.Errorf("Skipped() = %+v, %v; want the reason", absence, isSkipped)
	}
}

func TestEffectiveNamesEveryFieldAndExplainsTheEmptyOnes(t *testing.T) {
	effective := resilience.Defaults("payments").Effective()

	if len(effective) != 10 {
		t.Fatalf("Effective() has %d fields, want the ten of RES-21", len(effective))
	}
	for field, value := range effective {
		if value == "" {
			t.Errorf("field %q reports an empty effective value", field)
		}
	}
	if got := effective[resilience.FieldRateLimit]; !strings.HasPrefix(got, resilience.NotApplicablePrefix) {
		t.Errorf("rate limit = %q, want it to declare the absence", got)
	}
	if got := effective[resilience.FieldDeadline]; strings.HasPrefix(got, resilience.NotApplicablePrefix) {
		t.Errorf("deadline = %q, want a value", got)
	}
}

func TestAnOverrideRecordsWhatWhyAndSince(t *testing.T) {
	since := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	sheet := resilience.Defaults("payments")
	sheet.MaxAttempts = resilience.Declare(5)
	sheet.Overrides = []resilience.Override{{
		Field:  resilience.FieldMaxAttempts,
		Value:  "5",
		Reason: "caminho assíncrono, conforme RES-33",
		Since:  since,
	}}

	if err := sheet.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if got := sheet.Overrides[0]; got.Field == "" || got.Reason == "" || got.Since.IsZero() {
		t.Fatalf("Override = %+v, want field, reason and date declared (RES-40)", got)
	}
}

func TestTheBackoffPolicyConvertsToTheEvaluatorsBackoff(t *testing.T) {
	policy := resilience.BackoffPolicy{Base: 100 * time.Millisecond, Factor: 2, Cap: 5 * time.Second}

	got := policy.ForRetry()

	if got.Base != policy.Base || got.Factor != policy.Factor || got.Cap != policy.Cap {
		t.Fatalf("ForRetry() = %+v, want %+v", got, policy)
	}
}

func TestOperationValidateRefusesWhatTheDecoratorsCannotDecide(t *testing.T) {
	valid := resilience.Operation{
		Dependency:        "payments",
		Method:            "Authorize",
		Kind:              resilience.Remote,
		Deadline:          2 * time.Second,
		EstimatedDuration: 200 * time.Millisecond,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	cases := map[string]func(*resilience.Operation){
		"no dependency": func(o *resilience.Operation) { o.Dependency = "" },
		"no method":     func(o *resilience.Operation) { o.Method = "" },
		"no kind":       func(o *resilience.Operation) { o.Kind = "" },
		"no deadline":   func(o *resilience.Operation) { o.Deadline = 0 },
		"no estimate":   func(o *resilience.Operation) { o.EstimatedDuration = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			operation := valid
			mutate(&operation)

			if err := operation.Validate(); err == nil {
				t.Fatalf("Validate() = nil with %s, want a refusal", name)
			}
		})
	}
}

func TestForRetryCarriesWhatTheEvaluatorNeeds(t *testing.T) {
	absent := func(error) bool { return true }
	operation := resilience.Operation{
		Dependency:        "payments",
		Method:            "Authorize",
		Kind:              resilience.Remote,
		Idempotent:        true,
		EffectAbsent:      absent,
		Deadline:          2 * time.Second,
		EstimatedDuration: 200 * time.Millisecond,
	}

	got := operation.ForRetry()

	if got.Dependency != "payments" || got.Method != "Authorize" || !got.Idempotent {
		t.Errorf("ForRetry() = %+v, want the identity and the idempotence carried over", got)
	}
	if got.EstimatedDuration != 200*time.Millisecond {
		t.Errorf("EstimatedDuration = %v, want 200ms", got.EstimatedDuration)
	}
	if got.EffectAbsent == nil || !got.EffectAbsent(errors.New("x")) {
		t.Error("EffectAbsent did not survive the conversion")
	}
}

func TestEveryDecoratorFailureCarriesACategoryAndACode(t *testing.T) {
	failures := []*resilience.Error{
		resilience.ErrBreakerOpen,
		resilience.ErrBulkheadSaturated,
		resilience.ErrDeferIsOutbox,
		resilience.ErrWrapsUnitOfWork,
		resilience.ErrOrderReasonRequired,
		resilience.ErrDeadlineComposition,
		resilience.ErrBlankField,
	}

	for _, failure := range failures {
		if failure.ErrorCategory() == "" {
			t.Errorf("%v declares no category: a span could not record it without the message (TRC-12)", failure)
		}
		if failure.ErrorCode() == "" {
			t.Errorf("%v declares no code", failure)
		}
		if failure.Error() == "" {
			t.Errorf("a failure with an empty message: %#v", failure)
		}
	}
}

func TestADecoratorFailureIsFoundThroughAWrap(t *testing.T) {
	wrapped := errors.Join(errors.New("payments: authorize"), resilience.ErrBreakerOpen)

	if !errors.Is(wrapped, resilience.ErrBreakerOpen) {
		t.Fatal("errors.Is did not find ErrBreakerOpen through the wrap")
	}
	if errors.Is(wrapped, resilience.ErrBulkheadSaturated) {
		t.Fatal("errors.Is confused two distinct failures")
	}
}
