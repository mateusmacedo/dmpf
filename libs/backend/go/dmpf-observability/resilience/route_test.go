package resilience_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
)

func hop(dependency string, deadline time.Duration) resilience.Operation {
	return resilience.Operation{
		Dependency:        dependency,
		Method:            "Call",
		Kind:              resilience.Remote,
		Deadline:          deadline,
		EstimatedDuration: deadline / 2,
	}
}

func TestARouteThatFitsExactlyPasses(t *testing.T) {
	hops := []resilience.Operation{hop("payments", time.Second), hop("ledger", time.Second)}
	exact := 2*time.Second + 2*resilience.DefaultHopSlack

	if err := resilience.ValidateRoute(exact, hops); err != nil {
		t.Fatalf("ValidateRoute() = %v, want nil at the exact frontier", err)
	}
}

func TestARouteOneNanosecondShortIsRefused(t *testing.T) {
	hops := []resilience.Operation{hop("payments", time.Second), hop("ledger", time.Second)}
	short := 2*time.Second + 2*resilience.DefaultHopSlack - time.Nanosecond

	err := resilience.ValidateRoute(short, hops)

	if !errors.Is(err, resilience.ErrDeadlineComposition) {
		t.Fatalf("ValidateRoute() = %v, want ErrDeadlineComposition", err)
	}
}

func TestEachHopReservesItsOwnSlack(t *testing.T) {
	one := []resilience.Operation{hop("payments", time.Second)}
	three := []resilience.Operation{
		hop("payments", time.Second), hop("ledger", time.Second), hop("audit", time.Second),
	}
	remaining := 3*time.Second + 2*resilience.DefaultHopSlack

	if err := resilience.ValidateRoute(remaining, one); err != nil {
		t.Fatalf("ValidateRoute() = %v for one hop, want nil", err)
	}
	if err := resilience.ValidateRoute(remaining, three); !errors.Is(err, resilience.ErrDeadlineComposition) {
		t.Fatalf("ValidateRoute() = %v for three hops, want a refusal: the third slack does not fit", err)
	}
}

func TestAnEmptyRouteIsAccepted(t *testing.T) {
	if err := resilience.ValidateRoute(0, nil); err != nil {
		t.Fatalf("ValidateRoute() = %v for no hops, want nil", err)
	}
}

func TestARouteWithAnInvalidHopIsRefused(t *testing.T) {
	hops := []resilience.Operation{hop("payments", time.Second), {Dependency: "ledger"}}

	if err := resilience.ValidateRoute(time.Hour, hops); err == nil {
		t.Fatal("ValidateRoute() = nil with a hop that declares no deadline, want a refusal")
	}
}

func TestTheRefusalNamesWhatWasNeededAndWhatRemained(t *testing.T) {
	hops := []resilience.Operation{hop("payments", 5*time.Second)}

	err := resilience.ValidateRoute(time.Second, hops)

	if err == nil {
		t.Fatal("ValidateRoute() = nil, want a refusal")
	}
	message := err.Error()
	for _, expected := range []string{"1 hops", "5.05s", "1s"} {
		if !strings.Contains(message, expected) {
			t.Errorf("refusal = %q, want it to mention %q so the route can be fixed", message, expected)
		}
	}
}
