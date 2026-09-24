package bom

import (
	"cmp"
	"slices"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

// rule.SortDiagnostics põe a chave antes do código; o relatório do dmpf-bom
// agrupa por código, na ordem da tabela de B001–B011.
func SortReport(ds []rule.Diagnostic) {
	slices.SortStableFunc(ds, func(a, b rule.Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.CanonicalKey, b.CanonicalKey),
			cmp.Compare(a.Target, b.Target),
			cmp.Compare(a.Detail, b.Detail),
		)
	})
}
