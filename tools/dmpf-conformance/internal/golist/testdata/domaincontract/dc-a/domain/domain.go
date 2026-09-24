package domain

import "exemplo.test/dc-b/wire"

// Aresta domain -> contract entre módulos: célula 6 da matriz, proibida por P0-2
// (o tipo de wire nunca é modelo interno), ainda que contract seja superfície pública.
func Usar() string { return wire.TypeURL }
