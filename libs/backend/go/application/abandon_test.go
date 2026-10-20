package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type abandonRig struct {
	ledger *extLedger
	uow    *extUoW
	apply  *extApply
	clock  *countingClock
	inst   *extInstrumentation
}

func newAbandonRig() *abandonRig {
	ledger := &extLedger{finished: ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent}}
	uow := &extUoW{res: extResources{ledger: ledger}}
	return &abandonRig{ledger: ledger, uow: uow, apply: &extApply{uow: uow}, clock: &countingClock{at: 3_000}, inst: &extInstrumentation{}}
}

func (r *abandonRig) abandon(queryAvailable bool) usecase.ExternalAbandon[extResources] {
	return usecase.ExternalAbandon[extResources]{
		UoW:             r.uow,
		Ledger:          func(res extResources) ports.AttemptLedger { return res.ledger },
		Attempt:         openedAttempt(2),
		Profile:         validProfile(),
		QueryAvailable:  queryAvailable,
		Clock:           r.clock,
		Instrumentation: r.inst,
		Apply:           r.apply.apply,
	}
}

func tenantContext(t *testing.T, tenant *ports.TenantID) context.Context {
	t.Helper()
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1",
		Tenant: tenant, Deadline: 1, Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return ports.WithExternalClassSlot(ports.WithExecutionContext(context.Background(), execution))
}

func TestAbandon(t *testing.T) {
	tests := []struct {
		name           string
		finished       ports.Finished
		queryAvailable bool
		want           []usecase.Step
	}{
		{"a recorded abandon goes to the query", ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent},
			true, []usecase.Step{usecase.StepQuery}},
		{"a recorded abandon without a query reconciles", ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent},
			false, []usecase.Step{usecase.StepReconcile}},
		{"an attempt the redelivery already abandoned applies nothing", ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassUnknownSent},
			true, []usecase.Step{}},
		{"an attempt the handler finished first applies nothing", ports.Finished{Result: ports.FinishConflict, Recorded: ports.ClassAccepted},
			true, []usecase.Step{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newAbandonRig()
			rig.ledger.finished = tt.finished
			tenant := ports.TenantID("acme")
			ctx := tenantContext(t, &tenant)

			got, err := usecase.Abandon(ctx, rig.abandon(tt.queryAvailable))
			if err != nil {
				t.Fatalf("Abandon() = %v", err)
			}
			if got != tt.finished {
				t.Fatalf("Abandon() = %+v, want %+v", got, tt.finished)
			}
			finish := ports.AttemptOutcome{Key: dispatchKey, N: 2, Class: ports.ClassUnknownSent, FinishedAt: rig.clock.at, Presumed: true}
			if len(rig.ledger.finishes) != 1 || rig.ledger.finishes[0] != finish {
				t.Fatalf("finished %+v, want %+v", rig.ledger.finishes, finish)
			}
			if steps := rig.apply.stepsTaken(); !slices.Equal(steps, tt.want) || rig.apply.outTx {
				t.Fatalf("applied %v (outside a transaction: %v), want %v", steps, rig.apply.outTx, tt.want)
			}
			for _, step := range rig.apply.steps {
				want := usecase.ExternalStep{Step: step.Step, Attempt: openedAttempt(2), Class: ports.ClassUnknownSent}
				if step != want {
					t.Fatalf("applied %+v, want %+v", step, want)
				}
			}
			class, conflict := rig.inst.marked()
			if class != tt.finished.Recorded || conflict != (tt.finished.Result == ports.FinishConflict) {
				t.Fatalf("marked class %v, conflict %v; want %v, %v", class, conflict, tt.finished.Recorded, tt.finished.Result == ports.FinishConflict)
			}
			if !slices.Equal(rig.inst.operations, []string{"external.abandon"}) || rig.inst.outcome() != ports.OutcomeAccepted {
				t.Fatalf("instrumented %v ending %+v, want one accepted external.abandon", rig.inst.operations, rig.inst.results)
			}
		})
	}
}

func TestAbandonFailsWithoutClosingAnything(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(r *abandonRig)
		want    error
	}{
		{"a failed finish", func(r *abandonRig) { r.ledger.finishErr = errLedger }, errLedger},
		{"a failed apply rolls the finish back with it", func(r *abandonRig) { r.apply.err = errApply }, errApply},
		{"a failed commit", func(r *abandonRig) { r.uow.commitErr = errCommit }, errCommit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newAbandonRig()
			tt.arrange(rig)
			tenant := ports.TenantID("acme")
			ctx := tenantContext(t, &tenant)

			got, err := usecase.Abandon(ctx, rig.abandon(true))
			if !errors.Is(err, tt.want) || got != (ports.Finished{}) {
				t.Fatalf("Abandon() = %+v, %v; want zero, %v", got, err, tt.want)
			}
			if rig.uow.calls != 1 {
				t.Fatalf("withins = %d, want the finish and the apply in one", rig.uow.calls)
			}
			if class, _ := rig.inst.marked(); class != 0 || rig.inst.outcome() != ports.OutcomeFailed {
				t.Fatalf("marked %v, instrumented %+v; want no class and a failure", class, rig.inst.results)
			}
		})
	}
}

func TestAbandonRefusesAnAttemptOutsideItsTenant(t *testing.T) {
	other := ports.TenantID("globex")
	tests := []struct {
		name string
		ctx  func(t *testing.T) context.Context
		want error
	}{
		{"no execution context", func(*testing.T) context.Context { return context.Background() }, ports.ErrContextAbsent},
		{"no tenant", func(t *testing.T) context.Context { return tenantContext(t, nil) }, usecase.ErrAbandonOutsideTenant},
		{"another tenant", func(t *testing.T) context.Context { return tenantContext(t, &other) }, usecase.ErrAbandonOutsideTenant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newAbandonRig()

			if _, err := usecase.Abandon(tt.ctx(t), rig.abandon(true)); !errors.Is(err, tt.want) {
				t.Fatalf("Abandon() = %v, want %v", err, tt.want)
			}
			if rig.uow.calls != 0 {
				t.Fatalf("an abandon outside its tenant opened %d transactions", rig.uow.calls)
			}
		})
	}
}

func TestAbandonRefusesAnIncompleteAbandon(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(a *usecase.ExternalAbandon[extResources])
		want   error
	}{
		{"no unit of work", func(a *usecase.ExternalAbandon[extResources]) { a.UoW = nil }, usecase.ErrIncompleteAbandon},
		{"no ledger accessor", func(a *usecase.ExternalAbandon[extResources]) { a.Ledger = nil }, usecase.ErrIncompleteAbandon},
		{"no clock", func(a *usecase.ExternalAbandon[extResources]) { a.Clock = nil }, usecase.ErrIncompleteAbandon},
		{"no apply", func(a *usecase.ExternalAbandon[extResources]) { a.Apply = nil }, usecase.ErrIncompleteAbandon},
		{"no instrumentation", func(a *usecase.ExternalAbandon[extResources]) { a.Instrumentation = nil }, usecase.ErrIncompleteAbandon},
		{"no key", func(a *usecase.ExternalAbandon[extResources]) { a.Attempt.Key = "" }, usecase.ErrIncompleteAbandon},
		{"attempt zero", func(a *usecase.ExternalAbandon[extResources]) { a.Attempt.N = 0 }, usecase.ErrIncompleteAbandon},
		{"an invalid profile", func(a *usecase.ExternalAbandon[extResources]) { a.Profile.MaxQueries = 0 }, usecase.ErrInvalidExternalProfile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newAbandonRig()
			a := rig.abandon(true)
			tt.mutate(&a)
			tenant := ports.TenantID("acme")

			if _, err := usecase.Abandon(tenantContext(t, &tenant), a); !errors.Is(err, tt.want) {
				t.Fatalf("Abandon() = %v, want %v", err, tt.want)
			}
			if rig.uow.calls != 0 {
				t.Fatalf("an incomplete abandon opened %d transactions", rig.uow.calls)
			}
		})
	}
}

func TestAbandonRollbackLeavesTheAttemptBegun(t *testing.T) {
	tests := []struct {
		name         string
		applyErr     error
		commitErr    error
		want         error
		wantFinished bool
	}{
		{"a recorded abandon commits with its step", nil, nil, nil, true},
		{"a failed apply leaves the attempt begun", errApply, nil, errApply, false},
		{"a failed commit leaves the attempt begun", nil, errCommit, errCommit, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := newTxLedger()
			a := usecase.ExternalAbandon[extResources]{
				UoW:             &txLedgerUoW{ledger: ledger, commitErr: tt.commitErr},
				Ledger:          func(res extResources) ports.AttemptLedger { return res.ledger },
				Attempt:         openedAttempt(2),
				Profile:         validProfile(),
				Clock:           &countingClock{at: 3_000},
				Instrumentation: ports.NoInstrumentation(),
				Apply:           func(context.Context, extResources, usecase.ExternalStep) error { return tt.applyErr },
			}
			tenant := ports.TenantID("acme")

			if _, err := usecase.Abandon(tenantContext(t, &tenant), a); !errors.Is(err, tt.want) {
				t.Fatalf("Abandon() = %v, want %v", err, tt.want)
			}
			if _, finished := ledger.finished[2]; finished != tt.wantFinished {
				t.Fatalf("attempt 2 finished = %v, want %v", finished, tt.wantFinished)
			}
		})
	}
}
