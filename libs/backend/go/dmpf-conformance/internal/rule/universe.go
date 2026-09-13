package rule

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Unit é o que a classificação recai sobre. Em Go, os packages que o `include`
// captura por import path EXATO, dentro do próprio Module.
type Unit struct {
	ID                       string
	Block                    Block
	BoundedContext           string
	PublicIntegrationSurface bool
	SharedKernel             bool
	Include                  []string
	Module                   string
	ManifestPath             string
}

// Qualificado por módulo porque o `id` só é único dentro de um manifesto: dois
// módulos podem declarar `id: "domain"` legitimamente.
type UnitKey struct {
	Module string
	ID     string
}

func (u Unit) Key() UnitKey { return UnitKey{Module: u.Module, ID: u.ID} }

func (k UnitKey) String() string { return k.Module + "#" + k.ID }

// CanonicalKey é o import path completo — em Go, a identidade do package.
type Package struct {
	CanonicalKey string
	Module       string
}

// Descoberto por mais de uma fonte, e presente em qualquer uma delas já entra:
// omitir o módulo do go.work não o tira do alcance do DMPF-U004.
type Module struct {
	Path          string
	Dir           string
	HasManifest   bool
	HasProduction bool

	// Membro do go.work resolve NO workspace: fora dele, um require sem
	// `replace` local cairia na versão publicada, e o grafo analisado deixaria
	// de ser o que o build produz.
	WorkspaceMember bool
}

// Universe guarda as unidades em cópia própria: o veredicto não pode mudar
// depois da construção porque o chamador mexeu no slice que passou.
type Universe struct {
	units       []Unit
	byKey       map[string]int
	descobertos map[string]bool
}

// Discovered separa "package do universo sem classificação" de "dependência
// externa": o primeiro já reprovou em U001, e avaliá-lo como externo emitiria
// um E001 espúrio sobre código do próprio universo.
func (u *Universe) Discovered(canonicalKey string) bool {
	return u.descobertos[canonicalKey]
}

// Quais packages cada unidade acabou capturando. É isso que o baseline precisa
// registrar: remapear o `include` muda a classificação sem tocar em campo algum.
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

// Lookup devolve cópia profunda: expor o Include interno deixaria o consumidor
// alterar a classificação já construída.
func (u *Universe) Lookup(canonicalKey string) (Unit, bool) {
	i, ok := u.byKey[canonicalKey]
	if !ok {
		return Unit{}, false
	}
	return cloneUnit(u.units[i]), true
}

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
		SharedKernel:             unit.SharedKernel,
	}, true
}

func cloneUnit(u Unit) Unit {
	u.Include = slices.Clone(u.Include)
	return u
}

// NormalizeInclude: a barra final é a única tolerância de forma no `include`.
func NormalizeInclude(include string) string {
	return strings.TrimSuffix(include, "/")
}

// O casamento é exato, não por prefixo: por prefixo, `x/domain/infra` herdaria
// a classificação de `x/domain` só por estar embaixo dele. Classificar pelo
// lugar do diretório é o que a norma recusa, e abriria caminho para código novo
// entrar sem edição revisável do manifesto. Glob é o binding de TypeScript.
func covers(include, canonicalKey string) bool {
	return NormalizeInclude(include) == canonicalKey
}

// O default de toda ramificação ausente é reprovar: package sem unidade não é
// ignorado, é DMPF-U001.
func BuildUniverse(units []Unit, packages []Package, modules []Module) (*Universe, []Diagnostic) {
	var diags []Diagnostic

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
