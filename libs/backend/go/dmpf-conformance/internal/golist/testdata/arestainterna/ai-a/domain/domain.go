package domain

import "exemplo.test/ai-b/provider"

// Usar cria a aresta domain -> provider entre módulos: célula 5 da matriz,
// proibida por P0-1. É aresta INTERNA (os dois lados são unidades declaradas),
// então tem de sair D001 — não E001.
func Usar() string { return provider.P() }
