// Package conformance é bloco `application`: orquestra o caso de uso da
// verificação sobre as portas, sem conhecer driver, arquivo nem processo.
//
// A ordem dos passos é normativa, não conveniência: o manifesto é validado
// ANTES de qualquer aresta ser decidida. Universo derivado de manifesto
// inválido produziria diagnósticos de aresta calculados sobre classificação que
// não vale — ruído que esconde a causa real.
package conformance

import (
	"fmt"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/manifest"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/port"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// Input reúne as portas e o inventário que o caso de uso consome.
type Input struct {
	Modules   []rule.Module
	Manifests port.ManifestSource
	Graph     port.GraphSource
	Closure   rule.Closure
	Standard  func(string) bool
}

// Report é o veredicto: os diagnósticos ordenados e a fase em que a verificação
// parou.
type Report struct {
	Diagnostics []rule.Diagnostic
	PhaseHalted string
}

// Resumo descreve o veredicto em uma linha.
func (r Report) Resumo() string {
	if len(r.Diagnostics) == 0 {
		return "dmpf-conformance: conforme"
	}
	if r.PhaseHalted != "" {
		return fmt.Sprintf("dmpf-conformance: REPROVADO com %d diagnóstico(s); parou em %s",
			len(r.Diagnostics), r.PhaseHalted)
	}
	return fmt.Sprintf("dmpf-conformance: REPROVADO com %d diagnóstico(s)", len(r.Diagnostics))
}

// Check executa a verificação completa.
func Check(in Input) (Report, error) {
	docs, err := in.Manifests.Documents()
	if err != nil {
		return Report{}, err
	}

	// Passo 1-2 — validar os manifestos. Qualquer DMPF-M* aqui ENCERRA: sem
	// classificação válida não há decisão possível, e prosseguir seria adivinhar.
	var diags []rule.Diagnostic
	var units []rule.Unit

	// A allowlist e as exceções são POR MÓDULO, nunca unificadas: cada
	// `dmpf-units.json` declara a política do seu próprio ownership_module.
	// Unir tudo faria a exceção que o módulo A declarou para si autorizar o
	// módulo B — e como o `id` de unidade só é único DENTRO de um manifesto
	// (RFC §10.1), dois módulos podem declarar `id: "domain"` legitimamente.
	// A exceção deixaria de ser nominal e viraria a política paralela que
	// RFC §6.4 proíbe.
	politicaPorModulo := map[string]rule.ExternalPolicy{}
	unidadeDoPackage := map[string]rule.UnitKey{}

	for _, doc := range docs {
		diags = append(diags, manifest.Validate(doc)...)
		p := politicaDoDocumento(doc)
		diags = append(diags, p.Validate(doc.Path)...)
		politicaPorModulo[doc.Module] = p
		units = append(units, unidadesDoDocumento(doc)...)
	}
	if len(diags) > 0 {
		rule.SortDiagnostics(diags)
		return Report{Diagnostics: diags, PhaseHalted: "validação de manifesto"}, nil
	}

	// Passo 3 — construir o universo.
	pkgs, err := in.Graph.Packages()
	if err != nil {
		return Report{}, err
	}
	universo, cobertura := rule.BuildUniverse(units, pkgs, in.Modules)

	// U004 encerra junto com os DMPF-M*: módulo de produção sem manifesto não
	// tem classificação nenhuma, e decidir arestas sobre ele seria adivinhar.
	// U001/U002/U003 NÃO encerram — a spec fixa um único ponto de parada, e
	// acumular deixa o mesmo CI mostrar cobertura e dependência de uma vez.
	var semManifesto []rule.Diagnostic
	for _, d := range cobertura {
		if d.Code == rule.CodeU004 {
			semManifesto = append(semManifesto, d)
		}
	}
	if len(semManifesto) > 0 {
		diags = append(diags, cobertura...)
		rule.SortDiagnostics(diags)
		return Report{Diagnostics: diags, PhaseHalted: "módulo sem manifesto"}, nil
	}
	diags = append(diags, cobertura...)
	for _, u := range units {
		for _, inc := range u.Include {
			unidadeDoPackage[rule.NormalizeInclude(inc)] = u.Key()
		}
	}
	// Passo 4-6 — extrair, particionar e decidir.
	edges, err := in.Graph.Edges()
	if err != nil {
		return Report{}, err
	}
	for _, e := range edges {
		// E003 é avaliado ANTES da classificação da origem: um import que não
		// resolve não depende de bloco nenhum para reprovar, e exigir a
		// classificação primeiro esconderia o erro sempre que a origem também
		// estivesse em U001.
		if e.Unresolved {
			diags = append(diags, rule.Diagnostic{
				Code:         rule.CodeE003,
				CanonicalKey: e.From,
				Target:       e.To,
				SourceFile:   e.SourceFile,
				Detail:       detalheOuPadrao(e.Detail),
			})
			continue
		}

		origem, classificada := universo.Endpoint(e.From)
		if !classificada {
			// Origem descoberta e não classificada já reprovou em U001 ou U002;
			// decidir a aresta exigiria a classificação que falta.
			continue
		}
		if destino, classificado := universo.Endpoint(e.To); classificado {
			diags = append(diags, rule.DiagnoseEdge(origem, destino, e.SourceFile)...)
			continue
		}
		if universo.Discovered(e.To) {
			// Descoberto e não classificado: já reprovou em U001 ou U002.
			// Avaliá-lo como dependência externa emitiria E001 sobre código do
			// próprio universo.
			continue
		}

		dono := unidadeDoPackage[e.From]
		diags = append(diags, rule.EvaluateExternal(
			origem, dono.ID, e.To, e.SourceFile,
			politicaPorModulo[dono.Module], in.Closure, in.Standard,
		)...)
	}

	rule.SortDiagnostics(diags)
	return Report{Diagnostics: diags}, nil
}

func unidadesDoDocumento(doc manifest.Document) []rule.Unit {
	out := make([]rule.Unit, 0, len(doc.Units))
	for _, u := range doc.Units {
		out = append(out, rule.Unit{
			ID:                       u.ID,
			Block:                    rule.Block(u.Block),
			BoundedContext:           u.BoundedContext,
			PublicIntegrationSurface: u.PublicIntegrationSurface,
			Include:                  u.Include,
			Module:                   doc.Module,
			ManifestPath:             doc.Path,
		})
	}
	return out
}

func politicaDoDocumento(doc manifest.Document) rule.ExternalPolicy {
	var p rule.ExternalPolicy
	for _, e := range doc.External {
		p.Allowlist = append(p.Allowlist, rule.AllowlistEntry{
			Package: e.Package, Entrypoints: e.Entrypoints,
			Capability: rule.Capability(e.Capability), Versions: e.Versions,
		})
	}
	for _, x := range doc.Exceptions {
		p.Exceptions = append(p.Exceptions, rule.ExceptionEntry{
			Unit: x.Unit, Dependency: x.Dependency, Reason: x.Reason,
			Owner: x.Owner, ReviewBy: x.ReviewBy,
		})
	}
	return p
}

func detalheOuPadrao(d string) string {
	if d == "" {
		return "import não resolvido pelo toolchain"
	}
	return d
}
