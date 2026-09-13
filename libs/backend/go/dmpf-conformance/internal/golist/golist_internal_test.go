package golist

import (
	"slices"
	"testing"
)

// Package só de arquivos que importam "C" tem GoFiles vazio: olhar só GoFiles
// o faria sumir do universo, e sumir é a única forma de escapar do U001.
//
// Testa a projeção, não um build cgo real, que amarraria a suite ao ambiente.
func TestArquivosDeProducaoIncluiCgo(t *testing.T) {
	casos := []struct {
		nome string
		p    listPackage
		want []string
	}{
		{"só Go", listPackage{GoFiles: []string{"a.go"}}, []string{"a.go"}},
		{"só cgo", listPackage{CgoFiles: []string{"c.go"}}, []string{"c.go"}},
		{"mistos", listPackage{GoFiles: []string{"a.go"}, CgoFiles: []string{"c.go"}}, []string{"a.go", "c.go"}},
		{"nenhum", listPackage{}, nil},
		{"ignorados não contam", listPackage{IgnoredGoFiles: []string{"i.go"}}, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := c.p.arquivosDeProducao()
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("arquivosDeProducao() = %v, esperado %v", got, c.want)
			}
		})
	}
}

// O filtro que decide a entrada no universo precisa enxergar o só-cgo.
func TestPackageSoComCgoEProducao(t *testing.T) {
	if len(listPackage{CgoFiles: []string{"c.go"}}.arquivosDeProducao()) == 0 {
		t.Error("package só-cgo tratado como sem código de produção: ele sumiria do universo")
	}
}
