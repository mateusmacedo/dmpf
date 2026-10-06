//go:build integration

package appkit_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var keys atomic.Int64

// withExecution deposits on the carrier what the edge would have deposited, so
// a test exercises the same path production does (CTX-03, ADR-049).
func withExecution(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	return withKey(t, ctx, fmt.Sprintf("k-%d", keys.Add(1)))
}

func withKey(t *testing.T, ctx context.Context, key string) context.Context {
	t.Helper()
	return ports.WithIdempotencyKey(ports.WithExecutionContext(ctx, testExecution(t)), key)
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
