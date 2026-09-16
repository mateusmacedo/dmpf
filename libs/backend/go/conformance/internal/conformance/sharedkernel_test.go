package conformance_test

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/port"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

// storeSK, ao contrário de storeFalso, permite simular erro/ausência em
// Baseline() e conta as chamadas — é o que prova a leitura única de 2.1.
type storeSK struct {
	doc      baseline.Document
	existe   bool
	err      error
	chamadas *int

	antes   baseline.Document
	tinha   bool
	commits []baseline.Commit
}

func (s storeSK) Baseline() (baseline.Document, bool, error) {
	if s.chamadas != nil {
		*s.chamadas++
	}
	return s.doc, s.existe, s.err
}

func (s storeSK) BaselineEm(string) (baseline.Document, bool, error) {
	return s.antes, s.tinha, nil
}

func (s storeSK) CommitsQueTocaram(string) ([]baseline.Commit, error) { return s.commits, nil }

var _ port.BaselineStore = storeSK{}

type unidadeManifesto struct {
	id, block, bc, include string
}

func documentoDoModulo(modulo string, unidades ...unidadeManifesto) manifest.Document {
	units := make([]manifest.Unit, 0, len(unidades))
	for _, u := range unidades {
		units = append(units, manifest.Unit{
			ID: u.id, Block: u.block, BoundedContext: u.bc, Include: []string{u.include},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		})
	}
	return manifest.Document{Path: modulo + "/dmpf-units.json", Module: modulo, Schema: manifest.SchemaID, Units: units}
}

type manifestoModulos struct{ docs []manifest.Document }

func (m manifestoModulos) Documents() ([]manifest.Document, error) { return m.docs, nil }

type grafoSK struct {
	pkgs  []rule.Package
	edges []port.Edge
}

func (g grafoSK) Packages() ([]rule.Package, error) { return g.pkgs, nil }
func (g grafoSK) Edges() ([]port.Edge, error)       { return g.edges, nil }

// entradaSK monta três bounded contexts (x, kernel, y): kernel tem duas
// unidades, e só uma delas ("kernel-domain") é designável nos cenários.
func entradaSK(edges []port.Edge) conformance.Input {
	docs := []manifest.Document{
		documentoDoModulo("x", unidadeManifesto{id: "x-domain", block: "domain", bc: "x", include: "x/domain"}),
		documentoDoModulo("kernel",
			unidadeManifesto{id: "kernel-domain", block: "domain", bc: "kernel", include: "kernel/domain"},
			unidadeManifesto{id: "kernel-example", block: "domain", bc: "kernel", include: "kernel/example"},
		),
		documentoDoModulo("y", unidadeManifesto{id: "y-domain", block: "domain", bc: "y", include: "y/domain"}),
	}
	pkgs := []rule.Package{
		{CanonicalKey: "x/domain", Module: "x"},
		{CanonicalKey: "kernel/domain", Module: "kernel"},
		{CanonicalKey: "kernel/example", Module: "kernel"},
		{CanonicalKey: "y/domain", Module: "y"},
	}
	return conformance.Input{
		Modules: []rule.Module{
			{Path: "x", HasManifest: true, HasProduction: true},
			{Path: "kernel", HasManifest: true, HasProduction: true},
			{Path: "y", HasManifest: true, HasProduction: true},
		},
		Manifests: manifestoModulos{docs: docs},
		Graph:     grafoSK{pkgs: pkgs, edges: edges},
	}
}

// docSK deriva um baseline coerente com o universo de entradaSK, com o
// digest fechado — as entradas batem exatamente com o que o manifesto produz.
func docSK(chaves ...string) baseline.Document {
	entries := []baseline.Entry{
		{Unit: "x-domain", Module: "x", Block: "domain", BoundedContext: "x", Membership: []string{"x/domain"}},
		{Unit: "kernel-domain", Module: "kernel", Block: "domain", BoundedContext: "kernel", Membership: []string{"kernel/domain"}},
		{Unit: "kernel-example", Module: "kernel", Block: "domain", BoundedContext: "kernel", Membership: []string{"kernel/example"}},
		{Unit: "y-domain", Module: "y", Block: "domain", BoundedContext: "y", Membership: []string{"y/domain"}},
	}
	d := baseline.Document{Schema: baseline.SchemaID, Entries: entries}
	if len(chaves) > 0 {
		d.SharedKernelUnits = chaves
		d.HasSharedKernelUnits = true
	}
	d.Digest = baseline.DigestOf(d)
	return d
}

func TestSharedKernelDesignadaAprovaAEdgeParaDentro(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "x/domain", To: "kernel/domain", SourceFile: "x/domain/d.go"}})
	in.Baseline = storeSK{doc: docSK("kernel-domain"), existe: true, antes: docSK("kernel-domain"), tinha: true}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rel.Diagnostics) != 0 {
		t.Fatalf("edge para shared kernel designado reprovou: %v", rel.Diagnostics)
	}
}

func TestSharedKernelNaoDesignadaReprovaAEdge(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "x/domain", To: "kernel/domain", SourceFile: "x/domain/d.go"}})
	in.Baseline = storeSK{doc: docSK(), existe: true, antes: docSK(), tinha: true}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeD002) {
		t.Fatalf("edge sem shared kernel designado deveria reprovar por D002: %v", rel.Diagnostics)
	}
}

func TestSharedKernelNaoAlcancaUnidadeIrmaNaoDesignada(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "x/domain", To: "kernel/example", SourceFile: "x/domain/d.go"}})
	in.Baseline = storeSK{doc: docSK("kernel-domain"), existe: true, antes: docSK("kernel-domain"), tinha: true}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeD002) {
		t.Fatalf("shared kernel de uma unidade não deveria alcançar a unidade irmã: %v", rel.Diagnostics)
	}
}

func TestSemSharedKernelEdgeEntreContextosReprova(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "x/domain", To: "y/domain", SourceFile: "x/domain/d.go"}})
	in.Baseline = storeSK{doc: docSK(), existe: true, antes: docSK(), tinha: true}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeD002) {
		t.Fatalf("edge entre contextos sem shared kernel deveria reprovar por D002: %v", rel.Diagnostics)
	}
}

// 5. kernel-domain designado como ORIGEM não libera a edge: o corte de
// Decide olha o destino, então a exceção não relaxa na direção inversa.
func TestSharedKernelEUnidirecional(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "kernel/domain", To: "x/domain", SourceFile: "kernel/domain/d.go"}})
	in.Baseline = storeSK{doc: docSK("kernel-domain"), existe: true, antes: docSK("kernel-domain"), tinha: true}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeD002) {
		t.Fatalf("shared kernel como origem não deveria liberar a edge: %v", rel.Diagnostics)
	}
}

func TestSharedKernelChaveInexistenteEmiteM004EHaltaFase(t *testing.T) {
	in := entradaSK(nil)
	in.Baseline = storeSK{doc: docSK("kernel-fantasma"), existe: true}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rel.Diagnostics) != 1 || rel.Diagnostics[0].Code != rule.CodeM004 {
		t.Fatalf("esperado exatamente 1 DMPF-M004, got %v", rel.Diagnostics)
	}
	if rel.PhaseHalted != "designação de shared kernel" {
		t.Errorf("PhaseHalted = %q, esperado %q", rel.PhaseHalted, "designação de shared kernel")
	}
}

// Baseline ilegível halta a designação antes do universo: decidir aresta
// sobre designação que não vale produziria D002 em massa (godoc de check.go).
func TestSharedKernelBaselineIlegivelHaltaAntesDoUniverso(t *testing.T) {
	in := entradaSK(nil)
	in.Baseline = storeSK{err: errors.New("ref inalcançável")}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rel.NaoVerificado) == 0 || !rel.Reprovado() {
		t.Fatalf("baseline ilegível não reprovou: %+v", rel)
	}
	if rel.PhaseHalted != "designação de shared kernel" {
		t.Errorf("PhaseHalted = %q, esperado %q", rel.PhaseHalted, "designação de shared kernel")
	}
}

func TestSharedKernelBaselineLegadoSemAChaveNaoEmiteM004(t *testing.T) {
	in := entradaSK(nil)
	in.Baseline = storeSK{doc: docSK(), existe: true, antes: docSK(), tinha: true}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if temCodigo(rel, rule.CodeM004) {
		t.Fatalf("baseline legado sem shared_kernel_units não deveria emitir M004: %v", rel.Diagnostics)
	}
	if rel.PhaseHalted != "" {
		t.Errorf("baseline legado não deveria haltar: %q", rel.PhaseHalted)
	}
}

func TestSharedKernelChaveComDigestAntigoEmiteT001(t *testing.T) {
	desatualizado := docSK("kernel-domain")
	desatualizado.Digest = baseline.Digest(desatualizado.Entries)

	in := entradaSK(nil)
	in.Baseline = storeSK{doc: desatualizado, existe: true}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeT001) {
		t.Fatalf("digest desatualizado não reprovou por T001: %v", rel.Diagnostics)
	}
}

func TestSharedKernelSoAListaMudandoComCodigoNoMesmoCommitExigeAval(t *testing.T) {
	in := entradaSK(nil)
	in.Baseline = storeSK{
		doc: docSK("kernel-domain"), existe: true,
		antes: docSK(), tinha: true,
		commits: []baseline.Commit{{SHA: "aaa", Arquivos: []string{baseline.Path, "kernel/domain/d.go"}}},
	}
	in.Base = "origin/develop"

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeT002) {
		t.Fatalf("designar shared kernel misturado com código não exigiu aval: %v", rel.Diagnostics)
	}
}

func TestSharedKernelUnitsDaInputSemStoreAprova(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "x/domain", To: "kernel/domain", SourceFile: "x/domain/d.go"}})
	in.SharedKernelUnits = []string{"kernel-domain"}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if temCodigo(rel, rule.CodeD002) {
		t.Fatalf("Input.SharedKernelUnits sem store deveria aprovar a edge: %v", rel.Diagnostics)
	}
}

func TestSharedKernelUnitsDaInputComStoreEIgnorada(t *testing.T) {
	in := entradaSK([]port.Edge{{From: "x/domain", To: "kernel/domain", SourceFile: "x/domain/d.go"}})
	in.Baseline = storeSK{doc: docSK(), existe: true, antes: docSK(), tinha: true}
	in.Base = "origin/develop"
	in.SharedKernelUnits = []string{"kernel-domain"}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !temCodigo(rel, rule.CodeD002) {
		t.Fatalf("Input.SharedKernelUnits com store presente deveria ser ignorada: %v", rel.Diagnostics)
	}
}

// Veículo de mutação de 2.1: reintroduzir uma segunda leitura do baseline em
// conferirBaseline faz esta contagem passar de 1.
func TestSharedKernelBaselineLidoUmaUnicaVezPorCheck(t *testing.T) {
	var chamadas int
	in := entradaSK([]port.Edge{{From: "x/domain", To: "kernel/domain", SourceFile: "x/domain/d.go"}})
	in.Baseline = storeSK{
		doc: docSK("kernel-domain"), existe: true,
		antes: docSK("kernel-domain"), tinha: true,
		chamadas: &chamadas,
	}
	in.Base = "origin/develop"

	if _, err := conformance.Check(in); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if chamadas != 1 {
		t.Fatalf("Baseline() chamado %d vez(es), esperado exatamente 1", chamadas)
	}
}
