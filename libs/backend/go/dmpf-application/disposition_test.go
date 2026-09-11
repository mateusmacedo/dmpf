package dmpfapplication_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// DomainRejection is absent from this table on purpose: D2 comes from the
// UPR's own Rejected outcome (FND-03 §3.3), and never reaches Classify.
func TestClassifyMatchesTheFND07CategoryMatrix(t *testing.T) {
	tests := []struct {
		name      string
		category  dmpfapplication.Category
		retryable bool
		want      dmpfapplication.Disposition
	}{
		{"Validation", dmpfapplication.Validation, false, dmpfapplication.R1D4},
		{"NotFound by default", dmpfapplication.NotFound, false, dmpfapplication.R1D4},
		{"Conflict retryable", dmpfapplication.Conflict, true, dmpfapplication.R1D3},
		{"Conflict not retryable", dmpfapplication.Conflict, false, dmpfapplication.R1D4},
		{"Forbidden", dmpfapplication.Forbidden, false, dmpfapplication.R1D4},
		{"Unauthenticated", dmpfapplication.Unauthenticated, false, dmpfapplication.R1D4},
		{"TransientDependency", dmpfapplication.TransientDependency, true, dmpfapplication.R1D3},
		{"RateLimited", dmpfapplication.RateLimited, true, dmpfapplication.R1D3},
		{"DeadlineExceeded predicate satisfied", dmpfapplication.DeadlineExceeded, true, dmpfapplication.R1D3},
		{"DeadlineExceeded predicate not satisfied", dmpfapplication.DeadlineExceeded, false, dmpfapplication.R1D4},
		{"Cancelled", dmpfapplication.Cancelled, false, dmpfapplication.R1D4},
		{"Unexpected by default", dmpfapplication.Unexpected, false, dmpfapplication.R1D4},
		{"Unexpected with a declared predicate", dmpfapplication.Unexpected, true, dmpfapplication.R1D3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := dmpfapplication.NewFailure(tt.category, tt.retryable, nil)

			if got := dmpfapplication.Classify(failure); got != tt.want {
				t.Fatalf("Classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyRecognisesAWrappedFailure(t *testing.T) {
	failure := dmpfapplication.NewFailure(dmpfapplication.TransientDependency, true, nil)
	wrapped := fmt.Errorf("disposition_test: wrapped: %w", failure)

	if got := dmpfapplication.Classify(wrapped); got != dmpfapplication.R1D3 {
		t.Fatalf("Classify() = %v, want %v", got, dmpfapplication.R1D3)
	}
}

func TestClassifyMapsErrRegisterTimeoutToR1D3(t *testing.T) {
	if got := dmpfapplication.Classify(dmpfports.ErrRegisterTimeout); got != dmpfapplication.R1D3 {
		t.Fatalf("Classify() = %v, want %v (INB-17)", got, dmpfapplication.R1D3)
	}
}

func TestClassifyMapsContextDeadlineAndCancelToR1D4(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, context.Canceled} {
		if got := dmpfapplication.Classify(err); got != dmpfapplication.R1D4 {
			t.Fatalf("Classify(%v) = %v, want %v", err, got, dmpfapplication.R1D4)
		}
	}
}

func TestClassifyMapsAnUnclassifiedErrorToR1D4(t *testing.T) {
	err := errors.New("disposition_test: unrecognised failure")

	if got := dmpfapplication.Classify(err); got != dmpfapplication.R1D4 {
		t.Fatalf("Classify() = %v, want %v (ERR-11)", got, dmpfapplication.R1D4)
	}
}

func TestClassifyPanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Classify(nil) must panic: success and rejection never reach it")
		}
	}()

	_ = dmpfapplication.Classify(nil)
}

func TestDispositionString(t *testing.T) {
	tests := map[dmpfapplication.Disposition]string{
		dmpfapplication.R1D1: "R1×D1",
		dmpfapplication.R1D2: "R1×D2",
		dmpfapplication.R1D3: "R1×D3",
		dmpfapplication.R1D4: "R1×D4",
		dmpfapplication.R2:   "R2",
		dmpfapplication.R3:   "R3",
		dmpfapplication.R4:   "R4",
	}
	for d, want := range tests {
		if got := d.String(); got != want {
			t.Fatalf("%d.String() = %q, want %q", d, got, want)
		}
	}
}
