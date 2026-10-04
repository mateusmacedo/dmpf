package tracing_test

import (
	"maps"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func TestTheDefaultRatesAreTheBaselineOfTRC13(t *testing.T) {
	rates := tracing.DefaultRates()

	want := tracing.Rates{
		tracing.ClassError:        1.0,
		tracing.ClassWrite:        1.0,
		tracing.ClassRead:         0.01,
		tracing.ClassMaintenance:  1.0,
		tracing.ClassUnclassified: 0.01,
	}
	if !maps.Equal(rates, want) {
		t.Errorf("DefaultRates() = %v, want the five classes of TRC-13 declared at %v", rates, want)
	}
}

func TestAnUndeclaredClassTakesTheMostRestrictiveRate(t *testing.T) {
	rates := tracing.DefaultRates()

	for _, class := range []tracing.Class{tracing.ClassUnclassified, "something-new", ""} {
		if got := rates.RateFor(class); got != tracing.RateMostRestrictive {
			t.Errorf("RateFor(%q) = %v, want %v — an undeclared class must not pass everything through",
				class, got, tracing.RateMostRestrictive)
		}
	}
}

func TestAnEmptyRateSetIsStillRestrictive(t *testing.T) {
	if got := (tracing.Rates{}).RateFor(tracing.ClassError); got != tracing.RateMostRestrictive {
		t.Fatalf("RateFor() = %v on an empty set, want %v", got, tracing.RateMostRestrictive)
	}
}

func TestADeclaredZeroRateIsHonoured(t *testing.T) {
	rates := tracing.Rates{tracing.ClassRead: 0}

	if got := rates.RateFor(tracing.ClassRead); got != 0 {
		t.Fatalf("RateFor(read) = %v, want 0 — a declared zero is a decision, not an absence", got)
	}
}

func TestDefaultRatesReturnsAFreshMap(t *testing.T) {
	tracing.DefaultRates()[tracing.ClassRead] = 1.0

	if got := tracing.DefaultRates().RateFor(tracing.ClassRead); got != 0.01 {
		t.Fatalf("RateFor(read) = %v after a caller rewrote the map, want 0.01", got)
	}
}

func TestUniformRatesKeepsErrorsAtFullRate(t *testing.T) {
	rates := tracing.UniformRates(0.5)
	for class, want := range map[tracing.Class]float64{
		tracing.ClassError: 1, tracing.ClassMaintenance: 1,
		tracing.ClassWrite: 0.5, tracing.ClassRead: 0.5, tracing.ClassUnclassified: 0.5,
	} {
		if got := rates.RateFor(class); got != want {
			t.Fatalf("RateFor(%s) = %v, want %v", class, got, want)
		}
	}
}
