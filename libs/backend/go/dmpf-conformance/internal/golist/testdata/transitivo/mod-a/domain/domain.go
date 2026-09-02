package domain

import "exemplo.test/mod-b/util"

// Saudar exercita a aresta entre módulos: o `domain` do mod-a alcança
// `net/http` por dentro do mod-b. O golangci-lint do KRN-01 analisa um módulo
// por vez e não vê isso.
func Saudar() string { return util.Buscar() }
