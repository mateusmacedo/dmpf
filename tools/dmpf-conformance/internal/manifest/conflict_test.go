package manifest

import (
	"slices"
	"testing"
)

func TestConflictUnit(t *testing.T) {
	x := Exception{
		Unit:          "a",
		Object:        ExceptionObject{Unit: "b", PresentUnit: true},
		PresentObject: true,
	}
	got := x.ConflictPairs()
	if !slices.Contains(got, "unit vs object.unit") {
		t.Fatalf("expected conflict on unit, got %v", got)
	}
}

func TestConflictDependency(t *testing.T) {
	x := Exception{
		Dependency:    "reflect",
		Object:        ExceptionObject{Identity: "unsafe", PresentIdentity: true},
		PresentObject: true,
	}
	got := x.ConflictPairs()
	if !slices.Contains(got, "dependency vs object.identity") {
		t.Fatalf("expected conflict on dependency, got %v", got)
	}
}

func TestConflictReason(t *testing.T) {
	x := Exception{
		Reason:               "old",
		Justification:        "new",
		PresentJustification: true,
	}
	got := x.ConflictPairs()
	if !slices.Contains(got, "reason vs justification") {
		t.Fatalf("expected conflict on reason, got %v", got)
	}
}

func TestNoConflictWhenEqual(t *testing.T) {
	x := Exception{
		Unit:                 "u",
		Dependency:           "d",
		Reason:               "r",
		Object:               ExceptionObject{Unit: "u", Identity: "d", PresentUnit: true, PresentIdentity: true},
		PresentObject:        true,
		Justification:        "r",
		PresentJustification: true,
	}
	if got := x.ConflictPairs(); len(got) != 0 {
		t.Fatalf("expected no conflict, got %v", got)
	}
}

func TestNoConflictWhenNewAbsent(t *testing.T) {
	x := Exception{
		Unit:       "u",
		Dependency: "d",
		Reason:     "r",
	}
	if got := x.ConflictPairs(); len(got) != 0 {
		t.Fatalf("expected no conflict for legacy-only, got %v", got)
	}
}
