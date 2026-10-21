package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type resolveRig struct {
	ledger *extLedger
	uow    *extUoW
	apply  *extApply
	clock  *countingClock
	inst   *extInstrumentation
}

func newResolveRig() *resolveRig {
	ledger := &extLedger{}
	uow := &extUoW{res: extResources{ledger: ledger}}
	return &resolveRig{ledger: ledger, uow: uow, apply: &extApply{uow: uow}, clock: &countingClock{at: 2_000}, inst: &extInstrumentation{}}
}

func (r *resolveRig) resolution(finding ports.Finding, profile ports.ExternalProfile) usecase.ExternalResolution[extResources] {
	return usecase.ExternalResolution[extResources]{
		UoW:             r.uow,
		Ledger:          func(res extResources) ports.AttemptLedger { return res.ledger },
		Observation:     ports.Observation{Key: dispatchKey, Finding: finding, ID: "o-1"},
		Profile:         profile,
		QueryAvailable:  true,
		Clock:           r.clock,
		Instrumentation: r.inst,
		Apply:           r.apply.apply,
	}
}

func profileWithKey(semantics ports.KeySemantics, authoritative bool) ports.ExternalProfile {
	p := validProfile()
	p.KeySemantics, p.AuthoritativeAbsence = semantics, authoritative
	return p
}

func recorded(class ports.ExternalClass, queries int) ports.Observed {
	return ports.Observed{Result: ports.ObserveRecorded, N: 2, Class: class, QueriesDone: queries}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		finding  ports.Finding
		profile  ports.ExternalProfile
		observed ports.Observed
		want     []usecase.Step
	}{
		{"a duplicate applies nothing", ports.FindingApplied, validProfile(),
			ports.Observed{Result: ports.ObserveDuplicate, Class: ports.ClassUnknownSent, QueriesDone: 1}, []usecase.Step{}},
		{"a conflict applies nothing", ports.FindingRejected, validProfile(),
			ports.Observed{Result: ports.ObserveConflict, Class: ports.ClassUnknownSent, QueriesDone: 1}, []usecase.Step{}},
		{"a saturated attempt applies nothing", ports.FindingApplied, validProfile(),
			ports.Observed{Result: ports.ObserveSaturated, Class: ports.ClassUnknownSent, QueriesDone: 1}, []usecase.Step{}},
		{"an observation with no result applies nothing", ports.FindingApplied, validProfile(),
			ports.Observed{}, []usecase.Step{}},
		{"applied confirms", ports.FindingApplied, validProfile(),
			recorded(ports.ClassUnknownSent, 1), []usecase.Step{usecase.StepConfirm}},
		{"rejected fails", ports.FindingRejected, validProfile(),
			recorded(ports.ClassUnknownSent, 1), []usecase.Step{usecase.StepFail}},
		{"absent under a native key retries with the same key", ports.FindingAbsent, profileWithKey(ports.KeyNative, false),
			recorded(ports.ClassUnknownSent, 1), []usecase.Step{usecase.StepRetry}},
		{"absent under a header key retries with the same key", ports.FindingAbsent, profileWithKey(ports.KeyHeader, false),
			recorded(ports.ClassUnknownSent, 1), []usecase.Step{usecase.StepRetry}},
		{"absent without a key reconciles", ports.FindingAbsent, profileWithKey(ports.KeyNone, false),
			recorded(ports.ClassUnknownSent, 1), []usecase.Step{usecase.StepReconcile}},
		{"absent without a key retries when the absence is authoritative", ports.FindingAbsent, profileWithKey(ports.KeyNone, true),
			recorded(ports.ClassUnknownSent, 1), []usecase.Step{usecase.StepRetry}},
		{"pending under the query ceiling awaits", ports.FindingPending, validProfile(),
			recorded(ports.ClassAccepted, 2), []usecase.Step{usecase.StepAwait}},
		{"pending at the query ceiling reconciles", ports.FindingPending, validProfile(),
			recorded(ports.ClassAccepted, 3), []usecase.Step{usecase.StepReconcile}},
		{"inconclusive under the ceiling queries again", ports.FindingInconclusive, validProfile(),
			recorded(ports.ClassUnknownAmbiguous, 1), []usecase.Step{usecase.StepQuery}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newResolveRig()
			rig.ledger.observed = tt.observed
			r := rig.resolution(tt.finding, tt.profile)

			got, err := usecase.Resolve(context.Background(), r)
			if err != nil {
				t.Fatalf("Resolve() = %v", err)
			}
			if got != tt.observed {
				t.Fatalf("Resolve() = %+v, want %+v", got, tt.observed)
			}
			if steps := rig.apply.stepsTaken(); !slices.Equal(steps, tt.want) || rig.apply.outTx {
				t.Fatalf("applied %v (outside a transaction: %v), want %v", steps, rig.apply.outTx, tt.want)
			}
			if len(rig.ledger.observes) != 1 || rig.ledger.observes[0] != r.Observation || rig.ledger.nows[0] != rig.clock.at {
				t.Fatalf("observed %+v at %v, want %+v at %v", rig.ledger.observes, rig.ledger.nows, r.Observation, rig.clock.at)
			}
			class, conflict := rig.inst.marked()
			if class != tt.observed.Class || conflict != (tt.observed.Result == ports.ObserveConflict) {
				t.Fatalf("marked class %v, conflict %v; want %v, %v", class, conflict, tt.observed.Class, tt.observed.Result == ports.ObserveConflict)
			}
			if saturated := rig.inst.markedSaturated(); saturated != (tt.observed.Result == ports.ObserveSaturated) {
				t.Fatalf("marked saturation %v, want %v", saturated, tt.observed.Result == ports.ObserveSaturated)
			}
			if !slices.Equal(rig.inst.operations, []string{"external.resolve"}) || rig.inst.outcome() != ports.OutcomeAccepted {
				t.Fatalf("instrumented %v ending %+v, want one accepted external.resolve", rig.inst.operations, rig.inst.results)
			}
			for _, step := range rig.apply.steps {
				want := usecase.ExternalStep{Step: step.Step, Attempt: ports.Attempt{Key: dispatchKey, N: tt.observed.N}, Class: tt.observed.Class, Observation: r.Observation}
				if step != want {
					t.Fatalf("applied %+v, want %+v", step, want)
				}
			}
		})
	}
}

func TestResolveFailsWithoutKeepingAnything(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(r *resolveRig)
		want    error
		applied int
	}{
		{"a failed observe", func(r *resolveRig) { r.ledger.observeErr = ports.ErrAttemptInFlight }, ports.ErrAttemptInFlight, 0},
		{"a failed apply", func(r *resolveRig) {
			r.ledger.observed = recorded(ports.ClassUnknownSent, 1)
			r.apply.err = errApply
		}, errApply, 1},
		{"a failed commit", func(r *resolveRig) {
			r.ledger.observed = recorded(ports.ClassUnknownSent, 1)
			r.uow.commitErr = errCommit
		}, errCommit, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newResolveRig()
			tt.arrange(rig)
			got, err := usecase.Resolve(context.Background(), rig.resolution(ports.FindingApplied, validProfile()))
			if !errors.Is(err, tt.want) || got != (ports.Observed{}) {
				t.Fatalf("Resolve() = %+v, %v; want zero, %v", got, err, tt.want)
			}
			if len(rig.apply.steps) != tt.applied {
				t.Fatalf("applied %d steps, want %d", len(rig.apply.steps), tt.applied)
			}
			if class, _ := rig.inst.marked(); class != 0 || rig.inst.outcome() != ports.OutcomeFailed || !errors.Is(rig.inst.results[0].Err, tt.want) {
				t.Fatalf("marked %v, instrumented %+v; want no class and a failure carrying %v", class, rig.inst.results, tt.want)
			}
		})
	}
}

func TestResolveRefusesAnIncompleteResolution(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(r *usecase.ExternalResolution[extResources])
		want   error
	}{
		{"no unit of work", func(r *usecase.ExternalResolution[extResources]) { r.UoW = nil }, usecase.ErrIncompleteResolution},
		{"no ledger accessor", func(r *usecase.ExternalResolution[extResources]) { r.Ledger = nil }, usecase.ErrIncompleteResolution},
		{"no clock", func(r *usecase.ExternalResolution[extResources]) { r.Clock = nil }, usecase.ErrIncompleteResolution},
		{"no apply", func(r *usecase.ExternalResolution[extResources]) { r.Apply = nil }, usecase.ErrIncompleteResolution},
		{"no instrumentation", func(r *usecase.ExternalResolution[extResources]) { r.Instrumentation = nil }, usecase.ErrIncompleteResolution},
		{"an invalid profile", func(r *usecase.ExternalResolution[extResources]) { r.Profile.MaxQueries = 0 }, usecase.ErrInvalidExternalProfile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newResolveRig()
			r := rig.resolution(ports.FindingApplied, validProfile())
			tt.mutate(&r)

			if _, err := usecase.Resolve(context.Background(), r); !errors.Is(err, tt.want) {
				t.Fatalf("Resolve() = %v, want %v", err, tt.want)
			}
			if rig.uow.calls != 0 {
				t.Fatalf("an incomplete resolution opened %d transactions", rig.uow.calls)
			}
		})
	}
}

func TestResolveRollbackKeepsNoObservation(t *testing.T) {
	tests := []struct {
		name      string
		applyErr  error
		commitErr error
		want      error
		wantSeen  []string
	}{
		{"a recorded observation commits with its step", nil, nil, nil, []string{"o-1"}},
		{"a failed apply leaves the observation unrecorded", errApply, nil, errApply, []string{}},
		{"a failed commit leaves the observation unrecorded", nil, errCommit, errCommit, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := newTxLedger()
			ledger.finished[1] = ports.ClassUnknownSent
			uow := &txLedgerUoW{ledger: ledger, commitErr: tt.commitErr}
			r := usecase.ExternalResolution[extResources]{
				UoW:             uow,
				Ledger:          func(res extResources) ports.AttemptLedger { return res.ledger },
				Observation:     ports.Observation{Key: dispatchKey, Finding: ports.FindingApplied, ID: "o-1"},
				Profile:         validProfile(),
				Clock:           &countingClock{at: 2_000},
				Instrumentation: ports.NoInstrumentation(),
				Apply:           func(context.Context, extResources, usecase.ExternalStep) error { return tt.applyErr },
			}

			if _, err := usecase.Resolve(context.Background(), r); !errors.Is(err, tt.want) {
				t.Fatalf("Resolve() = %v, want %v", err, tt.want)
			}
			if !slices.Equal(append([]string{}, ledger.observed...), tt.wantSeen) {
				t.Fatalf("observations kept = %v, want %v", ledger.observed, tt.wantSeen)
			}
		})
	}
}
