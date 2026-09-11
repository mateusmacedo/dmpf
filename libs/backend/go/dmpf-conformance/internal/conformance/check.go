// Package conformance orquestra o caso de uso sobre as portas.
//
// A ordem dos passos é normativa: o manifesto é validado ANTES de qualquer
// aresta. Decidir aresta sobre classificação que não vale produz ruído que
// esconde a causa real.
package conformance

import (
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/port"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

type Input struct {
	Modules   []rule.Module
	Manifests port.ManifestSource
	Graph     port.GraphSource
	Closure   rule.Closure
	Standard  func(string) bool

	// Opcional, mas a ausência é DECLARADA: pular a conferência da autoridade
	// sobre a classificação e dizer "conforme" mentiria por omissão.
	Baseline port.BaselineStore

	// Só vale quando Baseline é nil: havendo store, ele é a única fonte
	// autoritativa sobre shared kernel, e esta lista é ignorada.
	SharedKernelUnits []string

	// Base delimita o intervalo em revisão, para julgar se a mudança de
	// classificação veio isolada do código.
	Base string
}

type Report struct {
	Diagnostics []rule.Diagnostic
	PhaseHalted string

	// Condição não avaliada nunca vira conforme: entrada aqui reprova.
	NaoVerificado []string
}

func (r Report) Reprovado() bool {
	return len(r.Diagnostics) > 0 || len(r.NaoVerificado) > 0
}

func (r Report) Resumo() string {
	if len(r.NaoVerificado) > 0 {
		return fmt.Sprintf("dmpf-conformance: REPROVADO com %d diagnóstico(s) e %d condição(oes) não verificada(s)",
			len(r.Diagnostics), len(r.NaoVerificado))
	}
	if len(r.Diagnostics) == 0 {
		return "dmpf-conformance: conforme"
	}
	if r.PhaseHalted != "" {
		return fmt.Sprintf("dmpf-conformance: REPROVADO com %d diagnóstico(s); parou em %s",
			len(r.Diagnostics), r.PhaseHalted)
	}
	return fmt.Sprintf("dmpf-conformance: REPROVADO com %d diagnóstico(s)", len(r.Diagnostics))
}

func Check(in Input) (Report, error) {
	docs, err := in.Manifests.Documents()
	if err != nil {
		return Report{}, err
	}

	// Qualquer DMPF-M* ENCERRA: sem classificação válida, prosseguir é adivinhar.
	var diags []rule.Diagnostic
	var units []rule.Unit

	// Política POR MÓDULO, nunca unificada: como o `id` só é único dentro de um
	// manifesto, unir faria a exceção de um módulo autorizar outro — deixaria de
	// nomear o par que autoriza, virando política paralela não revisada.
	politicaPorModulo := map[string]rule.ExternalPolicy{}
	unidadeDoPackage := map[string]rule.UnitKey{}

	for _, doc := range docs {
		diags = append(diags, manifest.Validate(doc)...)
		p := politicaDoDocumento(doc)
		diags = append(diags, p.Validate(doc.Path)...)
		politicaPorModulo[doc.Module] = p
		units = append(units, UnidadesDoDocumento(doc)...)
	}
	if len(diags) > 0 {
		rule.SortDiagnostics(diags)
		return Report{Diagnostics: diags, PhaseHalted: "validação de manifesto"}, nil
	}

	// Shared kernel é decidido ANTES do universo: decidir aresta sobre
	// designação que não vale produz D002 em massa e esconde a causa real.
	var baselineDoc baseline.Document
	var baselineExiste bool
	if in.Baseline != nil {
		var err error
		baselineDoc, baselineExiste, err = in.Baseline.Baseline()
		if err != nil {
			return Report{
				NaoVerificado: []string{"baseline ilegível (" + baseline.Path + "): " + err.Error()},
				PhaseHalted:   "designação de shared kernel",
			}, nil
		}
	}
	if in.Baseline == nil {
		baselineDoc = documentoSinteticoDeUnits(units, in.SharedKernelUnits)
	}
	units, m004 := baseline.Designar(baselineDoc, units)
	if len(m004) > 0 {
		rule.SortDiagnostics(m004)
		return Report{Diagnostics: m004, PhaseHalted: "designação de shared kernel"}, nil
	}

	pkgs, err := in.Graph.Packages()
	if err != nil {
		return Report{}, err
	}
	universo, cobertura := rule.BuildUniverse(units, pkgs, in.Modules)

	// U004 encerra junto com os M*: módulo sem manifesto não tem classificação
	// nenhuma. U001/U002/U003 acumulam, para o mesmo CI mostrar cobertura e
	// dependência de uma vez.
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
	edges, err := in.Graph.Edges()
	if err != nil {
		return Report{}, err
	}
	for _, e := range edges {
		// E003 vem ANTES da classificação: import não resolvido não depende de
		// bloco para reprovar, e a ordem inversa o esconderia quando a origem
		// também estivesse em U001.
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
			// Já reprovou em U001 ou U002; decidir exigiria a classificação que falta.
			continue
		}
		if destino, classificado := universo.Endpoint(e.To); classificado {
			diags = append(diags, rule.DiagnoseEdge(origem, destino, e.SourceFile)...)
			continue
		}
		if universo.Discovered(e.To) {
			// Avaliar como externo emitiria E001 sobre código do próprio universo.
			continue
		}

		dono := unidadeDoPackage[e.From]
		diags = append(diags, rule.EvaluateExternal(
			origem, dono.ID, e.To, e.SourceFile,
			politicaPorModulo[dono.Module], in.Closure, in.Standard,
		)...)
	}

	relatorio := Report{Diagnostics: diags}
	conferirBaseline(in, units, universo, baselineDoc, baselineExiste, &relatorio)

	rule.SortDiagnostics(relatorio.Diagnostics)
	return relatorio, nil
}

// versionado e existeBaseline vêm da leitura única feita em Check: reler aqui
// duplicaria o acesso ao store para o mesmo baseline dentro da mesma chamada.
func conferirBaseline(in Input, units []rule.Unit, universo *rule.Universe, versionado baseline.Document, existeBaseline bool, rel *Report) {
	if in.Baseline == nil {
		rel.NaoVerificado = append(rel.NaoVerificado,
			"autoridade sobre a classificação (RFC §10.2, T1-T6): nenhum BaselineStore fornecido")
		return
	}

	derivado := baseline.FromUniverse(units, universo.Membership())
	if !existeBaseline {
		rel.NaoVerificado = append(rel.NaoVerificado,
			"baseline ausente em "+baseline.Path+": a divergência de T3 não pode ser avaliada")
		return
	}

	rel.Diagnostics = append(rel.Diagnostics, baseline.Compare(versionado, derivado)...)

	// A comparação é entre o baseline de ANTES e o de agora, ao longo do
	// intervalo em revisão. Confrontar baseline e manifesto no mesmo ponto só
	// acha quem esqueceu de atualizar um dos dois; quem altera os dois de forma
	// coerente deixa a comparação verde, e é exatamente esse o caso que a
	// exigência de aval existe para pegar.
	if in.Base == "" {
		rel.NaoVerificado = append(rel.NaoVerificado,
			"sem base para ler o intervalo em revisão: mudança de classificação não pode ser avaliada")
		return
	}
	anterior, tinha, err := in.Baseline.BaselineEm(in.Base)
	if err != nil {
		rel.NaoVerificado = append(rel.NaoVerificado,
			"baseline anterior ilegível: "+err.Error())
		return
	}
	if !tinha {
		// Sem baseline no ponto de partida, tudo o que existe agora é criação
		// de unidade — e criar unidade também exige aval.
		anterior = baseline.Document{Schema: baseline.SchemaID}
	}

	mudancas := baseline.Detectar(anterior, versionado)
	if len(mudancas) == 0 {
		return
	}
	commits, err := in.Baseline.CommitsQueTocaram(in.Base)
	if err != nil {
		rel.NaoVerificado = append(rel.NaoVerificado,
			"mudança de classificação ("+baseline.Descrever(mudancas)+") e histórico ilegível: "+err.Error())
		return
	}
	rel.Diagnostics = append(rel.Diagnostics, baseline.VerificarAutorizacao(mudancas, commits)...)
}

// Exportada porque a regravação do baseline precisa da MESMA projeção que a
// verificação: derivá-la duas vezes deixaria o baseline descrever algo que o
// gate não confere.
func UnidadesDoDocumento(doc manifest.Document) []rule.Unit {
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

// Mesma trilha do baseline real: só assim M004 (chave inexistente/ambígua)
// vale igual nos dois caminhos, sem duas engines de designação divergindo.
func documentoSinteticoDeUnits(units []rule.Unit, chaves []string) baseline.Document {
	entries := make([]baseline.Entry, 0, len(units))
	for _, u := range units {
		entries = append(entries, baseline.Entry{Unit: u.ID, Module: u.Module})
	}
	return baseline.Document{
		Entries:              entries,
		SharedKernelUnits:    chaves,
		HasSharedKernelUnits: true,
	}
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
