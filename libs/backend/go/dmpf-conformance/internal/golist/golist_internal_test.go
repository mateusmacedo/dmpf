package golist

import (
	"slices"
	"testing"
)

// TestArquivosDeProducaoIncluiCgo: `go list` separa GoFiles de CgoFiles, e um
// package composto só de arquivos que importam "C" tem GoFiles vazio. Olhar só
// GoFiles o faria sumir do universo num perfil com CGO_ENABLED=1 — e sumir é a
// única forma de escapar do DMPF-U001.
//
// O teste é sobre a projeção, não sobre um build cgo real: o perfil inicial tem
// cgo desligado, e exigir toolchain C aqui amarraria a suite ao ambiente.
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

// TestPackageSoComCgoEProducao fecha a consequência: o filtro que decide se o
// package entra no universo precisa enxergar o package só-cgo.
func TestPackageSoComCgoEProducao(t *testing.T) {
	if len(listPackage{CgoFiles: []string{"c.go"}}.arquivosDeProducao()) == 0 {
		t.Error("package só-cgo tratado como sem código de produção: ele sumiria do universo")
	}
}
