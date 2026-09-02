package baseline

import (
	"slices"
	"strings"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

type Ato string

const (
	AtoAlterarBlock          Ato = "alterar block"
	AtoAlterarBoundedContext Ato = "alterar bounded_context"
	AtoCriarUnidade          Ato = "criar unidade"
	AtoRemoverUnidade        Ato = "remover unidade"
	AtoRemapearMembership    Ato = "remapear membership"
)

type MudancaNormativa struct {
	Ato      Ato
	Unidade  rule.UnitKey
	Anterior string
	Novo     string
}

// O que basta saber do commit para julgar se ele isolou a mudança.
type Commit struct {
	SHA      string
	Autor    string
	Assunto  string
	Arquivos []string
}

// O que conta é a classificação que cada arquivo passa a receber, não a edição
// de um campo: olhar só os campos deixaria a regra evitável por remapeamento.
func Detectar(anterior, novo Document) []MudancaNormativa {
	var out []MudancaNormativa
	antes := indexar(anterior.Entries)
	depois := indexar(novo.Entries)

	chaves := make([]rule.UnitKey, 0, len(antes)+len(depois))
	for k := range antes {
		chaves = append(chaves, k)
	}
	for k := range depois {
		if _, ja := antes[k]; !ja {
			chaves = append(chaves, k)
		}
	}
	slices.SortFunc(chaves, func(a, b rule.UnitKey) int { return strings.Compare(a.String(), b.String()) })

	for _, k := range chaves {
		a, tinha := antes[k]
		d, tem := depois[k]
		switch {
		case !tinha:
			out = append(out, MudancaNormativa{Ato: AtoCriarUnidade, Unidade: k, Novo: d.Block})
		case !tem:
			out = append(out, MudancaNormativa{Ato: AtoRemoverUnidade, Unidade: k, Anterior: a.Block})
		default:
			if a.Block != d.Block {
				out = append(out, MudancaNormativa{Ato: AtoAlterarBlock, Unidade: k, Anterior: a.Block, Novo: d.Block})
			}
			if a.BoundedContext != d.BoundedContext {
				out = append(out, MudancaNormativa{
					Ato: AtoAlterarBoundedContext, Unidade: k,
					Anterior: a.BoundedContext, Novo: d.BoundedContext,
				})
			}
			if !slices.Equal(a.Membership, d.Membership) {
				out = append(out, MudancaNormativa{
					Ato: AtoRemapearMembership, Unidade: k,
					Anterior: strings.Join(a.Membership, " "), Novo: strings.Join(d.Membership, " "),
				})
			}
		}
	}
	return out
}

// Editar qualquer um destes é mudar a classificação.
//
// Manifesto sob `testdata/` fica de fora: é dado de teste, e o módulo sintético
// que ele descreve não entra no universo. Contá-lo faria um commit que só ajusta
// fixture ser lido como ato de classificação misturado com código.
func arquivoNormativo(p string) bool {
	if ehDadoDeTeste(p) {
		return false
	}
	return p == Path || strings.HasSuffix(p, "/dmpf-units.json") || p == "dmpf-units.json"
}

func ehDadoDeTeste(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "testdata" || seg == "vendor" {
			return true
		}
	}
	return false
}

// Mudar a classificação exige duas coisas: vir num commit separado do código, e
// ser aprovada por alguém que não seja o autor.
//
// Só a primeira é verificável aqui. A aprovação vive na forge e só existe
// depois de o CI rodar, então cobrá-la neste ponto travaria todo PR: ele nunca
// ficaria verde, porque a aprovação vem depois da verificação.
func VerificarAutorizacao(mudancas []MudancaNormativa, commits []Commit) []rule.Diagnostic {
	if len(mudancas) == 0 {
		return nil
	}

	if len(commits) == 0 {
		return []rule.Diagnostic{{
			Code:         rule.CodeT002,
			CanonicalKey: Path,
			Detail:       "mudança normativa sem histórico para avaliar o commit próprio: não verificado",
		}}
	}

	var out []rule.Diagnostic
	for _, c := range commits {
		var normativos, codigo []string
		for _, f := range c.Arquivos {
			if arquivoNormativo(f) {
				normativos = append(normativos, f)
			} else {
				codigo = append(codigo, f)
			}
		}
		if len(normativos) == 0 || len(codigo) == 0 {
			continue
		}
		out = append(out, rule.Diagnostic{
			Code:         rule.CodeT002,
			CanonicalKey: Path,
			Detail: "commit " + curto(c.SHA) + " mistura mudança normativa (" + strings.Join(normativos, ", ") +
				") com mudança de código (" + primeiros(codigo, 3) + "): o mecanismo mínimo exige commit próprio",
		})
	}

	rule.SortDiagnostics(out)
	return out
}

func Descrever(mudancas []MudancaNormativa) string {
	partes := make([]string, 0, len(mudancas))
	for _, m := range mudancas {
		p := string(m.Ato) + " em " + m.Unidade.String()
		if m.Anterior != "" || m.Novo != "" {
			p += " (" + m.Anterior + " -> " + m.Novo + ")"
		}
		partes = append(partes, p)
	}
	return strings.Join(partes, "; ")
}

func curto(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func primeiros(v []string, n int) string {
	if len(v) <= n {
		return strings.Join(v, ", ")
	}
	return strings.Join(v[:n], ", ") + ", ..."
}
