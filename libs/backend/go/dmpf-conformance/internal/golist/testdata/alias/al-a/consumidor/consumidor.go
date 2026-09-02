package consumidor

import (
	apelido "exemplo.test/al-a/alvo"
	direto "exemplo.test/al-a/alvo"
)

// O mesmo package importado com dois nomes locais. A aresta é sobre o package
// que o compilador resolveu, não sobre o nome escrito, então as duas formas
// produzem uma aresta só.
func C() string { return apelido.A() + direto.A() }
