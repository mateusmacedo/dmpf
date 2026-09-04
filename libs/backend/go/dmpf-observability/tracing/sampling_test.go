package tracing_test

import (
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

func TestTheDefaultRatesAreTheBaselineOfTRC13(t *testing.T) {
	rates := tracing.DefaultRates()

	want := map[tracing.Class]float64{
		tracing.ClassError:       1.0,
		tracing.ClassWrite:       0.10,
		tracing.ClassRead:        0.01,
		tracing.ClassMaintenance: 1.0,
	}
	for class, rate := range want {
		if got := rates.RateFor(class); got != rate {
			t.Errorf("RateFor(%q) = %v, want %v", class, got, rate)
		}
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
