package conformance_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/port"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

type grafoComImports []string

func (grafoComImports) Packages() ([]rule.Package, error) {
	return []rule.Package{{CanonicalKey: "m/p", Module: "m"}}, nil
}

func (g grafoComImports) Edges() ([]port.Edge, error) {
	out := make([]port.Edge, 0, len(g))
	for _, imp := range g {
		out = append(out, port.Edge{From: "m/p", To: imp, SourceFile: "m/p/p.go"})
	}
	return out, nil
}

type manifestoComExcecoes struct {
	block      string
	exceptions []manifest.Exception
}

func (m manifestoComExcecoes) Documents() ([]manifest.Document, error) {
	return []manifest.Document{{
		Path: "m/dmpf-units.json", Module: "m", Schema: manifest.SchemaID,
		Units: []manifest.Unit{{
			ID: "u", Block: m.block, BoundedContext: "bc", Include: []string{"m/p"},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}},
		Exceptions: m.exceptions,
	}}, nil
}

func diaUTC(s string) exception.Instant {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return exception.Instant(t.UnixNano())
}

func excecaoE1(identity string) manifest.Exception {
	return manifest.Exception{
		Unit: "u", Dependency: identity, Reason: "emitido pelo protoc-gen-go",
		Owner: "team:tech-leads", ReviewBy: "2027-03-02", ReviewByAt: diaUTC("2027-03-02"),

		ID: "X-u-" + strings.ReplaceAll(identity, "/", "-"),
		Object: manifest.ExceptionObject{
			Kind: "external-dependency", Unit: "u", Identity: identity,
			PresentKind: true, PresentUnit: true, PresentIdentity: true,
		},
		ADR:           "ADR-033",
		Justification: "emitido pelo protoc-gen-go",
		Convergence: manifest.ExceptionConvergence{
			Kind:                "review",
			ReviewBy:            diaUTC("2027-03-02"),
			ApprovedBy:          []string{"arquitetura", "plataforma"},
			ReplanningCondition: "protoc-gen-go deixar de emitir o import",
			PresentKind:         true,
		},
		ValidFrom:  diaUTC("2026-09-12"),
		ValidUntil: diaUTC("2027-03-02"),
		History: []manifest.ExceptionHistoryEntry{
			{Event: "granted", At: diaUTC("2026-09-12"), By: "team:tech-leads"},
		},

		PresentID: true, PresentObject: true, PresentADR: true, PresentJustification: true,
		PresentConvergence: true, PresentValidFrom: true, PresentValidUntil: true, PresentHistory: true,
	}
}

func verificarComExcecoes(t *testing.T, m manifestoComExcecoes, now exception.Instant, imports ...string) conformance.Report {
	t.Helper()
	rel, err := conformance.Check(conformance.Input{
		Modules:   []rule.Module{{Path: "m", HasManifest: true, HasProduction: true}},
		Manifests: m,
		Graph:     grafoComImports(imports),
		Standard: func(p string) bool {
			_, ok := rule.StdlibCapability(p)
			return ok
		},
		Now: now,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return rel
}

func reprovaImport(rel conformance.Report, target string) bool {
	return slices.ContainsFunc(rel.Diagnostics, func(d rule.Diagnostic) bool {
		return d.Code == rule.CodeE001 && d.Target == target
	})
}

func recusas(rel conformance.Report) []rule.Diagnostic {
	var out []rule.Diagnostic
	for _, d := range rel.Diagnostics {
		if strings.HasPrefix(string(d.Code), "DMPF-X") {
			out = append(out, d)
		}
	}
	return out
}

func TestE1AdmitidaAutorizaSoOImportNominal(t *testing.T) {
	m := manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{excecaoE1("reflect")}}
	rel := verificarComExcecoes(t, m, 0, "reflect", "unsafe")

	if x := recusas(rel); len(x) > 0 {
		t.Fatalf("E1 completa recusada: %v", x)
	}
	if reprovaImport(rel, "reflect") {
		t.Errorf("reflect reprovou apesar da E1 admitida: %v", rel.Diagnostics)
	}
	if !reprovaImport(rel, "unsafe") {
		t.Errorf("unsafe passou sem exceção: a E1 de reflect autorizou além do import nominal: %v", rel.Diagnostics)
	}
}

func TestE1SemADRReprovaComoSeNaoExistisse(t *testing.T) {
	x := excecaoE1("reflect")
	x.ADR, x.PresentADR = "", false
	rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{x}}, 0, "reflect")

	if !temCodigo(rel, rule.CodeX001) {
		t.Errorf("E1 sem adr não emitiu X001: %v", rel.Diagnostics)
	}
	if !reprovaImport(rel, "reflect") {
		t.Errorf("E1 recusada ainda autorizou o import: %v", rel.Diagnostics)
	}
}

func TestE1EmDomainRecusadaPorN1(t *testing.T) {
	m := manifestoComExcecoes{block: "domain", exceptions: []manifest.Exception{excecaoE1("net/http")}}
	rel := verificarComExcecoes(t, m, 0, "net/http")

	if !temCodigo(rel, rule.CodeX003) {
		t.Errorf("E1 de io.network em domain não emitiu X003: %v", rel.Diagnostics)
	}
	if !reprovaImport(rel, "net/http") {
		t.Errorf("E1 recusada por N1 autorizou o import: %v", rel.Diagnostics)
	}
}

func TestE1VencidaDeixaDeAutorizar(t *testing.T) {
	x := excecaoE1("reflect")
	x.ReviewBy, x.ReviewByAt = "2026-09-20", diaUTC("2026-09-20")
	x.ValidUntil, x.PresentValidUntil = diaUTC("2026-10-01"), true
	m := manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{x}}

	antes := verificarComExcecoes(t, m, diaUTC("2026-09-30"), "reflect")
	if len(recusas(antes)) > 0 || reprovaImport(antes, "reflect") {
		t.Fatalf("E1 dentro da validade reprovou: %v", antes.Diagnostics)
	}

	depois := verificarComExcecoes(t, m, diaUTC("2026-10-02"), "reflect")
	if !temCodigo(depois, rule.CodeX006) {
		t.Errorf("Input.Now depois de valid_until não emitiu X006: %v", depois.Diagnostics)
	}
	if !reprovaImport(depois, "reflect") {
		t.Errorf("E1 vencida ainda autorizou o import: %v", depois.Diagnostics)
	}
}

func TestFronteiraEntreMEX(t *testing.T) {
	semADR := excecaoE1("reflect")
	semADR.ADR, semADR.PresentADR = "", false

	t.Run("M* encerra a fase", func(t *testing.T) {
		m := manifestoComExcecoes{block: "core", exceptions: []manifest.Exception{semADR}}
		rel := verificarComExcecoes(t, m, 0, "reflect")

		if rel.PhaseHalted != "validação de manifesto" {
			t.Errorf("M002 não encerrou a fase: PhaseHalted=%q", rel.PhaseHalted)
		}
		if reprovaImport(rel, "reflect") {
			t.Errorf("aresta decidida sobre classificação inválida: %v", rel.Diagnostics)
		}
		if !temCodigo(rel, rule.CodeX001) {
			t.Errorf("X001 sumiu do relatório que parou em M*: %v", rel.Diagnostics)
		}
	})

	t.Run("X* não encerra a fase", func(t *testing.T) {
		m := manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{semADR}}
		rel := verificarComExcecoes(t, m, 0, "reflect")

		if rel.PhaseHalted != "" {
			t.Errorf("X001 encerrou a fase em %q", rel.PhaseHalted)
		}
		if !temCodigo(rel, rule.CodeX001) {
			t.Errorf("X001 sumiu do relatório: %v", rel.Diagnostics)
		}
		if !reprovaImport(rel, "reflect") {
			t.Errorf("a verificação parou antes das arestas: %v", rel.Diagnostics)
		}
	})
}

func ehStdlib(p string) bool {
	_, ok := rule.StdlibCapability(p)
	return ok
}

type manifestosDeDoisModulos struct{ includeDeU2 []string }

func (m manifestosDeDoisModulos) Documents() ([]manifest.Document, error) {
	x := excecaoE1("reflect")
	x.Unit, x.Object.Unit, x.ID = "u2", "u2", "X-u2-reflect"
	unidade := func(id, block string, include ...string) manifest.Unit {
		return manifest.Unit{
			ID: id, Block: block, BoundedContext: "bc", Include: include,
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}
	}
	return []manifest.Document{
		{Path: "m1/dmpf-units.json", Module: "m1", Schema: manifest.SchemaID, Units: []manifest.Unit{unidade("u", "domain", "m1/p")}},
		{
			Path: "m2/dmpf-units.json", Module: "m2", Schema: manifest.SchemaID,
			Units:      []manifest.Unit{unidade("u2", "contract", m.includeDeU2...)},
			Exceptions: []manifest.Exception{x},
		},
	}, nil
}

type grafoDeDoisModulos struct{}

func (grafoDeDoisModulos) Packages() ([]rule.Package, error) {
	return []rule.Package{{CanonicalKey: "m1/p", Module: "m1"}, {CanonicalKey: "m2/q", Module: "m2"}}, nil
}

func (grafoDeDoisModulos) Edges() ([]port.Edge, error) {
	return []port.Edge{{From: "m1/p", To: "reflect", SourceFile: "m1/p/p.go"}}, nil
}

func verificarDoisModulos(t *testing.T, includeDeU2 ...string) conformance.Report {
	t.Helper()
	rel, err := conformance.Check(conformance.Input{
		Modules: []rule.Module{
			{Path: "m1", HasManifest: true, HasProduction: true},
			{Path: "m2", HasManifest: true, HasProduction: true},
		},
		Manifests: manifestosDeDoisModulos{includeDeU2: includeDeU2},
		Graph:     grafoDeDoisModulos{},
		Standard:  ehStdlib,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return rel
}

func TestIncludeDeOutroModuloReprovaEmM002(t *testing.T) {
	rel := verificarDoisModulos(t, "m2/q", "m1/p")
	if !temCodigo(rel, rule.CodeM002) || rel.PhaseHalted != "validação de manifesto" {
		t.Fatalf("include de outro módulo não reprovou em M002: PhaseHalted=%q %v", rel.PhaseHalted, rel.Diagnostics)
	}
}

func TestE1DeOutroModuloNaoAutorizaOImport(t *testing.T) {
	rel := verificarDoisModulos(t, "m2/q")
	if !reprovaImport(rel, "reflect") {
		t.Fatalf("a E1 de m2 autorizou reflect na unidade domain de m1: %v", rel.Diagnostics)
	}
}

func TestE1EncerradaNaoAutorizaNemVence(t *testing.T) {
	x := excecaoE1("reflect")
	x.History = append(x.History, manifest.ExceptionHistoryEntry{Event: "converged", At: diaUTC("2026-10-01"), By: "team:tech-leads"})
	rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{x}}, diaUTC("2028-01-01"), "reflect")

	if r := recusas(rel); len(r) > 0 {
		t.Fatalf("exceção encerrada recusada: %v", r)
	}
	if !reprovaImport(rel, "reflect") {
		t.Fatalf("exceção encerrada ainda autorizou o import: %v", rel.Diagnostics)
	}
}

func TestE1ParaUnidadeForaDoManifestoRecusadaPorX007(t *testing.T) {
	x := excecaoE1("reflect")
	x.Unit, x.Object.Unit = "fantasma", "fantasma"
	rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{x}}, 0, "reflect")
	if !temCodigo(rel, rule.CodeX007) || !reprovaImport(rel, "reflect") {
		t.Fatalf("E1 de unidade inexistente sem X007 ou autorizando: %v", rel.Diagnostics)
	}
}

func TestDivergenciaEntreLegadoENovoRecusaPorX001(t *testing.T) {
	x := excecaoE1("reflect")
	x.Dependency = "unsafe"
	rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{x}}, 0, "reflect", "unsafe")
	if !temCodigo(rel, rule.CodeX001) || !reprovaImport(rel, "reflect") || !reprovaImport(rel, "unsafe") {
		t.Fatalf("divergência entre legado e novo sem X001 ou autorizando: %v", rel.Diagnostics)
	}
}

func TestIDRepetidoRecusaPorX001(t *testing.T) {
	a, b := excecaoE1("reflect"), excecaoE1("unsafe")
	b.ID = a.ID
	rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{a, b}}, 0, "reflect", "unsafe")
	if !temCodigo(rel, rule.CodeX001) {
		t.Fatalf("id repetido sem X001: %v", rel.Diagnostics)
	}
}

func TestE1IncompletaRecusadaSemEncerrarAFase(t *testing.T) {
	casos := map[string]func(*manifest.Exception){
		"sem valid_until": func(x *manifest.Exception) { x.ValidUntil, x.PresentValidUntil = 0, false },
		"sem review_by":   func(x *manifest.Exception) { x.ReviewBy, x.ReviewByAt = "", 0 },
		"review_by inválido": func(x *manifest.Exception) {
			x.ReviewBy, x.ReviewByAt, x.InvalidDates = "amanhã", 0, []string{"review_by"}
		},
		"sem object.unit": func(x *manifest.Exception) { x.Unit, x.Object.Unit, x.Object.PresentUnit = "", "", false },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			x := excecaoE1("reflect")
			estraga(&x)
			rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{x}}, 0, "reflect")
			if rel.PhaseHalted != "" || len(recusas(rel)) == 0 || !reprovaImport(rel, "reflect") {
				t.Fatalf("PhaseHalted=%q, recusas=%v, E001=%v", rel.PhaseHalted, recusas(rel), reprovaImport(rel, "reflect"))
			}
		})
	}
}

func TestNowAusenteDeclaraVencimentoNaoVerificado(t *testing.T) {
	rel := verificarComExcecoes(t, manifestoComExcecoes{block: "contract", exceptions: []manifest.Exception{excecaoE1("reflect")}}, 0, "reflect")
	if !slices.ContainsFunc(rel.NaoVerificado, func(s string) bool { return strings.Contains(s, "Input.Now ausente") }) {
		t.Fatalf("vencimento sem Now não declarado: %v", rel.NaoVerificado)
	}
}
