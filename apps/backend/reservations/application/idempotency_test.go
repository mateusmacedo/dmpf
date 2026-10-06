package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type auditCounter struct{ audits int }

func (c *auditCounter) BeginOperation(ctx context.Context, _ string) (context.Context, ports.EndOperation) {
	return ctx, func(ports.Result) {}
}

func (c *auditCounter) Audit(context.Context, ports.AuditEvent) { c.audits++ }

func TestARepeatedReserveReplaysWithoutASecondEffectOrAudit(t *testing.T) {
	h := newSyncHarness(t)
	audits := &auditCounter{}
	h.service.Instrumentation = audits
	reserve := application.Reserve{Order: syncOrder, Items: 3}

	first, err := h.service.Reserve(withKey(t, context.Background(), "k-reserve"), reserve)
	if err != nil {
		t.Fatalf("first Reserve() = %v, want nil", err)
	}
	ctx := withKey(t, context.Background(), "k-reserve")
	again, err := h.service.Reserve(ctx, reserve)
	if err != nil {
		t.Fatalf("repeated Reserve() = %v, want the stored outcome", err)
	}

	if again.Response() != first.Response() {
		t.Fatalf("replay = %+v, want %+v", again.Response(), first.Response())
	}
	if h.saves != 1 || h.enqueues() != 1 || audits.audits != 1 {
		t.Fatalf("saves %d, enqueues %d, audits %d; want 1 each: the replay reapplied the effect", h.saves, h.enqueues(), audits.audits)
	}
	if outcome, _ := ports.IdempotencyOutcomeFrom(ctx); outcome != ports.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
}

func TestARepeatedRefusalReplaysTheRejectionWithoutLoading(t *testing.T) {
	h := newSyncHarness(t)
	h.seed(t, canceledSnapshot(), 0)
	reserve := application.Reserve{Order: syncOrder, Items: 3}

	if _, err := h.service.Reserve(withKey(t, context.Background(), "k-refused"), reserve); err != nil {
		t.Fatalf("first Reserve() = %v, want a rejection, not an error", err)
	}
	h.rec.Reset()
	again, err := h.service.Reserve(withKey(t, context.Background(), "k-refused"), reserve)
	if err != nil {
		t.Fatalf("repeated Reserve() = %v, want the stored rejection", err)
	}

	if _, rejected := again.Rejection(); !rejected {
		t.Fatalf("replay = %+v, want the rejection of a canceled reservation", again)
	}
	if slices.Contains(h.rec.Observed(), "domain.Load") {
		t.Fatalf("sequence = %v: the replay reached the aggregate", h.rec.Observed())
	}
}

func TestTheSameKeyWithAnotherPayloadIsAMismatch(t *testing.T) {
	h := newSyncHarness(t)
	if _, err := h.service.Reserve(withKey(t, context.Background(), "k-reused"), application.Reserve{Order: syncOrder, Items: 3}); err != nil {
		t.Fatalf("first Reserve() = %v, want nil", err)
	}

	_, err := h.service.Reserve(withKey(t, context.Background(), "k-reused"), application.Reserve{Order: syncOrder, Items: 4})

	if !errors.Is(err, ports.ErrIdempotencyMismatch) {
		t.Fatalf("Reserve(other items) = %v, want ErrIdempotencyMismatch", err)
	}
	if h.enqueues() != 1 {
		t.Fatalf("enqueues = %d, want 1", h.enqueues())
	}
}

func TestACommandWaitingPastTheCeilingIsInFlight(t *testing.T) {
	h := newSyncHarness(t, withSyncRegisterError(ports.ErrRegisterTimeout))

	_, err := h.service.Cancel(withExecution(t, context.Background()), application.Cancel{Order: syncOrder})

	if !errors.Is(err, ports.ErrIdempotencyInFlight) {
		t.Fatalf("Cancel() = %v, want ErrIdempotencyInFlight", err)
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeAnyEffect(t *testing.T) {
	h := newSyncHarness(t)
	ctx := ports.WithExecutionContext(context.Background(), testExecution(t))

	_, err := h.service.Reserve(ctx, application.Reserve{Order: syncOrder, Items: 3})

	if !errors.Is(err, ports.ErrIdempotencyKeyAbsent) {
		t.Fatalf("Reserve() = %v, want ErrIdempotencyKeyAbsent", err)
	}
	if h.saves != 0 || h.enqueues() != 0 {
		t.Fatalf("saves %d, enqueues %d, want none", h.saves, h.enqueues())
	}
}
