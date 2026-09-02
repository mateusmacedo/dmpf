// Package baseline guarda uma cópia independente da classificação declarada.
//
// A classificação é auto-declarada: a unidade declara o que deveria
// restringi-la. Confrontá-la com esta cópia pega a divergência acidental. Já o
// autor que altera as duas no mesmo commit é outro problema, em authorization.go.
package baseline

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

const SchemaID = "dmpf/units-baseline@1"

// Entry carrega o membership resolvido além de bloco e contexto: o que importa
// controlar é qual classificação cada arquivo acaba recebendo, e remapear o
// `include` muda isso sem tocar em campo nenhum.
type Entry struct {
	Unit           string   `json:"unit"`
	Module         string   `json:"module"`
	Block          string   `json:"block"`
	BoundedContext string   `json:"bounded_context"`
	Membership     []string `json:"membership"`
}

// O Digest fecha o conjunto: alterar entrada sem recalcular reprova, e
// recalcular obriga a tocar o arquivo, tornando a mudança visível em revisão.
type Document struct {
	Schema  string  `json:"schema"`
	Digest  string  `json:"digest"`
	Entries []Entry `json:"entries"`
}

// Fica fora de qualquer módulo que ele descreva: dentro de um, o mesmo commit
// que muda a classificação mexeria nos dois lados e a cópia deixaria de ser
// independente.
const Path = "tools/dmpf-baseline/units-baseline.json"

func FromUniverse(units []rule.Unit, membership map[rule.UnitKey][]string) Document {
	entries := make([]Entry, 0, len(units))
	for _, u := range units {
		membros := slices.Clone(membership[u.Key()])
		sort.Strings(membros)
		entries = append(entries, Entry{
			Unit:           u.ID,
			Module:         u.Module,
			Block:          string(u.Block),
			BoundedContext: u.BoundedContext,
			Membership:     membros,
		})
	}
	ordenar(entries)
	return Document{Schema: SchemaID, Digest: Digest(entries), Entries: entries}
}

func ordenar(entries []Entry) {
	slices.SortStableFunc(entries, func(a, b Entry) int {
		return cmp.Or(
			cmp.Compare(a.Module, b.Module),
			cmp.Compare(a.Unit, b.Unit),
		)
	})
}

// Compare olha a classificação que vale na prática: campo alterado e membership
// remapeado dão o mesmo DMPF-T001, porque os dois mudam o que a unidade de fato
// classifica.
func Compare(versionado, derivado Document) []rule.Diagnostic {
	var out []rule.Diagnostic

	if versionado.Schema != SchemaID {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeT001,
			CanonicalKey: Path,
			Detail:       "schema " + quote(versionado.Schema) + " fora do conjunto fechado (esperado " + quote(SchemaID) + ")",
		})
	}

	// Contra as entradas do PRÓPRIO arquivo: se não fecha, comparar adiante
	// seria comparar com um documento que já não descreve a si mesmo.
	if esperado := Digest(versionado.Entries); versionado.Digest != esperado {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeT001,
			CanonicalKey: Path,
			Detail:       "digest não fecha com as entradas do arquivo: registrado " + quote(versionado.Digest) + ", calculado " + quote(esperado),
		})
	}

	noBaseline := indexar(versionado.Entries)
	noManifesto := indexar(derivado.Entries)

	for chave, d := range noManifesto {
		v, existe := noBaseline[chave]
		if !existe {
			out = append(out, rule.Diagnostic{
				Code:         rule.CodeT001,
				CanonicalKey: chave.String(),
				Detail:       "unidade declarada no manifesto e ausente do baseline",
			})
			continue
		}
		out = append(out, divergencias(chave, v, d)...)
	}
	for chave := range noBaseline {
		if _, existe := noManifesto[chave]; !existe {
			out = append(out, rule.Diagnostic{
				Code:         rule.CodeT001,
				CanonicalKey: chave.String(),
				Detail:       "unidade registrada no baseline e ausente do manifesto",
			})
		}
	}

	rule.SortDiagnostics(out)
	return out
}

func divergencias(chave rule.UnitKey, versionado, derivado Entry) []rule.Diagnostic {
	var out []rule.Diagnostic
	if versionado.Block != derivado.Block {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeT001,
			CanonicalKey: chave.String(),
			Detail:       "block: baseline " + quote(versionado.Block) + ", manifesto " + quote(derivado.Block),
		})
	}
	if versionado.BoundedContext != derivado.BoundedContext {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeT001,
			CanonicalKey: chave.String(),
			Detail:       "bounded_context: baseline " + quote(versionado.BoundedContext) + ", manifesto " + quote(derivado.BoundedContext),
		})
	}
	if !slices.Equal(versionado.Membership, derivado.Membership) {
		// Nenhum campo mudou, mas o conjunto que a unidade classifica mudou.
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeT001,
			CanonicalKey: chave.String(),
			Detail: "membership: baseline [" + strings.Join(versionado.Membership, " ") +
				"], manifesto [" + strings.Join(derivado.Membership, " ") + "]",
		})
	}
	return out
}

func indexar(entries []Entry) map[rule.UnitKey]Entry {
	out := make(map[rule.UnitKey]Entry, len(entries))
	for _, e := range entries {
		out[rule.UnitKey{Module: e.Module, ID: e.Unit}] = e
	}
	return out
}

func quote(s string) string { return `"` + s + `"` }
