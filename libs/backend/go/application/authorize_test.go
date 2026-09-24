package application_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type command struct{ Order string }

func TestAllowAllAuthorizesEveryOperation(t *testing.T) {
	authorize := application.AllowAll[command]()

	if err := authorize(context.Background(), command{Order: "P-100"}); err != nil {
		t.Fatalf("AllowAll() = %v, want nil", err)
	}
}

func TestTheContextReachesTheDecisionIntact(t *testing.T) {
	subject, tenant := ports.SubjectID("s-1"), ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "r-1",
		CorrelationID: "c-1",
		TraceContext:  "t-1",
		Subject:       &subject,
		Tenant:        &tenant,
		Permissions:   []ports.Permission{"orders:write"},
		Deadline:      ports.Instant(1),
		Locale:        "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}

	var seen ports.ExecutionContext
	var authorize application.Authorize[command] = func(ctx context.Context, _ command) error {
		received, err := ports.RequireExecutionContext(ctx)
		if err != nil {
			return err
		}
		seen = received
		return nil
	}

	ctx := ports.WithExecutionContext(context.Background(), execution)
	if err := authorize(ctx, command{Order: "P-100"}); err != nil {
		t.Fatalf("authorize() = %v, want nil", err)
	}

	if got, identified := seen.Subject(); !identified || got != subject {
		t.Fatalf("Subject() = %q, %t, want %q, true (CTX-03)", got, identified, subject)
	}
	if got, scoped := seen.Tenant(); !scoped || got != tenant {
		t.Fatalf("Tenant() = %q, %t, want %q, true", got, scoped, tenant)
	}
	if got := seen.Permissions(); !slices.Equal(got, []ports.Permission{"orders:write"}) {
		t.Fatalf("Permissions() = %v, want the set resolved before the transaction (IDN-10)", got)
	}
}
