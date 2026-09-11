package dmpfapp_test

import (
	"testing"

	dmpfapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

func TestEveryReasonHasADeclaredMechanism(t *testing.T) {
	t.Parallel()
	reasons := []dmpfports.Reason{
		dmpfports.ReasonInvalidEnvelope,
		dmpfports.ReasonTerminalFailure,
		dmpfports.ReasonCollision,
		dmpfports.ReasonAttemptsExhausted,
	}
	for _, reason := range reasons {
		mechanism, ok := dmpfapp.MechanismFor(reason)
		if !ok {
			t.Fatalf("reason %q has no declared mechanism (GAR-11)", reason)
		}
		if mechanism != dmpfapp.MechanismQuarantine {
			t.Fatalf("reason %q maps to %q; this delivery realizes quarantine only", reason, mechanism)
		}
	}
	if len(dmpfapp.ContainmentMap) != len(reasons) {
		t.Fatalf("ContainmentMap has %d entries, want exactly the %d reasons of dmpf-ports", len(dmpfapp.ContainmentMap), len(reasons))
	}
}

func TestUnknownReasonHasNoMechanism(t *testing.T) {
	t.Parallel()
	if _, ok := dmpfapp.MechanismFor(dmpfports.Reason("made-up")); ok {
		t.Fatal("a reason outside the port's enumeration must not be routed anywhere")
	}
}
