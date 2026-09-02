// Command dmpf-conformance decide a regra de dependência do DMPF sobre o grafo
// real de imports do workspace e reprova o PR quando encontra violação.
//
// Este é o bloco `app`: o composition root, a única unidade que instancia
// providers concretos. Toda decisão vive no domínio; aqui só se monta o grafo
// de objetos, roda e imprime.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/conformance"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/fsstore"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/golist"
)

// Os três códigos de saída são distintos por necessidade: falha de execução
// NUNCA pode ser lida como conformidade por quem só olha "diferente de zero",
// nem confundida com reprovação por quem investiga o log.
const (
	exitConforme  = 0
	exitReprovado = 1
	exitFalha     = 2
)

func main() {
	raiz := flag.String("root", ".", "raiz do workspace")
	perfis := flag.String("profiles", "", "caminho do build-profiles.json (default: <root>/libs/backend/go/dmpf-conformance/build-profiles.json)")
	flag.Parse()

	os.Exit(run(*raiz, *perfis, os.Stdout, os.Stderr))
}

func run(raiz, perfis string, saida, erros io.Writer) int {
	relatorio, err := verificar(raiz, perfis)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-conformance: %v\n", err)
		return exitFalha
	}

	// bufio.Writer retém o primeiro erro de escrita e o devolve no Flush
	// (bufio.Writer.Write, doc do pacote), então checar cada Fprint seria
	// redundante — o Flush abaixo é o único ponto que precisa verificar.
	w := bufio.NewWriter(saida)
	for _, d := range relatorio.Diagnostics {
		_, _ = fmt.Fprintln(w, d.String())
	}
	_, _ = fmt.Fprintf(w, "\n%s\n", relatorio.Resumo())
	if err := w.Flush(); err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-conformance: escrever relatório: %v\n", err)
		return exitFalha
	}

	if len(relatorio.Diagnostics) > 0 {
		return exitReprovado
	}
	return exitConforme
}

// verificar monta os providers e executa a verificação.
func verificar(raiz, perfis string) (conformance.Report, error) {
	abs, err := filepath.Abs(raiz)
	if err != nil {
		return conformance.Report{}, err
	}
	if perfis == "" {
		perfis = filepath.Join(abs, "libs", "backend", "go", "dmpf-conformance", "build-profiles.json")
	}

	profiles, err := fsstore.LoadBuildProfiles(perfis)
	if err != nil {
		return conformance.Report{}, err
	}

	modules, err := fsstore.NewInventory(abs).Modules()
	if err != nil {
		return conformance.Report{}, err
	}
	if len(modules) == 0 {
		return conformance.Report{}, fmt.Errorf("nenhum módulo Go encontrado a partir de %s", abs)
	}

	grafo := golist.New(abs, modules, profiles)
	return conformance.Check(conformance.Input{
		Modules:   modules,
		Manifests: fsstore.NewManifestStore(fsstore.ModuleDirs(modules)),
		Graph:     grafo,
		Closure:   grafo.Closure,
		Standard:  grafo.IsStandard,
	})
}
