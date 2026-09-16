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

func TestClassifyMapsContextDeadlineAndCancelToR1D4(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, context.Canceled} {
		if got := application.Classify(err); got != application.R1D4 {
			t.Fatalf("Classify(%v) = %v, want %v", err, got, application.R1D4)
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
