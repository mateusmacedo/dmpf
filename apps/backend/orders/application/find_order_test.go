package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestFindOrderReadsOutsideTheUnitOfWork(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(2), 0)

	snapshot, err := h.service.FindOrder(withExecution(t, context.Background()), orderID)

	if err != nil {
		t.Fatalf("FindOrder() error = %v, want nil", err)
	}
	if len(snapshot.Items) != 2 {
		t.Fatalf("snapshot has %d items, want 2", len(snapshot.Items))
	}
	if got := h.serviceWithinCalls(); got != 0 {
		t.Fatalf("transactions opened = %d, want 0 — a query never opens one (UOW-11)", got)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty — a query never writes the outbox (UOW-11)", got)
	}
}

func TestFindOrderReportsErrNotFoundForAnAbsentAggregate(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindOrder(withExecution(t, context.Background()), "P-404")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindOrder() error = %v, want ErrNotFound through the wrapping", err)
	}
}

func TestFindOrderOfAnotherTenantAnswersNotFoundAndHandsTheAccessToTheInstrumentation(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.seed(t, openSnapshot(1), 0)

	_, err := h.service.FindOrder(asTenant(t, "globex"), orderID)

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindOrder() error = %v, want ErrNotFound: the caller must not learn the order exists (IDN-13)", err)
	}
	var access ports.CrossTenantAccess
	if result := instr.onlyResult(t); result.Outcome != ports.OutcomeFailed || !errors.As(result.Err, &access) {
		t.Fatalf("EndOperation(%+v), want a failure carrying CrossTenantAccess for the security record (IDN-12)", result)
	}
	if access.ContextTenant != "globex" || access.DataTenant != "acme" {
		t.Fatalf("CrossTenantAccess names context %q and data %q, want globex and acme", access.ContextTenant, access.DataTenant)
	}
}

func asTenant(t *testing.T, tenant ports.TenantID) context.Context {
	t.Helper()
	subject := ports.SubjectID("s-test")
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
	return ports.WithExecutionContext(context.Background(), execution)
}
