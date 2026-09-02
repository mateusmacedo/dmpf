package conformance_test

import (
	"path/filepath"
	"slices"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/baseline"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/golist"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

func caminhoDaFixture(t *testing.T, cenario, modulo string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(fixtures, cenario, modulo))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func entrada(unit, block, bc string, membros ...string) baseline.Entry {
	return baseline.Entry{Unit: unit, Module: "m", Block: block, BoundedContext: bc, Membership: membros}
}

func entradasDe(entries ...baseline.Entry) baseline.Document {
	return baseline.Document{Schema: baseline.SchemaID, Digest: baseline.Digest(entries), Entries: entries}
}

// TestTesteNaoAlteraClassificacao: um arquivo de teste importando rede dentro de
// uma unidade de domínio não pode reprovar.
//
// A exclusão é fechada, e teste não altera a classificação do código sob teste.
// Se reprovasse, todo módulo com teste de integração ficaria impedido de ter
// domínio — e o falso positivo é pior que o falso negativo aqui, porque ensina
// o time a ignorar o gate.
func TestTesteNaoAlteraClassificacao(t *testing.T) {
	rel := rodar(t, "exclusoes", "exemplo.test/ex-a")

	for _, d := range rel.Diagnostics {
		if d.CanonicalKey == "exemplo.test/ex-a/domain" && d.Target == "net/http" {
			t.Errorf("import de teste reprovou o código sob teste: %s", d)
		}
	}
}

// TestCodigoGeradoNaoIsenta: gerar não isenta. O arquivo é consumido em runtime
// e responde pelas regras do bloco que o declara, como código escrito à mão.
//
// É a exclusão que mais tenta a rotulagem conveniente: marcar como gerado o que
// se quer esconder.
func TestCodigoGeradoNaoIsenta(t *testing.T) {
	rel := rodar(t, "exclusoes", "exemplo.test/ex-a")

	var achou bool
	for _, d := range rel.Diagnostics {
		if d.Code == rule.CodeE001 && d.CanonicalKey == "exemplo.test/ex-a/gerado" && d.Target == "net/http" {
			achou = true
		}
	}
	if !achou {
		t.Fatalf("código gerado com acesso à rede num bloco de domínio passou: %v", rel.Diagnostics)
	}
}

// TestAliasProduzUmaArestaSo: o mesmo package importado com dois nomes locais
// produz uma aresta, não duas.
//
// A decisão é sobre o package que o compilador resolveu, não sobre o nome
// escrito no arquivo. Se a forma do import mudasse o veredicto, bastaria
// renomear para escapar.
func TestAliasProduzUmaArestaSo(t *testing.T) {
	modules := []rule.Module{{
		Path: "exemplo.test/al-a",
		Dir:  caminhoDaFixture(t, "alias", "al-a"),
	}}
	edges, err := golist.New("", modules, perfilLinuxAmd64).Edges()
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}

	var n int
	for _, e := range edges {
		if e.From == "exemplo.test/al-a/consumidor" && e.To == "exemplo.test/al-a/alvo" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("dois nomes locais para o mesmo package produziram %d aresta(s), esperada 1", n)
	}

	// E o veredicto não muda: app -> domain no mesmo contexto é permitido.
	rel := rodar(t, "alias", "exemplo.test/al-a")
	if len(rel.Diagnostics) != 0 {
		t.Errorf("cenário com alias reprovou: %v", rel.Diagnostics)
	}
}

// TestRemapeamentoDeIncludeExigeAval fecha o caso mais silencioso: mover um
// package de uma unidade para outra muda a classificação dele sem que nenhum
// campo seja editado.
//
// Um controle que olhasse só bloco e contexto por unidade não veria nada — as
// duas unidades continuam com os mesmos valores. O que mudou foi quem classifica
// o quê.
func TestRemapeamentoDeIncludeExigeAval(t *testing.T) {
	antes := entradasDe(
		entrada("a", "domain", "bc", "m/p1", "m/p2"),
		entrada("b", "app", "bc"),
	)
	agora := entradasDe(
		entrada("a", "domain", "bc", "m/p1"),
		entrada("b", "app", "bc", "m/p2"),
	)

	store := storeFalso{
		agora: agora, antes: antes, tinha: true,
		commits: []baseline.Commit{{
			SHA:      "aaa",
			Arquivos: []string{"tools/dmpf-baseline/units-baseline.json", "m/dmpf-units.json", "m/p2/p.go"},
		}},
	}

	mudancas := baseline.Detectar(antes, agora)
	if len(mudancas) == 0 {
		t.Fatal("remapeamento não foi detectado como ato que exige aval")
	}
	for _, m := range mudancas {
		if m.Ato != baseline.AtoRemapearMembership {
			t.Errorf("ato %q, esperado remapeamento", m.Ato)
		}
	}

	ds := baseline.VerificarAutorizacao(mudancas, store.commits)
	if !slices.ContainsFunc(ds, func(d rule.Diagnostic) bool { return d.Code == rule.CodeT002 }) {
		t.Fatalf("remapeamento misturado com código passou sem exigir aval: %v", ds)
	}
}
