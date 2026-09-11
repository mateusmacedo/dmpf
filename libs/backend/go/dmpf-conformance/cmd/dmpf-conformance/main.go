// Command dmpf-conformance decide a regra de dependência do DMPF sobre o grafo
// real de imports e reprova o PR quando encontra violação.
//
// Composition root: a única unidade que instancia providers concretos.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/fsstore"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/golist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

// Três códigos distintos: falha de execução nunca pode ser lida como
// conformidade por quem só olha "diferente de zero".
const (
	exitConforme  = 0
	exitReprovado = 1
	exitFalha     = 2
)

func main() {
	raiz := flag.String("root", ".", "raiz do workspace")
	perfis := flag.String("profiles", "", "caminho do build-profiles.json (default: <root>/libs/backend/go/dmpf-conformance/build-profiles.json)")
	base := flag.String("base", "", "ref base do intervalo em revisão, para avaliar o commit próprio de RFC §10.2")
	regravar := flag.Bool("write-baseline", false, "regrava o baseline a partir da classificação declarada; NUNCA usar no gate")
	flag.Parse()

	os.Exit(run(opcoes{
		raiz: *raiz, perfis: *perfis, base: *base, regravar: *regravar,
	}, os.Stdout, os.Stderr))
}

type opcoes struct {
	raiz     string
	perfis   string
	base     string
	regravar bool
}

func run(o opcoes, saida, erros io.Writer) int {
	if o.regravar {
		if err := regravarBaseline(o); err != nil {
			_, _ = fmt.Fprintf(erros, "dmpf-conformance: %v\n", err)
			return exitFalha
		}
		_, _ = fmt.Fprintf(saida, "baseline regravado em %s\n", baseline.Path)
		return exitConforme
	}

	relatorio, err := verificar(o)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-conformance: %v\n", err)
		return exitFalha
	}

	// bufio.Writer retém o primeiro erro e o devolve no Flush, que é o único
	// ponto que precisa verificar.
	w := bufio.NewWriter(saida)
	for _, d := range relatorio.Diagnostics {
		_, _ = fmt.Fprintln(w, d.String())
	}
	for _, n := range relatorio.NaoVerificado {
		// Mesmo peso dos diagnósticos: escondê-la faria o gate informar
		// cobertura que não tem.
		_, _ = fmt.Fprintf(w, "NAO VERIFICADO: %s\n", n)
	}
	_, _ = fmt.Fprintf(w, "\n%s\n", relatorio.Resumo())
	if err := w.Flush(); err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-conformance: escrever relatório: %v\n", err)
		return exitFalha
	}

	if relatorio.Reprovado() {
		return exitReprovado
	}
	return exitConforme
}

func verificar(o opcoes) (conformance.Report, error) {
	abs, perfis, err := preparar(o)
	if err != nil {
		return conformance.Report{}, err
	}
	modules, grafo, err := montar(abs, perfis)
	if err != nil {
		return conformance.Report{}, err
	}
	return conformance.Check(conformance.Input{
		Modules:   modules,
		Manifests: fsstore.NewManifestStore(fsstore.ModuleDirs(modules)),
		Graph:     grafo,
		Closure:   grafo.Closure,
		Standard:  grafo.IsStandard,
		Baseline:  fsstore.NewBaselineStore(abs),
		Base:      o.base,
	})
}

func preparar(o opcoes) (string, string, error) {
	abs, err := filepath.Abs(o.raiz)
	if err != nil {
		return "", "", err
	}
	perfis := o.perfis
	if perfis == "" {
		perfis = filepath.Join(abs, "libs", "backend", "go", "dmpf-conformance", "build-profiles.json")
	}
	return abs, perfis, nil
}

func montar(abs, perfis string) ([]rule.Module, *golist.Source, error) {
	profiles, err := fsstore.LoadBuildProfiles(perfis)
	if err != nil {
		return nil, nil, err
	}
	modules, err := fsstore.NewInventory(abs).Modules()
	if err != nil {
		return nil, nil, err
	}
	if len(modules) == 0 {
		return nil, nil, fmt.Errorf("nenhum módulo Go encontrado a partir de %s", abs)
	}
	return modules, golist.New(abs, modules, profiles), nil
}

// Comando SEPARADO, que nunca roda no gate: um gate que conserta o próprio
// insumo deixa de detectar a divergência que existe para detectar.
func regravarBaseline(o opcoes) error {
	abs, perfis, err := preparar(o)
	if err != nil {
		return err
	}
	modules, grafo, err := montar(abs, perfis)
	if err != nil {
		return err
	}
	docs, err := fsstore.NewManifestStore(fsstore.ModuleDirs(modules)).Documents()
	if err != nil {
		return err
	}
	pkgs, err := grafo.Packages()
	if err != nil {
		return err
	}

	var units []rule.Unit
	for _, doc := range docs {
		if d := manifest.Validate(doc); len(d) > 0 {
			return fmt.Errorf("manifesto inválido em %s: %s", doc.Path, d[0])
		}
		units = append(units, conformance.UnidadesDoDocumento(doc)...)
	}
	universo, diags := rule.BuildUniverse(units, pkgs, modules)
	if len(diags) > 0 {
		return fmt.Errorf("universo inválido: %s", diags[0])
	}

	store := fsstore.NewBaselineStore(abs)
	atual, existia, err := store.Baseline()
	if err != nil {
		return fmt.Errorf("ler baseline atual: %w", err)
	}
	return store.Escrever(baseline.Regravar(atual, existia, units, universo.Membership()))
}
