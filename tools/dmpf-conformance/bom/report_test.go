package bom

import (
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

func TestRelatorioOrdenaCodigoAntesDoCaminho(t *testing.T) {
	ds := []rule.Diagnostic{
		{Code: rule.CodeB002, CanonicalKey: "a"},
		{Code: rule.CodeB001, CanonicalKey: "z"},
		{Code: rule.CodeB001, CanonicalKey: "b"},
	}
	SortReport(ds)

	want := []rule.Diagnostic{
		{Code: rule.CodeB001, CanonicalKey: "b"},
		{Code: rule.CodeB001, CanonicalKey: "z"},
		{Code: rule.CodeB002, CanonicalKey: "a"},
	}
	for i := range want {
		if ds[i] != want[i] {
			t.Fatalf("ordem %v, esperado %v", ds, want)
		}
	}
}
