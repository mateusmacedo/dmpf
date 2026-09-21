package usecase_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

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
