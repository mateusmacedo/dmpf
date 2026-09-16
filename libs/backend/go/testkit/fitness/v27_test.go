package fitness_test

import (
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/fitness"
)

func TestV27IsRegisteredAsTypeScriptOnly(t *testing.T) {
	v, ok := fitness.Lookup("V27")
	if !ok {
		t.Fatal("V27 missing from the vector table")
	}
	if v.SingleStack != "typescript" {
		t.Fatalf("V27.SingleStack = %q, want typescript (RAS-15)", v.SingleStack)
	}
	if v.Reason == "" {
		t.Fatal("V27 has no reason: an asymmetry is registered, never left implicit (RAS-16)")
	}
	if gaps := fitness.Gaps("go"); len(gaps) != 0 {
		t.Fatalf("Go has %d unexplained gap(s): %+v", len(gaps), gaps)
	}
}

func TestVectorsCoverV13ToV32Once(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range fitness.Vectors {
		if seen[v.ID] {
			t.Errorf("vector %s listed twice", v.ID)
		}
		seen[v.ID] = true
		if v.Rule == "" || v.Where == "" {
			t.Errorf("vector %s lacks a rule or a place of proof (RAS-01)", v.ID)
		}
	}
	for n := 13; n <= 32; n++ {
		if id := fmt.Sprintf("V%d", n); !seen[id] {
			t.Errorf("vector %s missing", id)
		}
	}
	if len(fitness.Vectors) != 20 {
		t.Errorf("%d vectors, want 20 (V13..V32)", len(fitness.Vectors))
	}
}

func TestGapsReportsAnUnexplainedSingleStackVector(t *testing.T) {
	saved := fitness.Vectors
	t.Cleanup(func() { fitness.Vectors = saved })
	fitness.Vectors = append([]fitness.Vector{{ID: "V99", Rule: "x", SingleStack: "typescript", Where: "y"}}, saved...)
	if gaps := fitness.Gaps("go"); len(gaps) != 1 || gaps[0].ID != "V99" {
		t.Fatalf("Gaps(go) = %+v, want exactly V99", gaps)
	}
	if gaps := fitness.Gaps("typescript"); len(gaps) != 0 {
		t.Fatalf("Gaps(typescript) = %+v, want none: V99 is realized there", gaps)
	}
}
