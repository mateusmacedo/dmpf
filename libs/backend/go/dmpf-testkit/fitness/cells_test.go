package fitness_test

import (
	"fmt"
	"strings"
	"testing"

	conffit "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/fitness"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/fitness"
)

func TestCellsEachHaveAWorkingPair(t *testing.T) {
	for _, c := range fitness.Cells {
		t.Run(fmt.Sprintf("%02d-%s-%s", c.N, c.Source, c.Target), func(t *testing.T) {
			if err := c.Verify(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The oracle of RFC §7.4 by hand: 36 cells, every ordered pair of the six
// blocks exactly once, 17 allowed and 19 forbidden.
func TestCellsMatchTheNormativeOracle(t *testing.T) {
	if len(fitness.Cells) != 36 {
		t.Fatalf("%d cells, want 36", len(fitness.Cells))
	}
	blocks := []conffit.Block{conffit.BlockDomain, conffit.BlockApplication, conffit.BlockApp, conffit.BlockPort, conffit.BlockProvider, conffit.BlockContract}
	seen := map[[2]conffit.Block]int{}
	var allowed, forbidden, d001, d002, byConstruction int
	for i, c := range fitness.Cells {
		if c.N != i+1 {
			t.Errorf("cell at index %d is numbered %d", i, c.N)
		}
		k := [2]conffit.Block{c.Source, c.Target}
		if n, dup := seen[k]; dup {
			t.Errorf("pair %s -> %s appears in cells %d and %d", c.Source, c.Target, n, c.N)
		}
		seen[k] = c.N
		if c.Allowed {
			allowed++
		} else {
			forbidden++
		}
		switch c.NegativeCode() {
		case conffit.CodeD001:
			d001++
		case conffit.CodeD002:
			d002++
		case "":
			byConstruction++
		}
	}
	for _, s := range blocks {
		for _, tg := range blocks {
			if _, ok := seen[[2]conffit.Block{s, tg}]; !ok {
				t.Errorf("pair %s -> %s missing", s, tg)
			}
		}
	}
	if allowed != 17 || forbidden != 19 {
		t.Errorf("%d allowed / %d forbidden, want 17 / 19", allowed, forbidden)
	}
	if d001 != 19 || d002 != 14 || byConstruction != 3 {
		t.Errorf("negatives: %d D001, %d D002, %d by construction; want 19, 14, 3 (cells 18, 30, 36 target a contract)", d001, d002, byConstruction)
	}
}

// RAS-06: a negative the checker accepts must fail the suite, naming the cell.
func TestATamperedCellIsNamedByVerify(t *testing.T) {
	tampered := fitness.Cell{N: 1, Source: conffit.BlockDomain, Target: conffit.BlockDomain, Allowed: false}
	err := tampered.Verify()
	if err == nil {
		t.Fatal("a negative that passes went unnoticed")
	}
	if !strings.Contains(err.Error(), "cell 1") || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error does not name the cell and the failing side: %v", err)
	}

	inverted := fitness.Cell{N: 4, Source: conffit.BlockDomain, Target: conffit.BlockPort, Allowed: true}
	err = inverted.Verify()
	if err == nil || !strings.Contains(err.Error(), "cell 4") || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("a positive that fails was not named: %v", err)
	}
}
