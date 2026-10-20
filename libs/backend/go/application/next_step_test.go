package application_test

import (
	"fmt"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const stepMaxQueries = 3

func allClasses() []ports.ExternalClass {
	classes := make([]ports.ExternalClass, 0, 11)
	for c := ports.ExternalClass(0); c <= ports.ClassSchemaMismatch+1; c++ {
		classes = append(classes, c)
	}
	return classes
}

func allFindings() []ports.Finding {
	findings := make([]ports.Finding, 0, 7)
	for f := ports.Finding(0); f <= ports.FindingInconclusive+1; f++ {
		findings = append(findings, f)
	}
	return findings
}

func stepGrid() []usecase.StepInput {
	var grid []usecase.StepInput
	for _, class := range allClasses() {
		for _, finding := range allFindings() {
			for key := ports.KeySemantics(0); key <= ports.KeyNone; key++ {
				for efficacy := ports.Efficacy(0); efficacy <= ports.EfficacyByObservation; efficacy++ {
					for _, authoritative := range []bool{false, true} {
						for _, queryAvailable := range []bool{false, true} {
							for _, done := range []int{0, stepMaxQueries - 1, stepMaxQueries, stepMaxQueries + 1} {
								grid = append(grid, usecase.StepInput{
									Class:   class,
									Finding: finding,
									Profile: ports.ExternalProfile{
										KeySemantics:         key,
										Efficacy:             efficacy,
										AuthoritativeAbsence: authoritative,
										MaxQueries:           stepMaxQueries,
									},
									QueryAvailable: queryAvailable,
									QueriesDone:    done,
								})
							}
						}
					}
				}
			}
		}
	}
	return grid
}

func uncertain(c ports.ExternalClass) bool {
	switch c {
	case ports.ClassUnknownSent, ports.ClassUnknownAmbiguous, ports.ClassSchemaMismatch:
		return true
	default:
		return c == 0 || c > ports.ClassSchemaMismatch
	}
}

type stepProperty struct {
	name  string
	holds func(in usecase.StepInput, got usecase.Step, step func(usecase.StepInput) usecase.Step) bool
}

func stepProperties() []stepProperty {
	return []stepProperty{
		{"uncertain class without finding never confirms, fails nor retries", func(in usecase.StepInput, got usecase.Step, _ func(usecase.StepInput) usecase.Step) bool {
			if in.Finding != 0 || !uncertain(in.Class) {
				return true
			}
			return got != usecase.StepConfirm && got != usecase.StepFail && got != usecase.StepRetry
		}},
		{"schema mismatch without finding suspends", func(in usecase.StepInput, got usecase.Step, _ func(usecase.StepInput) usecase.Step) bool {
			if in.Finding != 0 || in.Class != ports.ClassSchemaMismatch {
				return true
			}
			return got == usecase.StepSuspend
		}},
		{"a finding decides regardless of the class", func(in usecase.StepInput, got usecase.Step, step func(usecase.StepInput) usecase.Step) bool {
			if in.Finding == 0 {
				return true
			}
			for _, class := range allClasses() {
				other := in
				other.Class = class
				if step(other) != got {
					return false
				}
			}
			return true
		}},
		{"the query ceiling ends querying and awaiting", func(in usecase.StepInput, got usecase.Step, _ func(usecase.StepInput) usecase.Step) bool {
			if in.QueriesDone < in.Profile.MaxQueries {
				return true
			}
			return got != usecase.StepQuery && got != usecase.StepAwait
		}},
		{"no query without a state port", func(in usecase.StepInput, got usecase.Step, _ func(usecase.StepInput) usecase.Step) bool {
			return in.QueryAvailable || got != usecase.StepQuery
		}},
	}
}

func violation(p stepProperty, step func(usecase.StepInput) usecase.Step) string {
	for _, in := range stepGrid() {
		if got := step(in); !p.holds(in, got, step) {
			return fmt.Sprintf("%+v -> %s", in, got)
		}
	}
	return ""
}

func TestNextStepHoldsEveryPropertyOverTheWholeGrid(t *testing.T) {
	for _, p := range stepProperties() {
		t.Run(p.name, func(t *testing.T) {
			if v := violation(p, usecase.NextStep); v != "" {
				t.Fatalf("violated at %s", v)
			}
		})
	}
}

func TestEveryStepPropertyRejectsAPermissiveStep(t *testing.T) {
	mutants := map[string]func(usecase.StepInput) usecase.Step{
		"uncertain class without finding never confirms, fails nor retries": func(usecase.StepInput) usecase.Step { return usecase.StepRetry },
		"schema mismatch without finding suspends":                          func(usecase.StepInput) usecase.Step { return usecase.StepReconcile },
		"a finding decides regardless of the class": func(in usecase.StepInput) usecase.Step {
			if in.Class == ports.ClassAccepted {
				return usecase.StepConfirm
			}
			return usecase.NextStep(in)
		},
		"the query ceiling ends querying and awaiting": func(usecase.StepInput) usecase.Step { return usecase.StepAwait },
		"no query without a state port":                func(usecase.StepInput) usecase.Step { return usecase.StepQuery },
	}
	for _, p := range stepProperties() {
		t.Run(p.name, func(t *testing.T) {
			mutant, ok := mutants[p.name]
			if !ok {
				t.Fatal("property without a red control")
			}
			if violation(p, mutant) == "" {
				t.Fatal("the property accepted a step that breaks it: the oracle is permissive")
			}
		})
	}
}

func TestNextStepFollowsTheSpecTable(t *testing.T) {
	profile := func(key ports.KeySemantics, efficacy ports.Efficacy, authoritative bool) ports.ExternalProfile {
		return ports.ExternalProfile{KeySemantics: key, Efficacy: efficacy, AuthoritativeAbsence: authoritative, MaxQueries: stepMaxQueries}
	}
	native := profile(ports.KeyNative, ports.EfficacyOnSubmit, false)
	tests := []struct {
		name string
		in   usecase.StepInput
		want usecase.Step
	}{
		{"accepted on submit", usecase.StepInput{Class: ports.ClassAccepted, Profile: native}, usecase.StepConfirm},
		{"accepted by observation", usecase.StepInput{Class: ports.ClassAccepted, Profile: profile(ports.KeyNative, ports.EfficacyByObservation, false)}, usecase.StepAwait},
		{"accepted by observation past the ceiling", usecase.StepInput{Class: ports.ClassAccepted, Profile: profile(ports.KeyNative, ports.EfficacyByObservation, false), QueriesDone: stepMaxQueries}, usecase.StepReconcile},
		{"retryable transport", usecase.StepInput{Class: ports.ClassRetryableTransport, Profile: native}, usecase.StepRetry},
		{"retryable upstream", usecase.StepInput{Class: ports.ClassRetryableUpstream, Profile: native}, usecase.StepRetry},
		{"final validation", usecase.StepInput{Class: ports.ClassFinalValidation, Profile: native}, usecase.StepFail},
		{"final business", usecase.StepInput{Class: ports.ClassFinalBusiness, Profile: native}, usecase.StepFail},
		{"final authorization", usecase.StepInput{Class: ports.ClassFinalAuthorization, Profile: native}, usecase.StepFail},
		{"unknown sent with query", usecase.StepInput{Class: ports.ClassUnknownSent, Profile: native, QueryAvailable: true}, usecase.StepQuery},
		{"unknown sent without query", usecase.StepInput{Class: ports.ClassUnknownSent, Profile: native}, usecase.StepReconcile},
		{"unknown ambiguous with query", usecase.StepInput{Class: ports.ClassUnknownAmbiguous, Profile: native, QueryAvailable: true}, usecase.StepQuery},
		{"unknown ambiguous without query", usecase.StepInput{Class: ports.ClassUnknownAmbiguous, Profile: native}, usecase.StepReconcile},
		{"unknown sent with query past the ceiling", usecase.StepInput{Class: ports.ClassUnknownSent, Profile: native, QueryAvailable: true, QueriesDone: stepMaxQueries}, usecase.StepReconcile},
		{"schema mismatch", usecase.StepInput{Class: ports.ClassSchemaMismatch, Profile: native, QueryAvailable: true}, usecase.StepSuspend},
		{"no class with query", usecase.StepInput{Profile: native, QueryAvailable: true}, usecase.StepQuery},
		{"no class without query", usecase.StepInput{Profile: native}, usecase.StepReconcile},
		{"class out of range", usecase.StepInput{Class: ports.ClassSchemaMismatch + 1, Profile: native}, usecase.StepReconcile},
		{"finding applied", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingApplied, Profile: native}, usecase.StepConfirm},
		{"finding rejected", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingRejected, Profile: native}, usecase.StepFail},
		{"finding absent with native key", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingAbsent, Profile: native}, usecase.StepRetry},
		{"finding absent with header key", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingAbsent, Profile: profile(ports.KeyHeader, ports.EfficacyOnSubmit, false)}, usecase.StepRetry},
		{"finding absent, no key, authoritative", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingAbsent, Profile: profile(ports.KeyNone, ports.EfficacyOnSubmit, true)}, usecase.StepRetry},
		{"finding absent, no key, not authoritative", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingAbsent, Profile: profile(ports.KeyNone, ports.EfficacyOnSubmit, false)}, usecase.StepReconcile},
		{"finding pending below the ceiling", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingPending, Profile: native, QueryAvailable: true, QueriesDone: stepMaxQueries - 1}, usecase.StepAwait},
		{"finding pending at the ceiling", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingPending, Profile: native, QueryAvailable: true, QueriesDone: stepMaxQueries}, usecase.StepReconcile},
		{"finding inconclusive below the ceiling", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingInconclusive, Profile: native, QueryAvailable: true, QueriesDone: stepMaxQueries - 1}, usecase.StepQuery},
		{"finding inconclusive at the ceiling", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingInconclusive, Profile: native, QueryAvailable: true, QueriesDone: stepMaxQueries}, usecase.StepReconcile},
		{"finding inconclusive without query", usecase.StepInput{Class: ports.ClassUnknownSent, Finding: ports.FindingInconclusive, Profile: native}, usecase.StepReconcile},
		{"finding applied overrides a final class", usecase.StepInput{Class: ports.ClassFinalBusiness, Finding: ports.FindingApplied, Profile: native}, usecase.StepConfirm},
		{"finding rejected overrides schema mismatch", usecase.StepInput{Class: ports.ClassSchemaMismatch, Finding: ports.FindingRejected, Profile: native}, usecase.StepFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usecase.NextStep(tt.in); got != tt.want {
				t.Fatalf("NextStep = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestTheSpecTableTellsNeighbouringStepsApart(t *testing.T) {
	onSubmit := usecase.NextStep(usecase.StepInput{Class: ports.ClassAccepted, Profile: ports.ExternalProfile{Efficacy: ports.EfficacyOnSubmit, MaxQueries: stepMaxQueries}})
	byObservation := usecase.NextStep(usecase.StepInput{Class: ports.ClassAccepted, Profile: ports.ExternalProfile{Efficacy: ports.EfficacyByObservation, MaxQueries: stepMaxQueries}})
	if onSubmit == byObservation {
		t.Fatalf("efficacy changes nothing: both answer %s", onSubmit)
	}
	authoritative := usecase.NextStep(usecase.StepInput{Finding: ports.FindingAbsent, Profile: ports.ExternalProfile{KeySemantics: ports.KeyNone, AuthoritativeAbsence: true, MaxQueries: stepMaxQueries}})
	silent := usecase.NextStep(usecase.StepInput{Finding: ports.FindingAbsent, Profile: ports.ExternalProfile{KeySemantics: ports.KeyNone, MaxQueries: stepMaxQueries}})
	if authoritative == silent {
		t.Fatalf("authoritative absence changes nothing: both answer %s", silent)
	}
}

func TestEveryStepNamesItself(t *testing.T) {
	tests := []struct {
		step usecase.Step
		want string
	}{
		{0, "invalid"},
		{usecase.StepConfirm, "confirm"},
		{usecase.StepAwait, "await"},
		{usecase.StepRetry, "retry"},
		{usecase.StepFail, "fail"},
		{usecase.StepQuery, "query"},
		{usecase.StepReconcile, "reconcile"},
		{usecase.StepSuspend, "suspend"},
		{usecase.StepSuspend + 1, "invalid"},
	}
	for _, tt := range tests {
		if got := tt.step.String(); got != tt.want {
			t.Errorf("Step(%d).String() = %q, want %q", tt.step, got, tt.want)
		}
	}
}
