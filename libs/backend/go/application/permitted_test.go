package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func writeOf(command) ports.Permission { return "orders:write" }

func carrying(t *testing.T, subject *ports.SubjectID, tenant *ports.TenantID, permissions []ports.Permission) context.Context {
	t.Helper()
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1",
		Subject: subject, Tenant: tenant, Permissions: permissions,
		Deadline: ports.Instant(1), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func ref[T any](v T) *T { return &v }

func TestPermittedAuthorizesASubjectHoldingTheDeclaredPermission(t *testing.T) {
	ctx := carrying(t, ref(ports.SubjectID("s-1")), ref(ports.TenantID("acme")), []ports.Permission{"orders:write"})

	if err := application.Permitted(writeOf)(ctx, command{}); err != nil {
		t.Fatalf("Permitted() = %v, want nil", err)
	}
}

// IDN-08: a subject of the right tenant without the permission is denied.
func TestPermittedDeniesASubjectWithoutThePermission(t *testing.T) {
	ctx := carrying(t, ref(ports.SubjectID("s-1")), ref(ports.TenantID("acme")), []ports.Permission{"orders:read"})

	if err := application.Permitted(writeOf)(ctx, command{}); !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("Permitted() = %v, want ErrDenied", err)
	}
}

// IDN-08, the other half: the permission does not stand in for the tenant.
func TestPermittedDeniesAContextWithoutTenant(t *testing.T) {
	ctx := carrying(t, ref(ports.SubjectID("s-1")), nil, []ports.Permission{"orders:write"})

	if err := application.Permitted(writeOf)(ctx, command{}); !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("Permitted() = %v, want ErrDenied", err)
	}
}

// IDN-18: a chain without a subject is legitimate, and is authorized by the
// workload that runs it, whose identity the channel already verified.
func TestPermittedAuthorizesAChainWithoutSubjectOnItsTenant(t *testing.T) {
	ctx := carrying(t, nil, ref(ports.TenantID("acme")), nil)

	if err := application.Permitted(writeOf)(ctx, command{}); err != nil {
		t.Fatalf("Permitted() = %v, want nil", err)
	}
}

// IDN-17: an operation the mapping does not declare is denied, never allowed.
func TestPermittedDeniesAnOperationThatDeclaresNoPermission(t *testing.T) {
	ctx := carrying(t, ref(ports.SubjectID("s-1")), ref(ports.TenantID("acme")), []ports.Permission{"orders:write"})
	undeclared := func(command) ports.Permission { return "" }

	if err := application.Permitted(undeclared)(ctx, command{}); !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("Permitted() = %v, want ErrDenied", err)
	}
}

func TestPermittedRefusesWithoutAContext(t *testing.T) {
	if err := application.Permitted(writeOf)(context.Background(), command{}); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("Permitted() = %v, want ErrContextAbsent", err)
	}
}
