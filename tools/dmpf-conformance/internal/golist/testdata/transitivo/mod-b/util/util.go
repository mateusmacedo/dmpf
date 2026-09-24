package util

import "net/http"

// Buscar usa io.network. Declarado como `domain` no manifesto do mod-b, é a
// violação que só aparece quando o verificador atravessa a fronteira de módulo.
func Buscar() string { return http.MethodGet }
