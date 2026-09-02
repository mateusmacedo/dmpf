package rule

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Unit é a verification_unit já classificada e validada: em Go, um conjunto de
// packages capturados por `include`, que casa por import path EXATO (ADR-011).
// Module é o ownership_module que declara a unidade — uma unidade só pode
// possuir packages do próprio módulo.
type Unit struct {
	ID                       string
	Block                    Block
	BoundedContext           string
	PublicIntegrationSurface bool
	Include                  []string
	Module                   string
	ManifestPath             string
}

// UnitKey identifica uma unidade no universo. O `id` só é único DENTRO de um
// manifesto (§10.1 o exige por documento), então dois módulos podem declarar
// `id: "domain"` legitimamente. A identidade do proprietário é o par.
type UnitKey struct {
	Module string
	ID     string
}

func (u Unit) Key() UnitKey { return UnitKey{Module: u.Module, ID: u.ID} }

func (k UnitKey) String() string { return k.Module + "#" + k.ID }

// Package é um package de produção descoberto no inventário. CanonicalKey é o
// import path completo (ADR-011; RFC §3.3); Module é o módulo que o contém.
type Package struct {
	CanonicalKey string
	Module       string
}

// Module é um módulo do inventário independente — reconciliação de go.mod
// rastreados, projetos Nx `stack:go` e membros do go.work. Módulo presente em
// QUALQUER fonte entra no universo: a omissão do go.work não o tira do alcance
// de DMPF-U004.
type Module struct {
	Path          string
	Dir           string
	HasManifest   bool
	HasProduction bool

	// WorkspaceMember indica que o módulo é `use` do go.work. A resolução dele
	// tem de acontecer NO workspace: fora dele, um require sem `replace` local
	// cairia na versão publicada, e o verificador analisaria um grafo que não é
	// o que o build produz.
	WorkspaceMember bool
}

// Universe é o mapeamento de cada package de produção à sua unidade. As
// unidades são armazenadas em cópia própria, com os slices clonados: o
// veredicto não pode mudar depois da construção porque o chamador mexeu no
// slice que passou.
type Universe struct {
	units       []Unit
	byKey       map[string]int
	descobertos map[string]bool
}

// Discovered reporta se o package foi descoberto como código de produção,
// mesmo que nenhuma unidade o classifique.
//
// A distinção importa: um package descoberto e não classificado já reprovou em
// U001, e tratá-lo como dependência externa por não estar em `byKey` emitiria
// um E001 espúrio sobre código que é do universo — trocando a causa real por
// um sintoma inventado.
func (u *Universe) Discovered(canonicalKey string) bool {
	return u.descobertos[canonicalKey]
}

// Membership devolve, por unidade, as canonical_keys que o `include` capturou.
// É o delta efetivo que o baseline precisa registrar (ADR-028): remapear
// `include` muda a classificação efetiva sem tocar em campo algum. A chave é
// qualificada por módulo — `id` homônimo em módulos distintos é legítimo.
func (u *Universe) Membership() map[UnitKey][]string {
	out := map[UnitKey][]string{}
	for key, i := range u.byKey {
		k := u.units[i].Key()
		out[k] = append(out[k], key)
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// Lookup devolve a unidade que contém o package, em cópia profunda.
func (u *Universe) Lookup(canonicalKey string) (Unit, bool) {
	i, ok := u.byKey[canonicalKey]
	if !ok {
		return Unit{}, false
	}
	return cloneUnit(u.units[i]), true
}

// Endpoint devolve o lado de aresta já classificado para um package do universo.
func (u *Universe) Endpoint(canonicalKey string) (Endpoint, bool) {
	i, ok := u.byKey[canonicalKey]
	if !ok {
		return Endpoint{}, false
	}
	unit := u.units[i]
	return Endpoint{
		CanonicalKey:             canonicalKey,
		Block:                    unit.Block,
		BoundedContext:           unit.BoundedContext,
		PublicIntegrationSurface: unit.PublicIntegrationSurface,
	}, true
}

func cloneUnit(u Unit) Unit {
	u.Include = slices.Clone(u.Include)
	return u
}

// NormalizeInclude tira a barra final de um import path declarado. É a única
// tolerância de forma: `x/domain/` e `x/domain` designam o mesmo package.
func NormalizeInclude(include string) string {
	return strings.TrimSuffix(include, "/")
}

// covers reporta se o include designa EXATAMENTE o package.
//
// O casamento é exato, não por prefixo: em Go a verification_unit é o package e
// a canonical_key é o import path completo (ADR-011; RFC §3.3). Casar por
// prefixo faria `x/domain/infra` herdar a classificação de `x/domain` pelo lugar
// em que o diretório está — inferência por convenção, que ADR-012 e RFC §4.4
// proíbem, e um caminho para classificar package novo sem edição revisável do
// manifesto. O binding com glob é o de TypeScript (RFC §10.1); o de Go é import
// path.
func covers(include, canonicalKey string) bool {
	return NormalizeInclude(include) == canonicalKey
}

// BuildUniverse mapeia os packages de produção às unidades e emite os
// diagnósticos de cobertura de RFC §3.6: U001 (não coberto), U002 (coberto mais
// de uma vez), U003 (canonical_key duplicada) e U004 (módulo de produção sem
// manifesto).
//
// O default de toda ramificação ausente é reprovar: package sem unidade não é
// ignorado, é U001.
func BuildUniverse(units []Unit, packages []Package, modules []Module) (*Universe, []Diagnostic) {
	var diags []Diagnostic

	// U003 — canonical_key duplicada no universo.
	count := map[string]int{}
	for _, p := range packages {
		count[p.CanonicalKey]++
	}
	for key, n := range count {
		if n > 1 {
			diags = append(diags, Diagnostic{
				Code:         CodeU003,
				CanonicalKey: key,
				Detail:       fmt.Sprintf("canonical_key descoberta %d vezes no universo", n),
			})
		}
	}

	// U004 — módulo de produção com código e sem manifesto. O inventário é
	// independente do go.work justamente para que a omissão não escape daqui.
	for _, m := range modules {
		if m.HasProduction && !m.HasManifest {
			diags = append(diags, Diagnostic{
				Code:         CodeU004,
				CanonicalKey: m.Path,
				Detail:       "módulo com código de produção e sem dmpf-units.json",
			})
		}
	}

	own := make([]Unit, len(units))
	for i := range units {
		own[i] = cloneUnit(units[i])
	}
	u := &Universe{units: own, byKey: map[string]int{}, descobertos: map[string]bool{}}

	seen := map[string]bool{}
	for _, pkg := range packages {
		if seen[pkg.CanonicalKey] {
			continue
		}
		seen[pkg.CanonicalKey] = true
		u.descobertos[pkg.CanonicalKey] = true

		var owners []string
		owner := -1
		for j := range own {
			// A unidade só possui packages do próprio ownership_module. Sem
			// esta comparação, unidade de módulo pai capturaria package de
			// módulo aninhado e o classificaria pelo módulo errado.
			if own[j].Module != pkg.Module {
				continue
			}
			for _, inc := range own[j].Include {
				if covers(inc, pkg.CanonicalKey) {
					owners = append(owners, own[j].Key().String())
					if owner < 0 {
						owner = j
					}
					break
				}
			}
		}

		switch len(owners) {
		case 0:
			diags = append(diags, Diagnostic{
				Code:         CodeU001,
				CanonicalKey: pkg.CanonicalKey,
				Detail:       "package de produção fora de todo include",
			})
		case 1:
			u.byKey[pkg.CanonicalKey] = owner
		default:
			sort.Strings(owners)
			diags = append(diags, Diagnostic{
				Code:         CodeU002,
				CanonicalKey: pkg.CanonicalKey,
				Detail:       "package coberto pelas unidades " + strings.Join(owners, ", "),
			})
			// Coberto duas vezes é ambiguidade de classificação: a unidade NÃO
			// é registrada, e toda aresta que toque este package reprova por
			// U001 em vez de decidir sobre classificação que não vale.
		}
	}

	SortDiagnostics(diags)
	return u, diags
}
