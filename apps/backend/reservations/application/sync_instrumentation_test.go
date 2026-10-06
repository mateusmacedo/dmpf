package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

type syncInstrumentation struct {
	steps   *serviceskit.Steps
	begins  []string
	results []ports.Result
	audits  []ports.AuditEvent
}

func (r *syncInstrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, ports.EndOperation) {
	r.steps.Record("begin")
	r.begins = append(r.begins, operation)
	return ctx, func(result ports.Result) {
		r.steps.Record("end")
		r.results = append(r.results, result)
	}
}

func (r *syncInstrumentation) Audit(_ context.Context, event ports.AuditEvent) {
	r.steps.Record("audit")
	r.audits = append(r.audits, event)
}

func newInstrumentedSyncHarness(t *testing.T) (*syncHarness, *syncInstrumentation) {
	t.Helper()
	h := newSyncHarness(t)
	instr := &syncInstrumentation{steps: h.rec}
	h.service.Instrumentation = instr
	return h, instr
}

func TestReserveAcceptedReportsAcceptedAndAudits(t *testing.T) {
	h, instr := newInstrumentedSyncHarness(t)

	if _, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1}); err != nil {
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

	if _, err := h.service.Cancel(withExecution(t, context.Background()), application.Cancel{Order: syncOrder}); err != nil {
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

	if _, err := h.service.FindReservation(withExecution(t, context.Background()), syncOrder); err != nil {
		t.Fatalf("FindReservation() error = %v, want nil", err)
	}

	if len(instr.begins) != 1 || instr.begins[0] != application.OperationFindReservation {
		t.Fatalf("BeginOperation operations = %v, want [%q]", instr.begins, application.OperationFindReservation)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none (LOG-14)", instr.audits)
	}
}

func TestBeginPrecedesAuthorizationAndEndClosesTheSequence(t *testing.T) {
	h, _ := newInstrumentedSyncHarness(t)

	if _, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1}); err != nil {
		t.Fatalf("Reserve() error = %v, want nil", err)
	}

	observed := h.rec.Observed()
	if len(observed) < 4 || observed[0] != "begin" {
		t.Fatalf("observed = %v, want begin first — the span opens before step 1 (TRC-16)", observed)
	}
	if observed[1] != "authorize" {
		t.Fatalf("observed = %v, want authorize right after begin", observed)
	}
	if got := observed[len(observed)-2:]; got[0] != "end" || got[1] != "audit" {
		t.Fatalf("observed = %v, want the sequence to close with end then audit", observed)
	}
}
