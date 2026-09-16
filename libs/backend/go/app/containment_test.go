package app_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestEveryReasonHasADeclaredMechanism(t *testing.T) {
	t.Parallel()
	reasons := []ports.Reason{
		ports.ReasonInvalidEnvelope,
		ports.ReasonTerminalFailure,
		ports.ReasonCollision,
		ports.ReasonAttemptsExhausted,
	}
	for _, reason := range reasons {
		mechanism, ok := app.MechanismFor(reason)
		if !ok {
			t.Fatalf("reason %q has no declared mechanism (GAR-11)", reason)
		}
		if mechanism != app.MechanismQuarantine {
			t.Fatalf("reason %q maps to %q; this delivery realizes quarantine only", reason, mechanism)
		}
	}
	if len(app.ContainmentMap) != len(reasons) {
		t.Fatalf("ContainmentMap has %d entries, want exactly the %d reasons of ports", len(app.ContainmentMap), len(reasons))
	}
}

func TestUnknownReasonHasNoMechanism(t *testing.T) {
	t.Parallel()
	if _, ok := app.MechanismFor(ports.Reason("made-up")); ok {
		t.Fatal("a reason outside the port's enumeration must not be routed anywhere")
	}
}
