package domain

import "exemplo.test/ai-b/provider"

// Aresta domain -> provider entre módulos, proibida pela matriz. Os dois lados
// são unidades declaradas, então tem de sair diagnóstico de aresta, não de
// dependência externa.
func Usar() string { return provider.P() }
