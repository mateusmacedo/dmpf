package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

// Validate aplica o schema dmpf/units@1 a um documento JÁ decodificado.
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

// Não há herança: a ausência é avaliada na própria unidade, sem consultar
// módulo pai nem unidade irmã.
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

	// Contar "≥ 1" não substitui validar cada entrada: um valor inválido não
	// vira conforme porque os outros casaram.
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

	// Sem esta cláusula, marcar o domínio como público seria o caminho trivial
	// para outro contexto importá-lo — que é justamente o que se proíbe.
	if u.PublicIntegrationSurface && rule.Block(u.Block) == rule.BlockDomain {
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeM002,
			CanonicalKey: key,
			Detail:       fmt.Sprintf("%s: public_integration_surface: true em unidade de bloco domain", where),
		})
	}

	return out
}

// Glob é o binding de TypeScript; aqui `**`, `.` e `..` são valores inválidos,
// não sintaxe de subárvore.
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
