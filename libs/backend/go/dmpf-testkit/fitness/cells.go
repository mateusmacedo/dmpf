package fitness

import (
	"fmt"

	conffit "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/fitness"
)

// Cell is one of the 36 cells of RFC §7.4, numbered as the norm numbers them.
type Cell struct {
	N       int
	Source  conffit.Block
	Target  conffit.Block
	Allowed bool
}

// Transcribed by hand from RFC §7.4, like the checker's own oracle: deriving
// it from the matrix would compare the datum with itself.
var Cells = []Cell{
	{1, conffit.BlockDomain, conffit.BlockDomain, true},
	{2, conffit.BlockDomain, conffit.BlockApplication, false},
	{3, conffit.BlockDomain, conffit.BlockApp, false},
	{4, conffit.BlockDomain, conffit.BlockPort, false},
	{5, conffit.BlockDomain, conffit.BlockProvider, false},
	{6, conffit.BlockDomain, conffit.BlockContract, false},
	{7, conffit.BlockApplication, conffit.BlockDomain, true},
	{8, conffit.BlockApplication, conffit.BlockApplication, true},
	{9, conffit.BlockApplication, conffit.BlockApp, false},
	{10, conffit.BlockApplication, conffit.BlockPort, true},
	{11, conffit.BlockApplication, conffit.BlockProvider, false},
	{12, conffit.BlockApplication, conffit.BlockContract, false},
	{13, conffit.BlockApp, conffit.BlockDomain, true},
	{14, conffit.BlockApp, conffit.BlockApplication, true},
	{15, conffit.BlockApp, conffit.BlockApp, true},
	{16, conffit.BlockApp, conffit.BlockPort, true},
	{17, conffit.BlockApp, conffit.BlockProvider, true},
	{18, conffit.BlockApp, conffit.BlockContract, true},
	{19, conffit.BlockPort, conffit.BlockDomain, true},
	{20, conffit.BlockPort, conffit.BlockApplication, false},
	{21, conffit.BlockPort, conffit.BlockApp, false},
	{22, conffit.BlockPort, conffit.BlockPort, true},
	{23, conffit.BlockPort, conffit.BlockProvider, false},
	{24, conffit.BlockPort, conffit.BlockContract, false},
	{25, conffit.BlockProvider, conffit.BlockDomain, true},
	{26, conffit.BlockProvider, conffit.BlockApplication, false},
	{27, conffit.BlockProvider, conffit.BlockApp, false},
	{28, conffit.BlockProvider, conffit.BlockPort, true},
	{29, conffit.BlockProvider, conffit.BlockProvider, true},
	{30, conffit.BlockProvider, conffit.BlockContract, true},
	{31, conffit.BlockContract, conffit.BlockDomain, false},
	{32, conffit.BlockContract, conffit.BlockApplication, false},
	{33, conffit.BlockContract, conffit.BlockApp, false},
	{34, conffit.BlockContract, conffit.BlockPort, false},
	{35, conffit.BlockContract, conffit.BlockProvider, false},
	{36, conffit.BlockContract, conffit.BlockContract, true},
}

const (
	cellModule = "exemplo/cell"
	cellSource = "exemplo/cell/source"
	cellTarget = "exemplo/cell/target"
	cellFile   = "source/source.go"
)

// NegativeCode is what the cell's negative universe must emit: D001 for a
// forbidden pair, D002 for an allowed pair crossed between contexts without a
// public surface. Empty for an allowed pair whose target is contract — a
// contract is public by construction (decide.go), so C2 cannot fail and the
// second side of the pair is V20's positive instead.
func (c Cell) NegativeCode() conffit.Code {
	switch {
	case !c.Allowed:
		return conffit.CodeD001
	case c.Target == conffit.BlockContract:
		return ""
	default:
		return conffit.CodeD002
	}
}

// Positive is the universe the cell must accept: two units in one context,
// with the edge when the pair is allowed and without it when it is not.
func (c Cell) Positive() []conffit.Diagnostic {
	return diagnose(c, "ctx", "ctx", c.Allowed)
}

// Negative is the universe the cell must refuse: the edge present in one
// context for a forbidden pair, across two contexts for an allowed one.
func (c Cell) Negative() []conffit.Diagnostic {
	if c.Allowed {
		return diagnose(c, "ctx-a", "ctx-b", true)
	}
	return diagnose(c, "ctx", "ctx", true)
}

// Verify runs both sides and reports, naming the cell, any side that does not
// behave as the norm says (RAS-06: a negative that passes fails the suite).
func (c Cell) Verify() error {
	if ds := c.Positive(); len(ds) != 0 {
		return fmt.Errorf("cell %d (%s -> %s): positive vector failed: %v", c.N, c.Source, c.Target, ds)
	}
	ds := c.Negative()
	want := c.NegativeCode()
	if want == "" {
		if len(ds) != 0 {
			return fmt.Errorf("cell %d (%s -> %s): a contract is a public surface by construction and still failed: %v", c.N, c.Source, c.Target, ds)
		}
		return nil
	}
	if len(ds) != 1 || ds[0].Code != want || ds[0].CanonicalKey != cellSource || ds[0].Target != cellTarget {
		return fmt.Errorf("cell %d (%s -> %s): negative vector must emit exactly one %s %s -> %s, emitted %v", c.N, c.Source, c.Target, want, cellSource, cellTarget, ds)
	}
	return nil
}

func diagnose(c Cell, sourceCtx, targetCtx string, withEdge bool) []conffit.Diagnostic {
	units := []conffit.Unit{
		{ID: "cell/source", Block: c.Source, BoundedContext: sourceCtx, Include: []string{cellSource}, Module: cellModule},
		{ID: "cell/target", Block: c.Target, BoundedContext: targetCtx, Include: []string{cellTarget}, Module: cellModule},
	}
	mods := []conffit.Module{{Path: cellModule, HasManifest: true, HasProduction: true}}
	g := conffit.Graph{Nodes: []conffit.Package{
		{CanonicalKey: cellSource, Module: cellModule},
		{CanonicalKey: cellTarget, Module: cellModule},
	}}
	if withEdge {
		g.Links = []conffit.Edge{{From: cellSource, To: cellTarget, SourceFile: cellFile}}
	}
	return conffit.Diagnose(units, mods, g)
}
