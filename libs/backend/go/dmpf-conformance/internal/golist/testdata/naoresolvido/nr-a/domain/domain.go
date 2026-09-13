package domain

import _ "exemplo.test/modulo/que/nao/existe"

// Importa um módulo inexistente: o verificador tem de reprovar, nunca tratar o
// import como ausente.
func F() string { return "x" }
