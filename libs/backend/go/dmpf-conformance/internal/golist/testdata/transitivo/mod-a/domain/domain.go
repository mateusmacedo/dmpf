package domain

import "exemplo.test/mod-b/util"

// O `domain` do mod-a alcança `net/http` por dentro do mod-b. Um lint que
// analisa um módulo por vez não vê isso.
func Saudar() string { return util.Buscar() }
