package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
)

type syncInstrumentation struct {
	begins  []string
	results []ports.Result
	audits  []ports.AuditEvent
}

func (r *syncInstrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, ports.EndOperation) {
	r.begins = append(r.begins, operation)
	return ctx, func(result ports.Result) { r.results = append(r.results, result) }
}

func (r *syncInstrumentation) Audit(_ context.Context, event ports.AuditEvent) {
	r.audits = append(r.audits, event)
}

func newInstrumentedSyncHarness(t *testing.T) (*syncHarness, *syncInstrumentation) {
	t.Helper()
	h := newSyncHarness(t)
	instr := &syncInstrumentation{}
	h.service.Instrumentation = instr
	return h, instr
}

func TestReserveAcceptedReportsAcceptedAndAudits(t *testing.T) {
	h, instr := newInstrumentedSyncHarness(t)

	if _, err := h.service.Reserve(context.Background(), application.Reserve{Order: syncOrder, Items: 1}); err != nil {
		t.Fatalf("Reserve() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != application.OperationReserve {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, application.OperationReserve)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != ports.OutcomeAccepted {
		t.Fatalf("results = %+v, want exactly one accepted", instr.results)
	}
	want := ports.AuditEvent{Object: string(syncOrder), Action: application.OperationReserve, Outcome: ports.OutcomeAccepted, At: syncOccurred}
	if len(instr.audits) != 1 || instr.audits[0] != want {
		t.Fatalf("audits = %+v, want exactly %+v", instr.audits, want)
	}
}

func TestCancelRejectedReportsRejectedAndAudits(t *testing.T) {
	h, instr := newInstrumentedSyncHarness(t)
	h.seed(t, confirmedSnapshot(1), 0)

	if _, err := h.service.Cancel(context.Background(), application.Cancel{Order: syncOrder}); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != application.OperationCancel {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, application.OperationCancel)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != ports.OutcomeRejected || instr.results[0].Err != nil {
		t.Fatalf("results = %+v, want exactly one rejected without error", instr.results)
	}
	if len(instr.audits) != 1 || instr.audits[0].Outcome != ports.OutcomeRejected {
		t.Fatalf("audits = %+v, want exactly one rejected event", instr.audits)
	}
}

func TestFindReservationReportsAcceptedWithoutAudit(t *testing.T) {
	h, instr := newInstrumentedSyncHarness(t)
	h.seed(t, confirmedSnapshot(1), 0)

	if _, err := h.service.FindReservation(context.Background(), syncOrder); err != nil {
		t.Fatalf("FindReservation() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != application.OperationFindReservation {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, application.OperationFindReservation)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none (LOG-14)", instr.audits)
	}
}
