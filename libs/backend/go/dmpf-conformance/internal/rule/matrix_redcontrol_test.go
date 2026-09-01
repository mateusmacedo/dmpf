package rule

import "testing"

// TestRedControlOraculoNaoETautologico é a guarda contra o oráculo derivado.
//
// Se a tabela de RFC §7.4 fosse computada a partir de matrix.go, corromper o
// dado de produção não faria a suite falhar — os dois lados mudariam juntos e o
// teste passaria com qualquer matriz. Este controle corrompe uma célula e exige
// que a comparação REPROVE.
func TestRedControlOraculoNaoETautologico(t *testing.T) {
	// Célula 4 (domain → port): proibida sem exceção por ADR-014. Se a suite
	// aceitar a matriz corrompida, o oráculo é tautológico.
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

// TestRedControlCobreTodasAsCelulas estende o controle: corromper QUALQUER uma
// das 36 posições precisa ser detectado, não só a célula 4.
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
