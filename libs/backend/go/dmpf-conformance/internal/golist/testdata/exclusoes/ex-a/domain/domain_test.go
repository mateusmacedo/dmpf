package domain

import (
	_ "net/http"
	"testing"
)

// Arquivo de teste importando rede dentro de uma unidade de domínio. Isso não
// pode reprovar: teste não altera a classificação do código sob teste, e a
// exclusão é fechada — se reprovasse, todo módulo com teste de integração
// ficaria impedido de ter domínio.
func TestNaoAlteraClassificacao(t *testing.T) { _ = D() }
