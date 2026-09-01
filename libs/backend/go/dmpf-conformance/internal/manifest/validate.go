package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// Validate aplica o schema dmpf/units@1 a um documento decodificado e devolve os
// diagnósticos de RFC §10.1 em ordem determinística.
//
// Nenhuma decodificação ocorre aqui: o documento chega como modelo puro.
func Validate(doc Document) []rule.Diagnostic {
	var out []rule.Diagnostic
	key := doc.Path

	if doc.Schema != SchemaID {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeM002,
			CanonicalKey: key,
			Detail:       fmt.Sprintf("schema %q fora do conjunto fechado (esperado %q)", doc.Schema, SchemaID),
		})
	}

	seen := map[string]int{}
	for i, u := range doc.Units {
		out = append(out, validateUnit(key, i, u)...)
		if u.PresentID && u.ID != "" {
			seen[u.ID]++
		}
	}
	for id, n := range seen {
		if n > 1 {
			out = append(out, rule.Diagnostic{
				Code:         rule.CodeM003,
				CanonicalKey: key,
				Detail:       fmt.Sprintf("id %q declarado %d vezes no mesmo manifesto", id, n),
			})
		}
	}

	rule.SortDiagnostics(out)
	return out
}

// validateUnit cobre M001 (campo obrigatório ausente) e M002 (valor fora do
// conjunto fechado) de uma unidade. Herança não existe: a ausência é avaliada na
// própria unidade, sem consultar módulo pai nem unidade irmã.
func validateUnit(key string, i int, u Unit) []rule.Diagnostic {
	var out []rule.Diagnostic
	where := fmt.Sprintf("units[%d]", i)
	if u.PresentID && u.ID != "" {
		where = fmt.Sprintf("units[%d] (%q)", i, u.ID)
	}

	for _, f := range []struct {
		name    string
		present bool
		empty   bool
	}{
		{"id", u.PresentID, u.ID == ""},
		{"block", u.PresentBlock, u.Block == ""},
		{"bounded_context", u.PresentBoundedContext, u.BoundedContext == ""},
		{"include", u.PresentInclude, len(u.Include) == 0},
	} {
		if !f.present || f.empty {
			out = append(out, rule.Diagnostic{
				Code:         rule.CodeM001,
				CanonicalKey: key,
				Detail:       fmt.Sprintf("%s: campo obrigatório %q ausente ou vazio", where, f.name),
			})
		}
	}

	// Cada elemento de `include` é validado individualmente: um `include` com um
	// valor que não é import path não pode ser reportado como conforme só porque
	// os outros elementos casaram. A RFC §10.1 exige "≥ 1 por unidade", e a
	// contagem não substitui a validade de cada entrada.
	for k, inc := range u.Include {
		if motivo := invalidImportPath(inc); motivo != "" {
			out = append(out, rule.Diagnostic{
				Code:         rule.CodeM002,
				CanonicalKey: key,
				Detail:       fmt.Sprintf("%s: include[%d] %q não é import path válido: %s", where, k, inc, motivo),
			})
		}
	}

	if u.PresentBlock && u.Block != "" && !rule.IsBlock(rule.Block(u.Block)) {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeM002,
			CanonicalKey: key,
			Detail:       fmt.Sprintf("%s: block %q fora do conjunto fechado dos seis", where, u.Block),
		})
	}

	// RFC §7.2 e ADR-017: uma domain library NUNCA é superfície pública de
	// integração. Sem esta cláusula o campo seria o caminho trivial para burlar
	// a proibição de import entre bounded contexts (§5.5).
	if u.PublicIntegrationSurface && rule.Block(u.Block) == rule.BlockDomain {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeM002,
			CanonicalKey: key,
			Detail:       fmt.Sprintf("%s: public_integration_surface: true em unidade de bloco domain", where),
		})
	}

	return out
}

// invalidImportPath devolve o motivo pelo qual o valor não serve como import
// path no binding Go, ou string vazia quando serve. O binding com glob é o de
// TypeScript (RFC §10.1); aqui `**`, `.` e `..` são valores inválidos, não
// sintaxe de subtree.
func invalidImportPath(v string) string {
	norm := rule.NormalizeInclude(v)
	switch {
	case strings.TrimSpace(v) == "":
		return "vazio"
	case norm == "":
		return "vazio após normalizar a barra final"
	case v != strings.TrimSpace(v):
		return "espaço em branco nas bordas"
	case strings.HasPrefix(norm, "/"):
		return "barra inicial"
	case strings.Contains(norm, "//"):
		return "barra dupla"
	case strings.ContainsAny(norm, " \t\n"):
		return "espaço no meio"
	}
	for _, seg := range strings.Split(norm, "/") {
		switch seg {
		case ".", "..":
			return "segmento relativo " + strconv.Quote(seg)
		case "*", "**":
			return "glob não é binding de Go (RFC §10.1): declare o import path"
		}
	}
	return ""
}
