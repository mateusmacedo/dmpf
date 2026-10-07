package golist

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/fsstore"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/port"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

var _ port.KernelSource = (*Source)(nil)

// Um módulo fora do inventário sem manifesto continua dependência externa,
// julgada por capability.
func (s *Source) Kernel() (port.Kernel, error) {
	if err := s.carregar(); err != nil {
		return port.Kernel{}, err
	}
	if len(s.buildListFalhou) > 0 {
		motivos := slices.Sorted(maps.Keys(s.buildListFalhou))
		return port.Kernel{}, fmt.Errorf("build list não carregada pelo toolchain: %s", strings.Join(motivos, "; "))
	}

	var k port.Kernel
	comManifesto := map[string]bool{}
	for _, m := range s.modulosForaDoInventario() {
		doc, existe, err := fsstore.LoadVersionedManifest(m.Path, m.Version, m.Dir)
		if err != nil {
			return port.Kernel{}, err
		}
		if !existe {
			continue
		}
		k.Documents = append(k.Documents, doc)
		comManifesto[m.Path] = true
	}
	for _, p := range s.pacotes {
		if p.Module == nil || !comManifesto[p.Module.Path] || len(p.arquivosDeProducao()) == 0 {
			continue
		}
		k.Packages = append(k.Packages, rule.Package{CanonicalKey: p.ImportPath, Module: p.Module.Path})
	}
	sort.Slice(k.Packages, func(a, b int) bool { return k.Packages[a].CanonicalKey < k.Packages[b].CanonicalKey })
	return k, nil
}

func (s *Source) modulosForaDoInventario() []listModule {
	porPath := map[string]listModule{}
	for _, p := range s.pacotes {
		if p.Standard || p.Module == nil || p.Module.Main || p.Module.Dir == "" || s.moduloDoInventario(p.Module.Path) {
			continue
		}
		porPath[p.Module.Path] = *p.Module
	}
	out := make([]listModule, 0, len(porPath))
	for _, path := range slices.Sorted(maps.Keys(porPath)) {
		out = append(out, porPath[path])
	}
	return out
}
