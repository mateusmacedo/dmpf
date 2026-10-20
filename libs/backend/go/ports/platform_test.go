package ports_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestPlatformReachZeroValueIsUndeclared(t *testing.T) {
	var reach ports.PlatformReach
	if reach != ports.ReachUndeclared {
		t.Fatalf("zero PlatformReach = %d; want ReachUndeclared, so an unset reach never reads as all tenants", reach)
	}
	if ports.ReachAllTenants == ports.ReachUndeclared {
		t.Fatal("ReachAllTenants equals ReachUndeclared")
	}
}

func TestPlatformReachCarrierRoundTripsTheDeclaration(t *testing.T) {
	ctx := ports.WithPlatformReach(context.Background(), ports.ReachAllTenants)

	reach, ok := ports.PlatformReachFrom(ctx)
	if !ok || reach != ports.ReachAllTenants {
		t.Fatalf("PlatformReachFrom = %d, %v; want ReachAllTenants, true", reach, ok)
	}
}

func TestPlatformReachCarrierReportsAbsenceAsUndeclared(t *testing.T) {
	reach, ok := ports.PlatformReachFrom(context.Background())
	if ok || reach != ports.ReachUndeclared {
		t.Fatalf("PlatformReachFrom on an empty carrier = %d, %v; want ReachUndeclared, false", reach, ok)
	}
}

func TestPlatformReachIsNotCarriedByTheExecutionContext(t *testing.T) {
	ctx := ports.WithPlatformReach(context.Background(), ports.ReachAllTenants)
	if _, ok := ports.ExecutionContextFrom(ctx); ok {
		t.Fatal("depositing the reach produced an execution context; the reach must travel apart from the nine CTX-01 fields")
	}
}

func TestPlatformSentinelsAreDistinctAndSurviveWrapping(t *testing.T) {
	sentinels := map[string]error{
		"ErrPlatformReachUndeclared":     ports.ErrPlatformReachUndeclared,
		"ErrPlatformLimitInvalid":        ports.ErrPlatformLimitInvalid,
		"ErrAuditUnavailable":            ports.ErrAuditUnavailable,
		"ErrProjectionDenied":            ports.ErrProjectionDenied,
		"ErrProjectionDestinationAbsent": ports.ErrProjectionDestinationAbsent,
	}
	for name, sentinel := range sentinels {
		wrapped := fmt.Errorf("postgres: platform list orders: %w", sentinel)
		if !errors.Is(wrapped, sentinel) {
			t.Errorf("errors.Is(wrapped, %s) = false", name)
		}
		if errors.Is(wrapped, ports.ErrNotFound) || errors.Is(wrapped, ports.ErrDenied) {
			t.Errorf("%s reads as ErrNotFound or ErrDenied; each failure must keep its own identity", name)
		}
		for other, candidate := range sentinels {
			if other != name && errors.Is(sentinel, candidate) {
				t.Errorf("%s reads as %s", name, other)
			}
		}
	}
}
