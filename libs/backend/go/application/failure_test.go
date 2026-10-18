package application_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
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

type categorized interface {
	ErrorCategory() string
	ErrorCode() string
}

func TestFailureExposesItsCategoryAndCodeToRedaction(t *testing.T) {
	wrapped := fmt.Errorf("orders: place: %w", application.NewFailure(application.Conflict, false, errors.New("duplicate key")))

	var failure categorized
	if !errors.As(wrapped, &failure) {
		t.Fatal("errors.As(Failure, Categorized) = false, want the shape redact.Error reads (RF-A3)")
	}
	if got := failure.ErrorCategory(); got != string(application.Conflict) {
		t.Errorf("ErrorCategory() = %q, want %q", got, application.Conflict)
	}
	if got := failure.ErrorCode(); got != string(application.Conflict) {
		t.Errorf("ErrorCode() = %q, want %q: the stable code comes from the FND-07 taxonomy (LOG-03)", got, application.Conflict)
	}
}

func TestCategoryOfGivesTheIdempotencySentinelsTheirCategory(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"key absent":     {ports.ErrIdempotencyKeyAbsent, string(application.Validation)},
		"key invalid":    {ports.ErrIdempotencyKeyInvalid, string(application.Validation)},
		"key reused":     {fmt.Errorf("failure_test: %w", ports.ErrIdempotencyMismatch), string(application.Validation)},
		"key in flight":  {ports.ErrIdempotencyInFlight, string(application.Conflict)},
		"already exists": {fmt.Errorf("failure_test: %w", ports.ErrAlreadyExists), string(application.Conflict)},
		"failure":        {application.NewFailure(application.NotFound, false, errors.New("failure_test: gone")), string(application.NotFound)},
		"anything else":  {errors.New("failure_test: boom"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := application.CategoryOf(c.err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

func TestCategoryOfGivesTheRepositorySentinelsTheCategoryTheEdgeAnswers(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"not found":               {fmt.Errorf("failure_test: %w", ports.ErrNotFound), string(application.NotFound)},
		"cross-tenant access":     {fmt.Errorf("failure_test: %w", ports.CrossTenantAccess{Object: "orders/x-1", ContextTenant: "acme", DataTenant: "globex"}), string(application.NotFound)},
		"version conflict":        {fmt.Errorf("failure_test: %w", ports.ErrVersionConflict), string(application.Conflict)},
		"unique violation":        {fmt.Errorf("failure_test: %w", ports.UniqueViolation{Table: "orders", Constraint: "orders_tenant_id_ref_key"}), string(application.Conflict)},
		"failure over a sentinel": {application.NewFailure(application.Unexpected, false, ports.ErrNotFound), string(application.Unexpected)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := application.CategoryOf(c.err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}
