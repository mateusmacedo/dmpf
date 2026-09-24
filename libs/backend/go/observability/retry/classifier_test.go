package retry_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
)

func TestRetryabilityNamesEveryVerdict(t *testing.T) {
	want := map[retry.Retryability]string{
		retry.Retryable:    "retryable",
		retry.NotRetryable: "not_retryable",
		retry.Unknown:      "unknown",
	}

	for verdict, label := range want {
		if got := verdict.String(); got != label {
			t.Errorf("Retryability(%d).String() = %q, want %q", verdict, got, label)
		}
	}
}

func TestTheZeroRetryabilityIsUnknown(t *testing.T) {
	var zero retry.Retryability

	if zero != retry.Unknown {
		t.Fatalf("the zero Retryability = %v, want Unknown: an unclassified error must deny the retry (RES-29)", zero)
	}
}

func TestFactorNamesEveryDenial(t *testing.T) {
	want := map[retry.Factor]string{
		retry.FactorNone:       "none",
		retry.FactorAttempts:   "attempts",
		retry.FactorRetryable:  "retryable",
		retry.FactorIdempotent: "idempotent",
		retry.FactorBudget:     "budget",
		retry.FactorDeadline:   "deadline",
	}

	for factor, label := range want {
		if got := factor.String(); got != label {
			t.Errorf("Factor(%d).String() = %q, want %q", factor, got, label)
		}
	}
}

func TestTheZeroFactorIsNone(t *testing.T) {
	var zero retry.Factor

	if zero != retry.FactorNone {
		t.Fatalf("the zero Factor = %v, want FactorNone: an allowed verdict names no denial", zero)
	}
}
