package execctx_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

func mounted(t *testing.T, ctx context.Context) ports.ExecutionContext {
	t.Helper()
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		t.Fatal("no execution context on the carrier")
	}
	return execution
}

func TestWithExecutionMountsWhatTheEdgeWouldHaveMounted(t *testing.T) {
	execution := mounted(t, execctx.WithExecution(t, context.Background()))

	subject, hasSubject := execution.Subject()
	tenant, hasTenant := execution.Tenant()
	if execution.RequestID() != "r-test" || subject != "s-test" || !hasSubject || tenant != "acme" || !hasTenant {
		t.Fatalf("execution = %s/%s/%s, want r-test/s-test/acme", execution.RequestID(), subject, tenant)
	}
	if execution.Deadline() != 1_755_432_000_000_000_000 || execution.Locale() != "en" {
		t.Fatalf("deadline, locale = %d, %q, want the fixed instant and en", execution.Deadline(), execution.Locale())
	}
}

func TestWithExecutionLeavesTheKeyAndTheSlotOutUnlessAsked(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background())
	ports.MarkIdempotency(ctx, ports.IdempotencyNew)

	if _, ok := ports.IdempotencyKeyFrom(ctx); ok {
		t.Fatal("a key was installed without AutoKey or WithKey")
	}
	if _, ok := ports.IdempotencyOutcomeFrom(ctx); ok {
		t.Fatal("a slot was installed without WithSlot")
	}
}

func TestAutoKeyGivesEachCallItsOwnKey(t *testing.T) {
	first, _ := ports.IdempotencyKeyFrom(execctx.WithExecution(t, context.Background(), execctx.AutoKey()))
	second, _ := ports.IdempotencyKeyFrom(execctx.WithExecution(t, context.Background(), execctx.AutoKey()))

	if !strings.HasPrefix(first, "k-") || first == second {
		t.Fatalf("keys = %q, %q, want two distinct k-<n>", first, second)
	}
}

func TestWithKeyCarriesTheKeyAndWithSlotTheSlot(t *testing.T) {
	ctx := execctx.WithKey(t, context.Background(), "k-fixed", execctx.WithSlot())
	ports.MarkIdempotency(ctx, ports.IdempotencyReplayed)

	if key, _ := ports.IdempotencyKeyFrom(ctx); key != "k-fixed" {
		t.Fatalf("key = %q, want k-fixed", key)
	}
	if outcome, _ := ports.IdempotencyOutcomeFrom(ctx); outcome != ports.IdempotencyReplayed {
		t.Fatalf("outcome = %v, want the one marked on the slot", outcome)
	}
}

func TestTheOptionsReachTheVariantsTheTestsUse(t *testing.T) {
	execution := execctx.Execution(t, execctx.WithoutSubject(), execctx.WithTenant("globex"), execctx.WithDeadline(42))

	if _, hasSubject := execution.Subject(); hasSubject {
		t.Fatal("WithoutSubject left a subject")
	}
	if tenant, _ := execution.Tenant(); tenant != "globex" || execution.Deadline() != 42 {
		t.Fatalf("tenant, deadline = %q, %d, want globex and 42", tenant, execution.Deadline())
	}
}
