package rule_test

import (
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

func cenario(nMod, pkgPorMod int) ([]rule.Unit, []rule.Package, []rule.Module) {
	var units []rule.Unit
	var pkgs []rule.Package
	var mods []rule.Module
	for m := range nMod {
		mod := fmt.Sprintf("org/mod-%d", m)
		mods = append(mods, rule.Module{Path: mod, HasManifest: true, HasProduction: true})
		for p := range pkgPorMod {
			key := fmt.Sprintf("%s/pkg-%d", mod, p)
			units = append(units, rule.Unit{
				ID: fmt.Sprintf("u-%d", p), Block: rule.BlockDomain,
				BoundedContext: mod, Include: []string{key}, Module: mod,
			})
			pkgs = append(pkgs, rule.Package{CanonicalKey: key, Module: mod})
		}
	}
	return units, pkgs, mods
}

func BenchmarkBuildUniverse(b *testing.B) {
	for _, c := range []struct{ mods, pkgs int }{{12, 10}, {50, 20}, {200, 25}} {
		units, packages, modules := cenario(c.mods, c.pkgs)
		b.Run(fmt.Sprintf("%dmod_x_%dpkg=%dunidades", c.mods, c.pkgs, len(units)), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, ds := rule.BuildUniverse(units, packages, modules)
				if len(ds) != 0 {
					b.Fatalf("cenário deveria ser limpo: %v", ds[:1])
				}
			}
		})
	}
}
