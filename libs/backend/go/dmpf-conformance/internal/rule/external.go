package rule

import (
	"fmt"
	"sort"
	"strings"
)

// ExternalPolicy reúne o que o manifesto declara sobre dependência externa: a
// allowlist e as exceções nominais.
type ExternalPolicy struct {
	Allowlist  []AllowlistEntry
	Exceptions []ExceptionEntry
}

// Closure devolve o fechamento transitivo de um import path. É fornecido pelo
// provider (o campo Deps do `go list`) e serve APENAS à pureza transitiva —
// nunca à decisão de aresta, que é sempre sobre a aresta direta.
type Closure func(importPath string) ([]string, bool)

// lookup resolve a entrada da allowlist que cobre o import path.
func (p ExternalPolicy) lookup(importPath string) (AllowlistEntry, bool) {
	for _, e := range p.Allowlist {
		if e.Complete() && e.Covers(importPath) {
			return e, true
		}
	}
	return AllowlistEntry{}, false
}

// exceptionFor resolve a exceção nominal para o par (unidade, dependência).
// Exceção inválida não autoriza: a forma incompleta é recusada em silêncio aqui
// e reportada por Validate.
func (p ExternalPolicy) exceptionFor(unitID, importPath string) bool {
	for _, x := range p.Exceptions {
		if x.Unit == unitID && x.Dependency == importPath && x.Valid() {
			return true
		}
	}
	return false
}

// Validate reprova entrada de allowlist incompleta (RFC §6.3) e exceção que não
// seja nominal (RFC §6.4).
//
// Sem os quatro elementos, a entrada da allowlist não descreve o que autoriza;
// sem os cinco, a exceção deixa de ser nominal e vira política paralela não
// revisada. Nos dois casos a declaração incompleta não autoriza nada, e deixá-la
// passar batida seria autorizar por omissão.
func (p ExternalPolicy) Validate(manifestPath string) []Diagnostic {
	var out []Diagnostic
	for i, e := range p.Allowlist {
		if faltando := e.faltando(); len(faltando) > 0 {
			out = append(out, Diagnostic{
				Code:         CodeM001,
				CanonicalKey: manifestPath,
				Detail: fmt.Sprintf("external[%d] (%s): entrada incompleta, falta %s",
					i, e.Package, strings.Join(faltando, ", ")),
			})
		}
	}
	for i, x := range p.Exceptions {
		if !x.Valid() {
			out = append(out, Diagnostic{
				Code:         CodeM001,
				CanonicalKey: manifestPath,
				Detail: fmt.Sprintf("exceptions[%d]: exceção não é nominal — exige unit, dependency, reason, owner e review_by",
					i),
			})
		}
	}
	return out
}

// EvaluateExternal decide um import que resolve FORA do universo (RFC §6).
//
// A matriz de §7 governa arestas entre unidades classificadas; ela não alcança
// um import de driver, SDK ou framework. Sem esta avaliação, a regra mais
// consequente do DMPF ficaria sem cobertura.
func EvaluateExternal(
	source Endpoint,
	sourceUnitID string,
	target string,
	sourceFile string,
	policy ExternalPolicy,
	closure Closure,
	isStandard func(string) bool,
) []Diagnostic {
	cap, conhecida := resolveCapability(target, policy, isStandard)
	if !conhecida {
		// Bloco permissivo aceita QUALQUER capability (RFC §6.2), então não
		// saber qual é não muda o veredicto — e fail-closed protege contra
		// afirmar conformidade sem decidir, não contra decidir com folga.
		// Fora deles, dependência que o verificador não consegue classificar
		// reprova (RFC §3.6, §10.2).
		if isPermissiveBlock(source.Block) {
			return nil
		}
		return []Diagnostic{{
			Code:         CodeE001,
			CanonicalKey: source.CanonicalKey,
			Target:       target,
			SourceFile:   sourceFile,
			Detail:       "dependência externa sem capability declarada na allowlist",
		}}
	}

	var out []Diagnostic
	if !CapabilityAllowed(source.Block, cap) && !policy.exceptionFor(sourceUnitID, target) {
		out = append(out, Diagnostic{
			Code:         CodeE001,
			CanonicalKey: source.CanonicalKey,
			Target:       target,
			SourceFile:   sourceFile,
			Detail:       fmt.Sprintf("capability %s não permitida para o bloco %s", cap, source.Block),
		})
	}

	// A pureza transitiva de RFC §6.3 vale para ENTRADA DA ALLOWLIST, computada
	// a partir dos entrypoints declarados. Não vale para builtin: §6.3 atribui
	// capability ao builtin diretamente ("net/http é io.network; crypto é
	// pure"), e descer o fechamento da stdlib chegaria sempre em internal/abi e
	// internal/runtime/*, tornando todo package puro impuro por construção.
	_, naAllowlist := policy.lookup(target)
	if cap == CapPure && naAllowlist {
		if impura, achada := impurezaNoFechamento(target, policy, closure, isStandard); achada {
			out = append(out, Diagnostic{
				Code:         CodeE002,
				CanonicalKey: source.CanonicalKey,
				Target:       target,
				SourceFile:   sourceFile,
				Detail: fmt.Sprintf("declarada pure, mas o fechamento alcança %s por %s",
					impura.capability, impura.via),
			})
		}
	}
	return out
}

// resolveCapability atribui a capability do import path. A allowlist do
// manifesto tem precedência sobre a tabela da stdlib: um projeto pode declarar
// um builtin de forma mais estrita, nunca mais permissiva por omissão.
func resolveCapability(importPath string, policy ExternalPolicy, isStandard func(string) bool) (Capability, bool) {
	if e, ok := policy.lookup(importPath); ok {
		if IsCapability(e.Capability) {
			return e.Capability, true
		}
		return "", false
	}
	if isStandard != nil && isStandard(importPath) {
		return StdlibCapability(importPath)
	}
	return "", false
}

type impureza struct {
	via        string
	capability Capability
}

// impurezaNoFechamento computa a pureza transitiva de RFC §6.3: um pacote
// declarado `pure` que introduza, por qualquer caminho do seu fechamento, uma
// capability diferente de `pure`, DEIXA DE SER pure.
//
// O fechamento parte dos entrypoints declarados, não do pacote inteiro — sem
// essa regra, qualquer pacote grande o bastante seria impuro por conter um
// acesso a I/O em algum subpath, e a política viraria proibição total de
// biblioteca externa no domínio.
func impurezaNoFechamento(entrypoint string, policy ExternalPolicy, closure Closure, isStandard func(string) bool) (impureza, bool) {
	if closure == nil {
		return impureza{}, false
	}
	deps, ok := closure(entrypoint)
	if !ok {
		return impureza{}, false
	}
	achados := map[string]Capability{}
	for _, d := range deps {
		if d == entrypoint {
			continue
		}
		cap, conhecida := resolveCapability(d, policy, isStandard)
		if !conhecida {
			// Dependência do fechamento sem classificação impede afirmar
			// pureza. Fail-closed: reporta como impureza não verificada.
			achados[d] = ""
			continue
		}
		if cap != CapPure {
			achados[d] = cap
		}
	}
	if len(achados) == 0 {
		return impureza{}, false
	}
	// Determinismo: entre vários caminhos impuros, reporta o menor import path.
	vias := make([]string, 0, len(achados))
	for d := range achados {
		vias = append(vias, d)
	}
	sort.Strings(vias)
	c := achados[vias[0]]
	if c == "" {
		c = "capability não declarada"
	}
	return impureza{via: vias[0], capability: c}, true
}
