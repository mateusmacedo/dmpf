package conformance_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/port"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
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
		ValidFrom: diaUTC("2026-09-12"),
		History: []manifest.ExceptionHistoryEntry{
			{Event: "granted", At: diaUTC("2026-09-12"), By: "team:tech-leads"},
		},

		PresentID: true, PresentObject: true, PresentADR: true, PresentJustification: true,
		PresentConvergence: true, PresentValidFrom: true, PresentHistory: true,
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
