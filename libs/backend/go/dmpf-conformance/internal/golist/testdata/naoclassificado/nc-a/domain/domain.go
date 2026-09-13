package domain

import "exemplo.test/nc-a/orfao"

// Usar aponta para um package do universo que não está classificado. O destino
// é interno, não dependência externa — a diferença que o E001 espúrio confundia.
func Usar() string { return orfao.Orfao() }
