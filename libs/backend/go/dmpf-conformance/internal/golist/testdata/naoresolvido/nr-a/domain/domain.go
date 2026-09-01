package domain

import _ "exemplo.test/modulo/que/nao/existe"

// Este package importa um módulo inexistente. O `go list -e` devolve o alvo com
// Error preenchido, e o verificador tem de emitir DMPF-E003 — nunca tratar o
// import como ausente.
func F() string { return "x" }
