//go:build integration && distributed

package distkit_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// withExecution deposits on the carrier what the edge would have deposited, so
// a test exercises the same path production does (CTX-03, ADR-049).
func withExecution(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	return ports.WithExecutionContext(ctx, testExecution(t))
}

// testExecution is what the edge would have mounted: every mandatory field of
// CTX-01 present, plus a subject and a tenant so a decision that reads them has
// something to read.
func testExecution(t *testing.T) ports.ExecutionContext {
	t.Helper()
	subject, tenant := ports.SubjectID("s-test"), ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "r-test",
		CorrelationID: "c-test",
		TraceContext:  "t-test",
		Subject:       &subject,
		Tenant:        &tenant,
		Permissions:   []ports.Permission{},
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return execution
}
