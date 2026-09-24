package exception

import (
	"fmt"
	"strings"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

// O universo do que nenhuma exceção alcança é FECHADO e decidido por regra
// sobre o objeto, nunca por identidade textual: um pedido que só mudasse de
// nome escaparia de uma lista de nomes, não desta tabela.
type deniedRule struct {
	item   string
	code   rule.Code
	detail string
	match  func(o Object) bool
}

// A ordem é a do catálogo N1–N7 de GOV-32.
var deniedCatalog = []deniedRule{
	{
		item:   "N1",
		code:   rule.CodeX004,
		detail: "constraint P0 não admite exceção",
		match:  identityHasPrefix("P0-"),
	},
	{
		item:   "N2",
		code:   rule.CodeX003,
		detail: "aresta entre unidades não admite exceção: a regra de dependência é a norma, não uma política negociável",
		match:  identityNamesEdge,
	},
	{
		item:   "N2",
		code:   rule.CodeX003,
		detail: "célula da matriz de blocos não admite exceção",
		match:  identityNamesCell,
	},
	{
		item:   "N3",
		code:   rule.CodeX004,
		detail: "âncora versionada não admite exceção",
		match:  identityHasPrefix("ANC-"),
	},
	{
		item:   "N4",
		code:   rule.CodeX004,
		detail: "gatilho de autorização T1–T6 não admite exceção",
		match:  identityNamesTrigger,
	},
	{
		item:   "N5",
		code:   rule.CodeX004,
		detail: "classificação de bloco ou bounded context não admite exceção",
		match:  identityIsClassification,
	},
	{
		item:   "N6",
		code:   rule.CodeX004,
		detail: "rito Buf e plugins pinados não admitem exceção",
		match:  identityIsBufRite,
	},
	{
		item:   "N7",
		code:   rule.CodeX004,
		detail: "evidência de certificação não admite exceção",
		match:  identityHasPrefix("evidence_"),
	},
}

func identityHasPrefix(prefix string) func(Object) bool {
	return func(o Object) bool {
		return strings.HasPrefix(o.Identity, prefix)
	}
}

// A seta é a forma com que o verificador nomeia aresta no relatório; pedir por
// ela é pedir para desligar C1 ou C2 num par.
func identityNamesEdge(o Object) bool {
	return strings.Contains(o.Identity, "->")
}

func identityNamesCell(o Object) bool {
	return strings.HasPrefix(o.Identity, "cell:")
}

// T1–T6 como identidade inteira ou como sufixo do nome do gatilho: `T3` e
// `autorizacao-T3` pedem a mesma coisa.
func identityNamesTrigger(o Object) bool {
	id := o.Identity
	if len(id) < 2 {
		return false
	}
	last := id[len(id)-1]
	if id[len(id)-2] != 'T' || last < '1' || last > '6' {
		return false
	}
	if len(id) == 2 {
		return true
	}
	prev := id[len(id)-3]
	return prev == '-' || prev == '_' || prev == ':' || prev == '/' || prev == ' '
}

func identityIsClassification(o Object) bool {
	id := strings.ToLower(o.Identity)
	return id == "block" || id == "bounded_context" ||
		strings.HasPrefix(id, "block:") || strings.HasPrefix(id, "bounded_context:")
}

func identityIsBufRite(o Object) bool {
	for _, marker := range []string{
		"tools/buf.sh",
		"buf.gen.yaml",
		"github.com/bufbuild/buf",
		"protoc-gen-",
	} {
		if strings.Contains(o.Identity, marker) {
			return true
		}
	}
	return false
}

// A outra metade de N1: exceção externa autoriza um import nominal, nunca a
// política de capability do bloco. Em domain e port essa política é RFC §6.2 e
// constraint P0, e furá-la por exceção seria reescrever a norma no manifesto.
func blockPolicyDiagnostics(x Exception, s Subject) []rule.Diagnostic {
	if s.Block == "" || x.Object.Kind != KindExternalDependency {
		return nil
	}
	if s.Block != rule.BlockDomain && s.Block != rule.BlockPort {
		return nil
	}
	if s.Capability == rule.CapPure {
		return nil
	}

	qual := string(s.Capability)
	if qual == "" {
		qual = "de capability não classificada"
	} else {
		qual = "de capability " + qual
	}
	return []rule.Diagnostic{{
		Code:   rule.CodeX003,
		Target: x.Object.Identity,
		Detail: fmt.Sprintf(
			"N1: o bloco %s admite apenas pure (RFC §6.2, constraint P0), e a dependência é %s",
			s.Block, qual),
	}}
}

// deniedBy devolve TODOS os itens alcançados, não o primeiro: um pedido que
// esbarra em dois itens do catálogo precisa mostrar os dois a quem o escreveu.
func deniedBy(o Object) []rule.Diagnostic {
	var out []rule.Diagnostic
	for _, r := range deniedCatalog {
		if r.match(o) {
			out = append(out, rule.Diagnostic{
				Code:   r.code,
				Target: o.Identity,
				Detail: r.item + ": " + r.detail,
			})
		}
	}
	return out
}
