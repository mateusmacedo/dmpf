package ports_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestAlreadyExistsIsDistinctFromAVersionConflict(t *testing.T) {
	wrapped := fmt.Errorf("application: reserve booking b-1: %w", ports.ErrAlreadyExists)

	if !errors.Is(wrapped, ports.ErrAlreadyExists) {
		t.Fatalf("errors.Is(%v, ErrAlreadyExists) = false", wrapped)
	}
	if errors.Is(wrapped, ports.ErrVersionConflict) {
		t.Fatal("ErrAlreadyExists reads as ErrVersionConflict; the edge would answer \"replay\" to an identity that will never converge")
	}
}

func TestCrossTenantAccessAnswersAsNotFound(t *testing.T) {
	access := ports.CrossTenantAccess{Object: "orders/o-1", ContextTenant: "globex", DataTenant: "acme"}
	wrapped := fmt.Errorf("application: find order o-1: %w", access)

	if !errors.Is(wrapped, ports.ErrNotFound) {
		t.Fatalf("errors.Is(%v, ErrNotFound) = false; every caller that answers not-found must keep answering it", wrapped)
	}
	if access.Error() != ports.ErrNotFound.Error() {
		t.Fatalf("Error() = %q, want %q: the text reaches responses and logs, and must not name the owner", access.Error(), ports.ErrNotFound.Error())
	}

	var got ports.CrossTenantAccess
	if !errors.As(wrapped, &got) || got != access {
		t.Fatalf("errors.As() = %+v, want %+v: the internal record has to tell the two cases apart", got, access)
	}
}

func TestUniqueViolationReadsAsTheSentinelAndKeepsTheDeclaredRejection(t *testing.T) {
	rejection := domain.Reject("resource-scheduling/resource/duplicate-name", "a resource with this name already exists in the tenant")
	violation := ports.UniqueViolation{Table: "resources", Constraint: "resources_tenant_id_name_key", Rejection: rejection}
	wrapped := fmt.Errorf("postgres: save resources r-1: %w", violation)

	if !errors.Is(wrapped, ports.ErrUniqueViolation) {
		t.Fatalf("errors.Is(%v, ErrUniqueViolation) = false", wrapped)
	}
	if errors.Is(wrapped, ports.ErrAlreadyExists) || errors.Is(wrapped, ports.ErrVersionConflict) {
		t.Fatal("UniqueViolation reads as an identity conflict; the declared rejection would never reach Decide")
	}
	if violation.Error() != ports.ErrUniqueViolation.Error() {
		t.Fatalf("Error() = %q, want %q: the text reaches responses and logs", violation.Error(), ports.ErrUniqueViolation.Error())
	}

	got, ok := errors.AsType[ports.UniqueViolation](wrapped)
	if !ok || got.Rejection != rejection || got.Table != violation.Table || got.Constraint != violation.Constraint {
		t.Fatalf("errors.AsType() = %+v, %v, want %+v", got, ok, violation)
	}
}

func TestUniqueViolationWithoutRejectionIsStillRecognised(t *testing.T) {
	wrapped := fmt.Errorf("postgres: save resources r-1: %w", ports.UniqueViolation{Table: "resources", Constraint: "resources_tenant_id_code_key"})

	got, ok := errors.AsType[ports.UniqueViolation](wrapped)
	if !ok || got.Rejection != nil {
		t.Fatalf("errors.AsType() = %+v, %v, want a violation with a nil Rejection", got, ok)
	}
}
