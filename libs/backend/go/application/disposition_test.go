package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// DomainRejection is absent from this table on purpose: D2 comes from the
// UPR's own Rejected outcome (FND-03 §3.3), and never reaches Classify.
func TestClassifyMatchesTheFND07CategoryMatrix(t *testing.T) {
	tests := []struct {
		name      string
		category  application.Category
		retryable bool
		want      application.Disposition
	}{
		{"Validation", application.Validation, false, application.R1D4},
		{"NotFound by default", application.NotFound, false, application.R1D4},
		{"Conflict retryable", application.Conflict, true, application.R1D3},
		{"Conflict not retryable", application.Conflict, false, application.R1D4},
		{"Forbidden", application.Forbidden, false, application.R1D4},
		{"Unauthenticated", application.Unauthenticated, false, application.R1D4},
		{"TransientDependency", application.TransientDependency, true, application.R1D3},
		{"RateLimited", application.RateLimited, true, application.R1D3},
		{"DeadlineExceeded predicate satisfied", application.DeadlineExceeded, true, application.R1D3},
		{"DeadlineExceeded predicate not satisfied", application.DeadlineExceeded, false, application.R1D4},
		{"Cancelled", application.Cancelled, false, application.R1D4},
		{"Unexpected by default", application.Unexpected, false, application.R1D4},
		{"Unexpected with a declared predicate", application.Unexpected, true, application.R1D3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := application.NewFailure(tt.category, tt.retryable, nil)

			if got := application.Classify(failure); got != tt.want {
				t.Fatalf("Classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyRecognisesAWrappedFailure(t *testing.T) {
	failure := application.NewFailure(application.TransientDependency, true, nil)
	wrapped := fmt.Errorf("disposition_test: wrapped: %w", failure)

	if got := application.Classify(wrapped); got != application.R1D3 {
		t.Fatalf("Classify() = %v, want %v", got, application.R1D3)
	}
}

func TestClassifyMapsErrRegisterTimeoutToR1D3(t *testing.T) {
	if got := application.Classify(ports.ErrRegisterTimeout); got != application.R1D3 {
		t.Fatalf("Classify() = %v, want %v (INB-17)", got, application.R1D3)
	}
}

func TestClassifyMapsAnExpiredDeadlineToR1D3(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, fmt.Errorf("disposition_test: handler: %w", context.DeadlineExceeded)} {
		if got := application.Classify(err); got != application.R1D3 {
			t.Fatalf("Classify(%v) = %v, want %v: a slow handler is retried, not quarantined", err, got, application.R1D3)
		}
	}
}

func TestClassifyKeepsACancellationTerminal(t *testing.T) {
	if got := application.Classify(context.Canceled); got != application.R1D4 {
		t.Fatalf("Classify(Canceled) = %v, want %v: Cancelled is not retryable (CTX-23, ERR-11)", got, application.R1D4)
	}
}

type sqlStateError string

func (e sqlStateError) Error() string    { return "disposition_test: sqlstate " + string(e) }
func (e sqlStateError) SQLState() string { return string(e) }

func TestClassifyMapsATransientDatabaseFailureToR1D3(t *testing.T) {
	for _, code := range []string{"08000", "08006", "40001", "40P01", "57P01"} {
		err := fmt.Errorf("disposition_test: save: %w", sqlStateError(code))
		if got := application.Classify(err); got != application.R1D3 {
			t.Errorf("Classify(SQLSTATE %s) = %v, want %v", code, got, application.R1D3)
		}
	}
}

func TestClassifyKeepsAPermanentDatabaseFailureTerminal(t *testing.T) {
	for _, code := range []string{"23505", "42P01", "22001"} {
		if got := application.Classify(sqlStateError(code)); got != application.R1D4 {
			t.Errorf("Classify(SQLSTATE %s) = %v, want %v", code, got, application.R1D4)
		}
	}
}

func TestClassifyMapsAnUnclassifiedErrorToR1D4(t *testing.T) {
	err := errors.New("disposition_test: unrecognised failure")

	if got := application.Classify(err); got != application.R1D4 {
		t.Fatalf("Classify() = %v, want %v (ERR-11)", got, application.R1D4)
	}
}

func TestClassifyPanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Classify(nil) must panic: success and rejection never reach it")
		}
	}()

	_ = application.Classify(nil)
}

func TestDispositionString(t *testing.T) {
	tests := map[application.Disposition]string{
		application.R1D1: "R1×D1",
		application.R1D2: "R1×D2",
		application.R1D3: "R1×D3",
		application.R1D4: "R1×D4",
		application.R2:   "R2",
		application.R3:   "R3",
		application.R4:   "R4",
	}
	for d, want := range tests {
		if got := d.String(); got != want {
			t.Fatalf("%d.String() = %q, want %q", d, got, want)
		}
	}
}

type unsentError bool

func (unsentError) Error() string       { return "disposition_test: statement not sent" }
func (e unsentError) SafeToRetry() bool { return bool(e) }

type timeoutError bool

func (timeoutError) Error() string   { return "disposition_test: i/o timeout" }
func (e timeoutError) Timeout() bool { return bool(e) }

func TestClassifyFollowsTheDriverOnATransientTransportFailure(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want application.Disposition
	}{
		"not sent to the server":    {fmt.Errorf("disposition_test: save: %w", unsentError(true)), application.R1D3},
		"sent to the server":        {unsentError(false), application.R1D4},
		"network timeout":           {fmt.Errorf("disposition_test: dial: %w", timeoutError(true)), application.R1D3},
		"network failure, no timer": {timeoutError(false), application.R1D4},
	} {
		t.Run(name, func(t *testing.T) {
			if got := application.Classify(c.err); got != c.want {
				t.Fatalf("Classify(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
