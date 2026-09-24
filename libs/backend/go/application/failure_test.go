package application_test

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

func TestNewFailureCarriesTheClassificationVerbatim(t *testing.T) {
	cause := errors.New("failure_test: dependency unavailable")

	f := application.NewFailure(application.TransientDependency, true, cause)

	if got := f.Category(); got != application.TransientDependency {
		t.Fatalf("Category() = %v, want %v", got, application.TransientDependency)
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
	f := application.NewFailure(application.Conflict, false, cause)

	if got, want := f.Error(), "Conflict: failure_test: boom"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestFailureErrorWithoutCauseIsJustTheCategory(t *testing.T) {
	f := application.NewFailure(application.Unexpected, false, nil)

	if got, want := f.Error(), "Unexpected"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if f.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil", f.Unwrap())
	}
}
