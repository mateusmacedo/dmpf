// Package modsync deriva dos imports reais o require que cada módulo do go.work
// declara e o bloco replace versionado do go.work, e grava ou confere esse estado.
package modsync

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

const versaoInicial = "v0.1.0"

type Module struct {
	Path string
	Dir  string
}

type Requirement struct {
	Path    string
	Version string
}

type Replacement struct {
	Path    string
	Version string
	Dir     string
}

type ModulePlan struct {
	Module   Module
	Siblings []Requirement
	externos []string
}

type Plan struct {
	Modules  []ModulePlan
	Replaces []Replacement
}

type Finding struct {
	Module string
	Detail string
}

func (f Finding) String() string {
	return f.Module + ": " + f.Detail
}

const ondeGoWork = "go.work"

func Derive(ctx context.Context, root string) (Plan, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Plan{}, err
	}
	if err := recusarCloneRaso(ctx, abs); err != nil {
		return Plan{}, err
	}
	modules, err := modulosDoWorkspace(ctx, abs)
	if err != nil {
		return Plan{}, err
	}
	versoes, err := versoesPorTag(ctx, abs, modules)
	if err != nil {
		return Plan{}, err
	}
	porPath := make(map[string]Module, len(modules))
	for _, m := range modules {
		porPath[m.Path] = m
	}

	var plan Plan
	replaces := map[Replacement]bool{}
	for _, m := range modules {
		dir := filepath.Join(abs, filepath.FromSlash(m.Dir))
		imports, err := importsDoModulo(dir)
		if err != nil {
			return Plan{}, fmt.Errorf("%s: %w", m.Dir, err)
		}
		atual, err := lerGoMod(ctx, dir)
		if err != nil {
			return Plan{}, fmt.Errorf("%s: %w", m.Dir, err)
		}
		declarados := atual.versoesRequeridas()
		mp := ModulePlan{Module: m, externos: importsExternos(imports, modules)}
		for _, irmao := range irmaosImportados(imports, modules, m.Path) {
			versao, ok := declarados[irmao]
			if !ok {
				versao = versoes[irmao]
			}
			mp.Siblings = append(mp.Siblings, Requirement{Path: irmao, Version: versao})
			declarados[irmao] = versao
		}
		for path, versao := range declarados {
			if irmao, ok := porPath[path]; ok && path != m.Path {
				replaces[Replacement{Path: path, Version: versao, Dir: "./" + irmao.Dir}] = true
			}
		}
		plan.Modules = append(plan.Modules, mp)
	}
	plan.Replaces = slices.SortedFunc(maps.Keys(replaces), func(a, b Replacement) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Version, b.Version))
	})
	return plan, nil
}

func Write(ctx context.Context, root string, plan Plan) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, mp := range plan.Modules {
		atual, err := lerGoMod(ctx, filepath.Join(abs, filepath.FromSlash(mp.Module.Dir)))
		if err != nil {
			return fmt.Errorf("%s: %w", mp.Module.Dir, err)
		}
		declarados := atual.versoesRequeridas()
		var faltantes []Requirement
		for _, s := range mp.Siblings {
			if _, ok := declarados[s.Path]; !ok {
				faltantes = append(faltantes, s)
			}
		}
		if err := requerer(ctx, abs, mp.Module, faltantes); err != nil {
			return err
		}
	}
	if err := gravarGoWork(ctx, abs, plan); err != nil {
		return err
	}

	lista, err := buildList(ctx, abs)
	if err != nil {
		return fmt.Errorf("build list do workspace: %w", err)
	}
	for _, mp := range plan.Modules {
		atual, err := lerGoMod(ctx, filepath.Join(abs, filepath.FromSlash(mp.Module.Dir)))
		if err != nil {
			return fmt.Errorf("%s: %w", mp.Module.Dir, err)
		}
		faltantes, semDono := externosSemRequire(mp, atual.versoesRequeridas(), lista)
		if len(semDono) > 0 {
			return fmt.Errorf("%s: import sem módulo na build list do workspace, rode go get antes: %s", mp.Module.Dir, strings.Join(semDono, ", "))
		}
		if err := requerer(ctx, abs, mp.Module, faltantes); err != nil {
			return err
		}
	}
	return nil
}

func requerer(ctx context.Context, abs string, m Module, requires []Requirement) error {
	if len(requires) == 0 {
		return nil
	}
	args := []string{"mod", "edit"}
	for _, r := range requires {
		args = append(args, "-require="+r.Path+"@"+r.Version)
	}
	if _, err := executar(ctx, filepath.Join(abs, filepath.FromSlash(m.Dir)), "go", args...); err != nil {
		return fmt.Errorf("%s: %w", m.Dir, err)
	}
	return nil
}

func Check(ctx context.Context, root string, plan Plan) ([]Finding, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	declaradosPorModulo := make(map[string]map[string]string, len(plan.Modules))
	for _, mp := range plan.Modules {
		atual, err := lerGoMod(ctx, filepath.Join(abs, filepath.FromSlash(mp.Module.Dir)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", mp.Module.Dir, err)
		}
		declarados := atual.versoesRequeridas()
		declaradosPorModulo[mp.Module.Dir] = declarados
		for _, s := range mp.Siblings {
			if _, ok := declarados[s.Path]; !ok {
				findings = append(findings, Finding{Module: mp.Module.Dir, Detail: fmt.Sprintf("falta require %s %s", s.Path, s.Version)})
			}
		}
		for _, r := range atual.Replace {
			findings = append(findings, Finding{
				Module: mp.Module.Dir,
				Detail: fmt.Sprintf("replace %s => %s no go.mod: replace fica só no go.work", r.Old.chave(), strings.TrimSpace(r.New.Path+" "+r.New.Version)),
			})
		}
	}

	divergencias, err := divergenciasDoGoWork(ctx, abs, plan)
	if err != nil {
		return nil, err
	}
	findings = append(findings, divergencias...)

	lista, err := buildList(ctx, abs)
	if err != nil {
		if len(divergencias) == 0 {
			return nil, fmt.Errorf("build list do workspace: %w", err)
		}
		return ordenar(findings), nil
	}
	for _, mp := range plan.Modules {
		faltantes, semDono := externosSemRequire(mp, declaradosPorModulo[mp.Module.Dir], lista)
		for _, r := range faltantes {
			findings = append(findings, Finding{Module: mp.Module.Dir, Detail: fmt.Sprintf("falta require %s %s", r.Path, r.Version)})
		}
		for _, imp := range semDono {
			findings = append(findings, Finding{Module: mp.Module.Dir, Detail: fmt.Sprintf("import %s sem módulo na build list do workspace", imp)})
		}
	}
	return ordenar(findings), nil
}

func divergenciasDoGoWork(ctx context.Context, abs string, plan Plan) ([]Finding, error) {
	work, err := lerGoWork(ctx, abs)
	if err != nil {
		return nil, err
	}
	esperados := make(map[Replacement]bool, len(plan.Replaces))
	for _, r := range plan.Replaces {
		esperados[r] = true
	}
	var findings []Finding
	presentes := map[Replacement]bool{}
	for _, r := range work.Replace {
		atual := Replacement{Path: r.Old.Path, Version: r.Old.Version, Dir: r.New.Path}
		presentes[atual] = true
		if !esperados[atual] || r.New.Version != "" {
			findings = append(findings, Finding{
				Module: ondeGoWork,
				Detail: fmt.Sprintf("replace %s => %s sobrando no go.work", r.Old.chave(), strings.TrimSpace(r.New.Path+" "+r.New.Version)),
			})
		}
	}
	for _, r := range plan.Replaces {
		if !presentes[r] {
			findings = append(findings, Finding{Module: ondeGoWork, Detail: fmt.Sprintf("falta replace %s %s => %s no go.work", r.Path, r.Version, r.Dir)})
		}
	}
	return findings, nil
}

func externosSemRequire(mp ModulePlan, declarados map[string]string, lista map[string]string) ([]Requirement, []string) {
	faltantes := map[string]bool{}
	var semDono []string
	for _, imp := range mp.externos {
		dono := donoDoImport(imp, slices.Collect(maps.Keys(lista)))
		switch {
		case dono == "":
			semDono = append(semDono, imp)
		case declarados[dono] == "":
			faltantes[dono] = true
		}
	}
	var requires []Requirement
	for _, path := range slices.Sorted(maps.Keys(faltantes)) {
		requires = append(requires, Requirement{Path: path, Version: lista[path]})
	}
	return requires, semDono
}

func ordenar(findings []Finding) []Finding {
	slices.SortFunc(findings, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.Module, b.Module), cmp.Compare(a.Detail, b.Detail))
	})
	return findings
}
