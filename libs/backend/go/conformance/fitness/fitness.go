// comment-discipline-ok-file: superfície pública do verificador; cada godoc cita a regra de FND-09 (FIT-01..FIT-04) ou de RFC §10.2 que justifica o que é exposto e o que é omitido, dentro do limite de 3 linhas.

// Package fitness is the public surface of the checker for a test suite
// (FIT-01..FIT-04): the inventory, the graph, the decision over an edge and
// the diagnostic codes — and nothing of the trust model (FIT-03): no baseline,
// no review interval, no authorization of a classification change. App block,
// because it composes rule (domain), conformance (application) and
// fsstore/golist (provider), which only the app row of the matrix reaches.
package fitness

import (
	"fmt"
	"path/filepath"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/fsstore"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/golist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/port"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

type (
	// Input.SharedKernelUnits is the caller's own designation, used only when
	// Input.Baseline is nil (FIT-03): the fitness function never reads a
	// baseline itself.
	Input       = conformance.Input
	Diagnostic  = rule.Diagnostic
	Code        = rule.Code
	Unit        = rule.Unit
	Module      = rule.Module
	Package     = rule.Package
	Edge        = port.Edge
	Block       = rule.Block
	Endpoint    = rule.Endpoint
	GraphSource = port.GraphSource
	Capability  = rule.Capability
)

const (
	CodeU001 = rule.CodeU001
	CodeU002 = rule.CodeU002
	CodeU003 = rule.CodeU003
	CodeU004 = rule.CodeU004
	CodeM001 = rule.CodeM001
	CodeM002 = rule.CodeM002
	CodeM003 = rule.CodeM003
	CodeM004 = rule.CodeM004
	CodeT001 = rule.CodeT001
	CodeT002 = rule.CodeT002
	CodeD001 = rule.CodeD001
	CodeD002 = rule.CodeD002
	CodeE001 = rule.CodeE001
	CodeE002 = rule.CodeE002
	CodeE003 = rule.CodeE003
	CodeE004 = rule.CodeE004
)

const (
	CapPure             = rule.CapPure
	CapIOStorage        = rule.CapIOStorage
	CapIOMessaging      = rule.CapIOMessaging
	CapIONetwork        = rule.CapIONetwork
	CapIOFilesystem     = rule.CapIOFilesystem
	CapIOClock          = rule.CapIOClock
	CapIORandom         = rule.CapIORandom
	CapRuntimeFramework = rule.CapRuntimeFramework
	CapWireCodec        = rule.CapWireCodec
	CapObservability    = rule.CapObservability
)

const (
	BlockDomain      = rule.BlockDomain
	BlockApplication = rule.BlockApplication
	BlockApp         = rule.BlockApp
	BlockPort        = rule.BlockPort
	BlockProvider    = rule.BlockProvider
	BlockContract    = rule.BlockContract
)

// Workspace is the gate's own composition (cmd/conformance) minus the
// BaselineStore and the base ref: the suite asserts over the production
// universe as it is, never over who authorized its classification (FIT-03).
// An empty profilesPath resolves to the checker's build-profiles.json.
func Workspace(root, profilesPath string) (Input, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Input{}, err
	}
	if profilesPath == "" {
		profilesPath = filepath.Join(abs, "libs", "backend", "go", "conformance", "build-profiles.json")
	}
	profiles, err := fsstore.LoadBuildProfiles(profilesPath)
	if err != nil {
		return Input{}, err
	}
	modules, err := fsstore.NewInventory(abs).Modules()
	if err != nil {
		return Input{}, err
	}
	if len(modules) == 0 {
		return Input{}, fmt.Errorf("nenhum módulo Go encontrado a partir de %s", abs)
	}
	grafo := golist.New(abs, modules, profiles)
	return Input{
		Modules:   modules,
		Manifests: fsstore.NewManifestStore(fsstore.ModuleDirs(modules)),
		Graph:     grafo,
		Closure:   grafo.Closure,
		Standard:  grafo.IsStandard,
	}, nil
}

// Units lists what every manifest of the input declares, in manifest order,
// so a suite can walk the classified universe (which unit is domain, which
// packages it captures) without reading dmpf-units.json a second time.
func Units(in Input) ([]Unit, error) {
	docs, err := in.Manifests.Documents()
	if err != nil {
		return nil, err
	}
	var units []Unit
	for _, doc := range docs {
		units = append(units, conformance.UnidadesDoDocumento(doc)...)
	}
	return units, nil
}

// StandardCapability is the checker's own classification of a standard-library
// package (rule/stdlib.go); false for a package it has no entry for.
func StandardCapability(importPath string) (Capability, bool) {
	return rule.StdlibCapability(importPath)
}

// Diagnostics runs the full check — manifests, coverage, edges, externals —
// and returns its diagnostics. The "not verified" conditions the gate reports
// for a missing baseline are dropped on purpose: here their absence is the
// contract, not an omission (FIT-03).
func Diagnostics(in Input) ([]Diagnostic, error) {
	rel, err := conformance.Check(in)
	if err != nil {
		return nil, err
	}
	return rel.Diagnostics, nil
}

// Graph is an in-memory GraphSource for synthetic universes (the 36 cells of
// RFC §7.4). The field names differ from the interface's methods only because
// Go forbids a field and a method to share a name.
type Graph struct {
	Nodes []Package
	Links []Edge
}

func (g Graph) Packages() ([]Package, error) { return g.Nodes, nil }
func (g Graph) Edges() ([]Edge, error)       { return g.Links, nil }

// Diagnose decides the coverage and every internal edge of a universe by the
// block matrix and the bounded-context rule (DMPF-U*, D001, D002, E003). An
// edge whose target is outside the universe is an external dependency, judged
// by capability rather than by the matrix, and is left to Diagnostics.
func Diagnose(units []Unit, modules []Module, g GraphSource) []Diagnostic {
	pkgs, err := g.Packages()
	if err != nil {
		return []Diagnostic{{Code: CodeE003, Detail: err.Error()}}
	}
	universe, diags := rule.BuildUniverse(units, pkgs, modules)

	edges, err := g.Edges()
	if err != nil {
		return append(diags, Diagnostic{Code: CodeE003, Detail: err.Error()})
	}
	for _, e := range edges {
		if e.Unresolved {
			diags = append(diags, Diagnostic{
				Code:         CodeE003,
				CanonicalKey: e.From,
				Target:       e.To,
				SourceFile:   e.SourceFile,
				Detail:       detalheOuPadrao(e.Detail),
			})
			continue
		}
		src, okS := universe.Endpoint(e.From)
		tgt, okT := universe.Endpoint(e.To)
		if !okS || !okT {
			continue
		}
		diags = append(diags, rule.DiagnoseEdge(src, tgt, e.SourceFile)...)
	}
	rule.SortDiagnostics(diags)
	return diags
}

func detalheOuPadrao(d string) string {
	if d == "" {
		return "import não resolvido pelo toolchain"
	}
	return d
}
