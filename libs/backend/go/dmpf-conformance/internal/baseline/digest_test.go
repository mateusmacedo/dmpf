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
