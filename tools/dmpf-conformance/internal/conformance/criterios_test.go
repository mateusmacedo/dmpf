package conformance_test

import (
	"path/filepath"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/golist"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

func caminhoDaFixture(t *testing.T, cenario, modulo string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(fixtures, cenario, modulo))
	if err != nil {
		t.Fatal(err)
	}
	return p
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
