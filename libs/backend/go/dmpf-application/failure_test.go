package dmpfapplication_test

import (
	"errors"
	"testing"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
)

func TestNewFailureCarriesTheClassificationVerbatim(t *testing.T) {
	cause := errors.New("failure_test: dependency unavailable")

	f := dmpfapplication.NewFailure(dmpfapplication.TransientDependency, true, cause)

	if got := f.Category(); got != dmpfapplication.TransientDependency {
		t.Fatalf("Category() = %v, want %v", got, dmpfapplication.TransientDependency)
	}
	if !f.Retryable() {
		t.Fatal("Retryable() = false, want true")
	}
	if !errors.Is(f, cause) {
		t.Fatal("Unwrap() did not expose cause to errors.Is")
	}
}

func TestFailureErrorFormatsCategoryAndCause(t *testing.T) {
	cause := errors.New("failure_test: boom")
	f := dmpfapplication.NewFailure(dmpfapplication.Conflict, false, cause)

	if got, want := f.Error(), "Conflict: failure_test: boom"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestFailureErrorWithoutCauseIsJustTheCategory(t *testing.T) {
	f := dmpfapplication.NewFailure(dmpfapplication.Unexpected, false, nil)

	if got, want := f.Error(), "Unexpected"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if f.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil", f.Unwrap())
	}
}
