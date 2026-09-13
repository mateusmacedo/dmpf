package baseline

import "testing"

// TestDigestEInjetivo: conjuntos distintos precisam produzir digests distintos,
// inclusive quando um valor contém os bytes que uma codificação por delimitador
// usaria como fronteira.
//
// O baseline é um arquivo que quem abre o PR edita, e o digest existe para
// FECHAR o conjunto — obrigar quem altera uma entrada a recalculá-lo. Uma
// codificação por separador satisfaz isso só enquanto nenhum valor contém o
// separador; com prefixo de comprimento, a injetividade não depende do conteúdo.
func TestDigestEInjetivo(t *testing.T) {
	casos := []struct {
		nome string
		a, b []Entry
	}{
		{
			"membership com 0x1f embutido",
			[]Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc", Membership: []string{"a\x1fb"}}},
			[]Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc", Membership: []string{"a", "b"}}},
		},
		{
			// Sem prefixo de comprimento por CAMPO, "m"+"u" e "mu"+"" produzem
			// a mesma sequência: a fronteira entre campos adjacentes some.
			"deslocamento entre campos adjacentes",
			[]Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc"}},
			[]Entry{{Module: "mu", Unit: "", Block: "domain", BoundedContext: "bc"}},
		},
		{
			"deslocamento entre block e bounded_context",
			[]Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc"}},
			[]Entry{{Module: "m", Unit: "u", Block: "domainb", BoundedContext: "c"}},
		},
		{
			"campo com 0x1f embutido",
			[]Entry{{Module: "m", Unit: "u\x1fdomain", Block: "", BoundedContext: "bc", Membership: nil}},
			[]Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc", Membership: nil}},
		},
		{
			"entrada com 0x1e embutido",
			[]Entry{{Module: "m", Unit: "u", Block: "d", BoundedContext: "bc", Membership: []string{"x\x1em"}}},
			[]Entry{
				{Module: "m", Unit: "u", Block: "d", BoundedContext: "bc", Membership: []string{"x"}},
				{Module: "m", Unit: "", Block: "", BoundedContext: "", Membership: nil},
			},
		},
	}
	for _, c := range casos {
		if Digest(c.a) == Digest(c.b) {
			t.Errorf("colisão em %q: conjuntos distintos produziram o mesmo digest", c.nome)
		}
	}
}

// TestDigestVetoresGoldenLegado fixa os hashes conhecidos ANTES do shared
// kernel existir: se a fórmula legada mudar por engano, este teste denuncia.
func TestDigestVetoresGoldenLegado(t *testing.T) {
	casos := []struct {
		nome    string
		entries []Entry
		golden  string
	}{
		{
			"duas entradas",
			[]Entry{
				{Module: "m1", Unit: "u1", Block: "domain", BoundedContext: "bc1", Membership: []string{"m1/a", "m1/b"}},
				{Module: "m2", Unit: "u2", Block: "port", BoundedContext: "bc2", Membership: nil},
			},
			"sha256:b08267558c3f972a6fdee71e6a66e8373f2d74074a9ba086ca1a4a514d439cea",
		},
		{"vazio", nil, "sha256:af5570f5a1810b7af78caf4bc70a660f0df51e42baf91d4de5b2328de0e83dfc"},
	}
	for _, c := range casos {
		if got := Digest(c.entries); got != c.golden {
			t.Errorf("%s: got %s, want %s", c.nome, got, c.golden)
		}
	}
}

func TestDigestOfSemPresencaReplicaOLegado(t *testing.T) {
	entries := []Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc", Membership: []string{"m/a"}}}
	doc := Document{Entries: entries}
	if got, want := DigestOf(doc), Digest(entries); got != want {
		t.Errorf("DigestOf sem presença = %s, want %s (legado)", got, want)
	}
}

func TestDigestOfSensivelALista(t *testing.T) {
	entries := []Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc"}}
	base := DigestOf(Document{Entries: entries, HasSharedKernelUnits: true, SharedKernelUnits: []string{"u"}})
	outra := DigestOf(Document{Entries: entries, HasSharedKernelUnits: true, SharedKernelUnits: []string{"u", "v"}})
	if base == outra {
		t.Error("DigestOf insensível a mudança na lista de shared kernel")
	}
}

// TestDigestOfPresencaEhSignificativa: lista vazia DECLARADA (Has=true) não
// pode fechar igual a "chave ausente" — são estados distintos do arquivo.
func TestDigestOfPresencaEhSignificativa(t *testing.T) {
	entries := []Entry{{Module: "m", Unit: "u", Block: "domain", BoundedContext: "bc"}}
	semPresenca := DigestOf(Document{Entries: entries})
	comPresencaVazia := DigestOf(Document{Entries: entries, HasSharedKernelUnits: true, SharedKernelUnits: []string{}})
	if semPresenca == comPresencaVazia {
		t.Error("presença da chave shared_kernel_units (mesmo vazia) deveria mudar o digest")
	}
}
