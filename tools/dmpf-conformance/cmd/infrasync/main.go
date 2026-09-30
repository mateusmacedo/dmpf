// Command dmpf-infrasync grava ou confere os arquivos de infra/ que agregam as
// apps, derivados do manifesto deploy/infra.json de cada uma.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/infrasync"
)

const (
	exitConforme  = 0
	exitReprovado = 1
	exitFalha     = 2
)

func main() {
	raiz := flag.String("root", ".", "raiz do workspace")
	gravar := flag.Bool("write", false, "grava os arquivos gerados a partir dos manifestos")
	conferir := flag.Bool("check", false, "confere os arquivos gerados sem alterar e reprova divergência")
	flag.Parse()

	os.Exit(run(opcoes{raiz: *raiz, gravar: *gravar, conferir: *conferir}, os.Stdout, os.Stderr))
}

type opcoes struct {
	raiz     string
	gravar   bool
	conferir bool
}

func run(o opcoes, saida, erros io.Writer) int {
	if o.gravar == o.conferir {
		_, _ = fmt.Fprintln(erros, "dmpf-infrasync: use exatamente um entre --write e --check")
		return exitFalha
	}
	manifests, err := infrasync.Load(o.raiz)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-infrasync: %v\n", err)
		return exitFalha
	}
	files, err := infrasync.Render(o.raiz, manifests)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-infrasync: %v\n", err)
		return exitFalha
	}
	if o.gravar {
		if err := infrasync.Write(o.raiz, files); err != nil {
			_, _ = fmt.Fprintf(erros, "dmpf-infrasync: %v\n", err)
			return exitFalha
		}
		_, _ = fmt.Fprintf(saida, "dmpf-infrasync: %d arquivo(s) gerado(s) de %d manifesto(s)\n", len(files), len(manifests))
		return exitConforme
	}

	findings := append(infrasync.Check(o.raiz, files), infrasync.CheckDevSecrets(o.raiz, manifests)...)
	w := bufio.NewWriter(saida)
	for _, f := range findings {
		_, _ = fmt.Fprintln(w, f.String())
	}
	if len(findings) == 0 {
		_, _ = fmt.Fprintln(w, "dmpf-infrasync: conforme")
	} else {
		_, _ = fmt.Fprintf(w, "\ndmpf-infrasync: REPROVADO com %d divergência(s)\n", len(findings))
	}
	if err := w.Flush(); err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-infrasync: escrever relatório: %v\n", err)
		return exitFalha
	}
	if len(findings) > 0 {
		return exitReprovado
	}
	return exitConforme
}
