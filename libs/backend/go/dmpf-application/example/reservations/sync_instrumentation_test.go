package reservationsapp_test

import (
	"context"
	"testing"

	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/reservations"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

type syncInstrumentation struct {
	begins  []string
	results []dmpfports.Result
	audits  []dmpfports.AuditEvent
}

func (r *syncInstrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, dmpfports.EndOperation) {
	r.begins = append(r.begins, operation)
	return ctx, func(result dmpfports.Result) { r.results = append(r.results, result) }
}

func (r *syncInstrumentation) Audit(_ context.Context, event dmpfports.AuditEvent) {
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

	if _, err := h.service.Reserve(context.Background(), reservationsapp.Reserve{Order: syncOrder, Items: 1}); err != nil {
		t.Fatalf("Reserve() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != reservationsapp.OperationReserve {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, reservationsapp.OperationReserve)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != dmpfports.OutcomeAccepted {
		t.Fatalf("results = %+v, want exactly one accepted", instr.results)
	}
	want := dmpfports.AuditEvent{Object: string(syncOrder), Action: reservationsapp.OperationReserve, Outcome: dmpfports.OutcomeAccepted, At: syncOccurred}
	if len(instr.audits) != 1 || instr.audits[0] != want {
		t.Fatalf("audits = %+v, want exactly %+v", instr.audits, want)
	}
}

func TestCancelRejectedReportsRejectedAndAudits(t *testing.T) {
	h, instr := newInstrumentedSyncHarness(t)
	h.seed(t, confirmedSnapshot(1), 0)

	if _, err := h.service.Cancel(context.Background(), reservationsapp.Cancel{Order: syncOrder}); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != reservationsapp.OperationCancel {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, reservationsapp.OperationCancel)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != dmpfports.OutcomeRejected || instr.results[0].Err != nil {
		t.Fatalf("results = %+v, want exactly one rejected without error", instr.results)
	}
	if len(instr.audits) != 1 || instr.audits[0].Outcome != dmpfports.OutcomeRejected {
		t.Fatalf("audits = %+v, want exactly one rejected event", instr.audits)
	}
}

func TestFindReservationReportsAcceptedWithoutAudit(t *testing.T) {
	h, instr := newInstrumentedSyncHarness(t)
	h.seed(t, confirmedSnapshot(1), 0)

	if _, err := h.service.FindReservation(context.Background(), syncOrder); err != nil {
		t.Fatalf("FindReservation() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != reservationsapp.OperationFindReservation {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, reservationsapp.OperationFindReservation)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none (LOG-14)", instr.audits)
	}
}
