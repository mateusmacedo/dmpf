package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func reaching(ctx context.Context) context.Context {
	return ports.WithPlatformReach(ctx, ports.ReachAllTenants)
}

func TestPlatformPermittedAuthorizesAnOperatorHoldingThePermission(t *testing.T) {
	ctx := reaching(carrying(t, ref(ports.SubjectID("s-1")), nil, []ports.Permission{"orders:write"}))

	if err := application.PlatformPermitted(writeOf)(ctx, command{}); err != nil {
		t.Fatalf("PlatformPermitted() = %v, want nil", err)
	}
}

func TestPlatformPermittedDeniesAnOperatorWithoutThePermission(t *testing.T) {
	ctx := reaching(carrying(t, ref(ports.SubjectID("s-1")), nil, []ports.Permission{"orders:read"}))

	err := application.PlatformPermitted(writeOf)(ctx, command{})
	if !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("PlatformPermitted() = %v, want ErrDenied", err)
	}
	if errors.Is(err, ports.ErrPlatformReachUndeclared) {
		t.Fatalf("PlatformPermitted() = %v; the reach was declared, the permission is what is missing", err)
	}
}

func TestPlatformPermittedAuthorizesAChainWithoutSubjectUnderTheDeclaredReach(t *testing.T) {
	ctx := reaching(carrying(t, nil, nil, nil))

	if err := application.PlatformPermitted(writeOf)(ctx, command{}); err != nil {
		t.Fatalf("PlatformPermitted() = %v, want nil", err)
	}
}

func TestPlatformPermittedIgnoresATenantOnTheCarrier(t *testing.T) {
	ctx := reaching(carrying(t, ref(ports.SubjectID("s-1")), ref(ports.TenantID("acme")), []ports.Permission{"orders:write"}))

	if err := application.PlatformPermitted(writeOf)(ctx, command{}); err != nil {
		t.Fatalf("PlatformPermitted() = %v, want nil: a tenant on the carrier neither grants nor blocks the platform reach", err)
	}
}

func TestPlatformPermittedDeniesWithoutTheDeclaredReach(t *testing.T) {
	scoped := carrying(t, ref(ports.SubjectID("s-1")), nil, []ports.Permission{"orders:write"})
	cases := map[string]context.Context{
		"carrier without reach":    scoped,
		"reach declared undefined": ports.WithPlatformReach(scoped, ports.ReachUndeclared),
		"no execution context":     context.Background(),
		"chain without subject":    carrying(t, nil, nil, nil),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			err := application.PlatformPermitted(writeOf)(ctx, command{})
			if !errors.Is(err, ports.ErrPlatformReachUndeclared) || !errors.Is(err, ports.ErrDenied) {
				t.Fatalf("PlatformPermitted() = %v, want ErrPlatformReachUndeclared reading as ErrDenied", err)
			}
		})
	}
}

func TestPlatformPermittedDeniesAnOperationThatDeclaresNoPermission(t *testing.T) {
	undeclared := func(command) ports.Permission { return "" }
	for name, ctx := range map[string]context.Context{
		"operator": reaching(carrying(t, ref(ports.SubjectID("s-1")), nil, []ports.Permission{"orders:write"})),
		"chain":    reaching(carrying(t, nil, nil, nil)),
	} {
		t.Run(name, func(t *testing.T) {
			if err := application.PlatformPermitted(undeclared)(ctx, command{}); !errors.Is(err, ports.ErrDenied) {
				t.Fatalf("PlatformPermitted() = %v, want ErrDenied", err)
			}
		})
	}
}

func TestPlatformPermittedRefusesADeclaredReachWithoutAContext(t *testing.T) {
	ctx := reaching(context.Background())

	if err := application.PlatformPermitted(writeOf)(ctx, command{}); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("PlatformPermitted() = %v, want ErrContextAbsent", err)
	}
}
