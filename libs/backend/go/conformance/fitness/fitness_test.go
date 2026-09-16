package fitness_test

import (
	"path/filepath"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/fitness"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/fsstore"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/golist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/port"
)

const (
	repoRoot = "../../../../.."
	fixtures = "../internal/golist/testdata"
	modA     = "exemplo/mod-a"
	pkgDom   = "exemplo/mod-a/domain"
	pkgPort  = "exemplo/mod-a/port"
)

var perfilLinuxAmd64 = []fsstore.BuildProfile{
	{ID: "linux-amd64", GOOS: "linux", GOARCH: "amd64", CGOEnabled: false},
}

var _ fitness.GraphSource = fitness.Graph{}

// FIT-03: the suite never judges the authority over the classification — that
// is the gate's job, with the baseline and the review interval it needs.
func TestWorkspaceCarriesNoBaselineAndNoBase(t *testing.T) {
	in, err := fitness.Workspace(repoRoot, "")
	if err != nil {
		t.Fatalf("Workspace: %v", err)
	}
	if in.Baseline != nil {
		t.Error("Workspace set a BaselineStore: the fitness function must not verify authority")
	}
	if in.Base != "" {
		t.Errorf("Workspace set Base %q", in.Base)
	}
	if len(in.Modules) == 0 {
		t.Fatal("Workspace found no module")
	}
	if in.Graph == nil || in.Closure == nil || in.Standard == nil {
		t.Fatal("Workspace left the graph, the closure or the standard-library oracle unset")
	}
}

func TestDiagnosticsReportsD001OverTheRealExtractor(t *testing.T) {
	in := entradaDaFixture(t, "domaincontract", "exemplo.test/dc-a", "exemplo.test/dc-b")
	ds, err := fitness.Diagnostics(in)
	if err != nil {
		t.Fatalf("Diagnostics: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("expected exactly one diagnostic, got %v", ds)
	}
	d := ds[0]
	if d.Code != fitness.CodeD001 || d.CanonicalKey != "exemplo.test/dc-a/domain" || d.Target != "exemplo.test/dc-b/wire" {
		t.Fatalf("diagnostic = %+v, want D001 dc-a/domain -> dc-b/wire", d)
	}
}

// Mirrors TestVetorD001 of internal/rule over the exported surface.
func TestDiagnoseReproducesVectorD001(t *testing.T) {
	units := []fitness.Unit{
		unidade("a/domain", fitness.BlockDomain, "a", pkgDom),
		unidade("a/port", fitness.BlockPort, "a", pkgPort),
	}
	mods := []fitness.Module{{Path: modA, HasManifest: true, HasProduction: true}}
	pkgs := []fitness.Package{{CanonicalKey: pkgDom, Module: modA}, {CanonicalKey: pkgPort, Module: modA}}

	t.Run("positivo: port -> domain (célula 19)", func(t *testing.T) {
		g := fitness.Graph{Nodes: pkgs, Links: []fitness.Edge{{From: pkgPort, To: pkgDom, SourceFile: "port/p.go"}}}
		if ds := fitness.Diagnose(units, mods, g); len(ds) != 0 {
			t.Fatalf("positive vector failed: %v", ds)
		}
	})

	t.Run("negativo: domain -> port (célula 4)", func(t *testing.T) {
		g := fitness.Graph{Nodes: pkgs, Links: []fitness.Edge{{From: pkgDom, To: pkgPort, SourceFile: "domain/d.go"}}}
		ds := fitness.Diagnose(units, mods, g)
		if len(ds) != 1 || ds[0].Code != fitness.CodeD001 || ds[0].CanonicalKey != pkgDom || ds[0].Target != pkgPort || ds[0].SourceFile != "domain/d.go" {
			t.Fatalf("diagnostics = %v, want exactly one D001 domain -> port from domain/d.go", ds)
		}
	})

	t.Run("negativo: import não resolvido emite E003 antes da classificação", func(t *testing.T) {
		g := fitness.Graph{Nodes: pkgs, Links: []fitness.Edge{{From: pkgDom, To: "exemplo/nada", SourceFile: "domain/d.go", Unresolved: true}}}
		ds := fitness.Diagnose(units, mods, g)
		if len(ds) != 1 || ds[0].Code != fitness.CodeE003 {
			t.Fatalf("diagnostics = %v, want exactly one E003", ds)
		}
	})

	t.Run("positivo: destino fora do universo não é decidido pela matriz", func(t *testing.T) {
		g := fitness.Graph{Nodes: pkgs, Links: []fitness.Edge{{From: pkgDom, To: "net/http", SourceFile: "domain/d.go"}}}
		if ds := fitness.Diagnose(units, mods, g); len(ds) != 0 {
			t.Fatalf("external edge was judged by the block matrix: %v", ds)
		}
	})
}

// The edge testkit (kernel, app) -> conformance/fitness
// (conformance, app): C1 holds (app -> app), and C2 holds only because the
// unit declares a public integration surface — without it, D002 (decide.go C2).
func TestFitnessUnitIsReachableAcrossBoundedContexts(t *testing.T) {
	const (
		modKit  = "exemplo/testkit"
		modConf = "exemplo/conformance"
		pkgKit  = "exemplo/testkit/fitness"
		pkgFit  = "exemplo/conformance/fitness"
	)
	mods := []fitness.Module{
		{Path: modKit, HasManifest: true, HasProduction: true},
		{Path: modConf, HasManifest: true, HasProduction: true},
	}
	pkgs := []fitness.Package{{CanonicalKey: pkgKit, Module: modKit}, {CanonicalKey: pkgFit, Module: modConf}}
	kit := fitness.Unit{ID: "kernel/testkit-fitness", Block: fitness.BlockApp, BoundedContext: "kernel", Include: []string{pkgKit}, Module: modKit}
	g := fitness.Graph{Nodes: pkgs, Links: []fitness.Edge{{From: pkgKit, To: pkgFit, SourceFile: "fitness/universe.go"}}}

	t.Run("positivo: unidade fitness com superfície pública", func(t *testing.T) {
		fit := fitness.Unit{ID: "conformance/fitness", Block: fitness.BlockApp, BoundedContext: "conformance", PublicIntegrationSurface: true, Include: []string{pkgFit}, Module: modConf}
		if ds := fitness.Diagnose([]fitness.Unit{kit, fit}, mods, g); len(ds) != 0 {
			t.Fatalf("cross-context edge to a public surface failed: %v", ds)
		}
	})

	t.Run("negativo: sem superfície pública reprova por D002", func(t *testing.T) {
		fit := fitness.Unit{ID: "conformance/fitness", Block: fitness.BlockApp, BoundedContext: "conformance", Include: []string{pkgFit}, Module: modConf}
		ds := fitness.Diagnose([]fitness.Unit{kit, fit}, mods, g)
		if len(ds) != 1 || ds[0].Code != fitness.CodeD002 {
			t.Fatalf("diagnostics = %v, want exactly one D002", ds)
		}
	})
}

func unidade(id string, block fitness.Block, bc string, include ...string) fitness.Unit {
	return fitness.Unit{ID: id, Block: block, BoundedContext: bc, Include: include, Module: modA}
}

// Same hand-built inventory as internal/conformance/integration_test.go: the
// fixture is not a git repository, so Workspace cannot discover it.
func entradaDaFixture(t *testing.T, cenario string, mods ...string) fitness.Input {
	t.Helper()
	var modules []fitness.Module
	for _, m := range mods {
		dir, err := filepath.Abs(filepath.Join(fixtures, cenario, filepath.Base(m)))
		if err != nil {
			t.Fatal(err)
		}
		modules = append(modules, fitness.Module{Path: m, Dir: dir, HasManifest: true, HasProduction: true})
	}
	grafo := golist.New("", modules, perfilLinuxAmd64)
	return fitness.Input{
		Modules:   modules,
		Manifests: fsstore.NewManifestStore(fsstore.ModuleDirs(modules)),
		Graph:     grafo,
		Closure:   grafo.Closure,
		Standard:  grafo.IsStandard,
	}
}

func TestUnitsListsEveryManifestUnit(t *testing.T) {
	in := entradaDaFixture(t, "domaincontract", "exemplo.test/dc-a", "exemplo.test/dc-b")
	units, err := fitness.Units(in)
	if err != nil {
		t.Fatalf("Units: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("Units = %d entries, want 2 (one per fixture manifest): %+v", len(units), units)
	}
	var domain bool
	for _, u := range units {
		if u.ID == "a/domain" && u.Block == fitness.BlockDomain && u.Module == "exemplo.test/dc-a" {
			domain = true
		}
	}
	if !domain {
		t.Fatalf("unit a/domain (domain, exemplo.test/dc-a) missing from %+v", units)
	}
}

type manifestoFixo struct{ docs []manifest.Document }

func (m manifestoFixo) Documents() ([]manifest.Document, error) { return m.docs, nil }

type grafoFixo struct {
	pkgs  []fitness.Package
	edges []fitness.Edge
}

func (g grafoFixo) Packages() ([]fitness.Package, error) { return g.pkgs, nil }
func (g grafoFixo) Edges() ([]fitness.Edge, error)       { return g.edges, nil }

type storeFixo struct {
	doc    baseline.Document
	existe bool
}

func (s storeFixo) Baseline() (baseline.Document, bool, error) { return s.doc, s.existe, nil }
func (s storeFixo) BaselineEm(string) (baseline.Document, bool, error) {
	return baseline.Document{}, false, nil
}
func (s storeFixo) CommitsQueTocaram(string) ([]baseline.Commit, error) { return nil, nil }

var _ port.BaselineStore = storeFixo{}

func documentoFixo(modulo, id, bc, include string) manifest.Document {
	return manifest.Document{
		Path: modulo + "/dmpf-units.json", Module: modulo, Schema: manifest.SchemaID,
		Units: []manifest.Unit{{
			ID: id, Block: string(fitness.BlockDomain), BoundedContext: bc, Include: []string{include},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}},
	}
}

func inputComEdgeEntreAeB() fitness.Input {
	return fitness.Input{
		Modules: []fitness.Module{
			{Path: "a", HasManifest: true, HasProduction: true},
			{Path: "b", HasManifest: true, HasProduction: true},
		},
		Manifests: manifestoFixo{docs: []manifest.Document{
			documentoFixo("a", "a-domain", "a", "a/domain"),
			documentoFixo("b", "b-domain", "b", "b/domain"),
		}},
		Graph: grafoFixo{
			pkgs:  []fitness.Package{{CanonicalKey: "a/domain", Module: "a"}, {CanonicalKey: "b/domain", Module: "b"}},
			edges: []fitness.Edge{{From: "a/domain", To: "b/domain", SourceFile: "a/domain/d.go"}},
		},
	}
}

// Without a baseline, Input.SharedKernelUnits is the only source of the
// designation, and it must be enough to approve the edge.
func TestSharedKernelUnitsAppliedWithoutBaseline(t *testing.T) {
	in := inputComEdgeEntreAeB()
	in.SharedKernelUnits = []string{"b-domain"}

	ds, err := fitness.Diagnostics(in)
	if err != nil {
		t.Fatalf("Diagnostics: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("SharedKernelUnits without a baseline should approve the edge: %v", ds)
	}
}

// With a baseline present, Input.SharedKernelUnits must never leak into the
// decision: only the store's own designation counts (here, none).
func TestSharedKernelUnitsIgnoredWithBaseline(t *testing.T) {
	entries := []baseline.Entry{
		{Unit: "a-domain", Module: "a", Block: "domain", BoundedContext: "a", Membership: []string{"a/domain"}},
		{Unit: "b-domain", Module: "b", Block: "domain", BoundedContext: "b", Membership: []string{"b/domain"}},
	}
	doc := baseline.Document{Schema: baseline.SchemaID, Entries: entries}
	doc.Digest = baseline.DigestOf(doc)

	in := inputComEdgeEntreAeB()
	in.Baseline = storeFixo{doc: doc, existe: true}
	in.SharedKernelUnits = []string{"b-domain"}

	ds, err := fitness.Diagnostics(in)
	if err != nil {
		t.Fatalf("Diagnostics: %v", err)
	}
	if len(ds) != 1 || ds[0].Code != fitness.CodeD002 {
		t.Fatalf("diagnostics = %v, want exactly one D002 (the input list must be ignored when a baseline is present)", ds)
	}
}

func TestStandardCapabilityMirrorsTheChecker(t *testing.T) {
	cases := []struct {
		path string
		want fitness.Capability
		ok   bool
	}{
		{"os", fitness.CapIOFilesystem, true},
		{"time", fitness.CapIOClock, true},
		{"encoding/json", fitness.CapWireCodec, true},
		{"testing", fitness.CapObservability, true},
		{"fmt", fitness.CapPure, true},
		{"exemplo/nada", "", false},
	}
	for _, c := range cases {
		got, ok := fitness.StandardCapability(c.path)
		if ok != c.ok || got != c.want {
			t.Errorf("StandardCapability(%q) = (%q, %v), want (%q, %v)", c.path, got, ok, c.want, c.ok)
		}
	}
}
