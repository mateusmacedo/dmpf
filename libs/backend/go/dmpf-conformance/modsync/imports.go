package modsync

import (
	"fmt"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func importsDoModulo(raiz string) (map[string]bool, error) {
	imports := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(raiz, func(caminho string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if caminho != raiz && diretorioIgnorado(caminho, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		nome := d.Name()
		if !strings.HasSuffix(nome, ".go") || strings.HasPrefix(nome, ".") || strings.HasPrefix(nome, "_") {
			return nil
		}
		return absorverImports(fset, caminho, imports)
	})
	return imports, err
}

func diretorioIgnorado(caminho, nome string) bool {
	if nome == "testdata" || nome == "vendor" || strings.HasPrefix(nome, ".") || strings.HasPrefix(nome, "_") {
		return true
	}
	_, err := os.Stat(filepath.Join(caminho, "go.mod"))
	return err == nil
}

func absorverImports(fset *token.FileSet, caminho string, imports map[string]bool) error {
	fonte, err := os.ReadFile(caminho)
	if err != nil {
		return err
	}
	entra, err := entraNoTidy(fonte)
	if err != nil {
		return fmt.Errorf("%s: %w", caminho, err)
	}
	if !entra {
		return nil
	}
	arquivo, err := parser.ParseFile(fset, caminho, fonte, parser.ImportsOnly)
	if err != nil {
		return err
	}
	for _, spec := range arquivo.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return fmt.Errorf("%s: %w", caminho, err)
		}
		imports[importPath] = true
	}
	return nil
}

func entraNoTidy(fonte []byte) (bool, error) {
	var goBuild constraint.Expr
	var plusBuild []constraint.Expr
	for linha := range strings.Lines(string(fonte)) {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		if !strings.HasPrefix(linha, "//") {
			break
		}
		if !constraint.IsGoBuild(linha) && !constraint.IsPlusBuild(linha) {
			continue
		}
		expr, err := constraint.Parse(linha)
		if err != nil {
			return false, err
		}
		if constraint.IsGoBuild(linha) {
			goBuild = expr
		} else {
			plusBuild = append(plusBuild, expr)
		}
	}
	switch {
	case goBuild != nil:
		return compilaComAlgumaTag(goBuild, true), nil
	case len(plusBuild) > 0:
		return !slices.ContainsFunc(plusBuild, func(x constraint.Expr) bool { return !compilaComAlgumaTag(x, true) }), nil
	}
	return true, nil
}

// WHY: mesma avaliação do go mod tidy (cmd/go/internal/imports, AnyTags): toda
// tag vale presente e ausente, menos ignore, que nunca vale.
func compilaComAlgumaTag(x constraint.Expr, prefer bool) bool {
	switch x := x.(type) {
	case *constraint.TagExpr:
		return x.Tag != "ignore" && prefer
	case *constraint.NotExpr:
		return !compilaComAlgumaTag(x.X, !prefer)
	case *constraint.AndExpr:
		return compilaComAlgumaTag(x.X, prefer) && compilaComAlgumaTag(x.Y, prefer)
	case *constraint.OrExpr:
		return compilaComAlgumaTag(x.X, prefer) || compilaComAlgumaTag(x.Y, prefer)
	}
	return false
}

func irmaosImportados(imports map[string]bool, modules []Module, proprio string) []string {
	paths := make([]string, 0, len(modules))
	for _, m := range modules {
		paths = append(paths, m.Path)
	}
	achados := map[string]bool{}
	for importPath := range imports {
		if dono := donoDoImport(importPath, paths); dono != "" && dono != proprio {
			achados[dono] = true
		}
	}
	return slices.Sorted(maps.Keys(achados))
}

func importsExternos(imports map[string]bool, modules []Module) []string {
	paths := make([]string, 0, len(modules))
	for _, m := range modules {
		paths = append(paths, m.Path)
	}
	var externos []string
	for importPath := range imports {
		if !daBibliotecaPadrao(importPath) && donoDoImport(importPath, paths) == "" {
			externos = append(externos, importPath)
		}
	}
	slices.Sort(externos)
	return externos
}

func daBibliotecaPadrao(importPath string) bool {
	primeiro, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(primeiro, ".")
}

func donoDoImport(importPath string, modulePaths []string) string {
	dono := ""
	for _, p := range modulePaths {
		if (importPath == p || strings.HasPrefix(importPath, p+"/")) && len(p) > len(dono) {
			dono = p
		}
	}
	return dono
}
