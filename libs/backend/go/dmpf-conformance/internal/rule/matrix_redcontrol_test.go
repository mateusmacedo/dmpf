package rule

import "testing"

// Se o oráculo fosse computado de matrix.go, corromper a produção não faria a
// suite falhar: os dois lados mudariam juntos. Prova comportamental — a
// estrutural, que pega o oráculo derivado, está em oracle_independence_test.go.
func TestRedControlOraculoNaoETautologico(t *testing.T) {
	// Célula 4: se a suite aceitar a matriz corrompida, o oráculo é tautológico.
	original := matrix[BlockDomain]
	t.Cleanup(func() { matrix[BlockDomain] = original })

	corrompida := original
	iPort, ok := blockIndex(BlockPort)
	if !ok {
		t.Fatal("bloco port ausente do conjunto fechado")
	}
	corrompida[iPort] = true
	matrix[BlockDomain] = corrompida

	if !AllowedByMatrix(BlockDomain, BlockPort) {
		t.Fatal("corrupção não teve efeito: o teste de controle não prova nada")
	}

	var divergiu bool
	for _, c := range oracle {
		if AllowedByMatrix(c.source, c.target) != c.allow {
			divergiu = true
			break
		}
	}
	if !divergiu {
		t.Fatal("matriz corrompida na célula 4 e o oráculo não acusou: a tabela de §7.4 está derivada de matrix.go, não transcrita")
	}
}

// Corromper QUALQUER uma das 36 posições precisa ser detectado.
func TestRedControlCobreTodasAsCelulas(t *testing.T) {
	for _, c := range oracle {
		original := matrix[c.source]
		j, ok := blockIndex(c.target)
		if !ok {
			t.Fatalf("célula %d: bloco de destino %s fora do conjunto fechado", c.n, c.target)
		}

		corrompida := original
		corrompida[j] = !original[j]
		matrix[c.source] = corrompida

		got := AllowedByMatrix(c.source, c.target)
		matrix[c.source] = original

		if got == c.allow {
			t.Errorf("célula %d (%s -> %s): inversão do dado não mudou a decisão", c.n, c.source, c.target)
		}
	}
}
