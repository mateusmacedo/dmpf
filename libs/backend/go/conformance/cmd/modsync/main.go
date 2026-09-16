// Command dmpf-modsync grava ou confere, em cada módulo do go.work, o require e
// o replace para os módulos irmãos derivados dos imports reais.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/modsync"
)

const (
	exitConforme  = 0
	exitReprovado = 1
	exitFalha     = 2
)

func main() {
	raiz := flag.String("root", ".", "raiz do workspace (diretório do go.work)")
	gravar := flag.Bool("write", false, "grava require e replace dos irmãos nos go.mod")
	conferir := flag.Bool("check", false, "confere os go.mod sem alterar e reprova divergência")
	flag.Parse()

	os.Exit(run(context.Background(), opcoes{raiz: *raiz, gravar: *gravar, conferir: *conferir}, os.Stdout, os.Stderr))
}

type opcoes struct {
	raiz     string
	gravar   bool
	conferir bool
}

func run(ctx context.Context, o opcoes, saida, erros io.Writer) int {
	if o.gravar == o.conferir {
		_, _ = fmt.Fprintln(erros, "dmpf-modsync: use exatamente um entre --write e --check")
		return exitFalha
	}
	plan, err := modsync.Derive(ctx, o.raiz)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-modsync: %v\n", err)
		return exitFalha
	}
	if o.gravar {
		if err := modsync.Write(ctx, o.raiz, plan); err != nil {
			_, _ = fmt.Fprintf(erros, "dmpf-modsync: %v\n", err)
			return exitFalha
		}
		_, _ = fmt.Fprintf(saida, "dmpf-modsync: %d módulo(s) e %d replace(s) no go.work sincronizados\n", len(plan.Modules), len(plan.Replaces))
		return exitConforme
	}

	findings, err := modsync.Check(ctx, o.raiz, plan)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-modsync: %v\n", err)
		return exitFalha
	}
	w := bufio.NewWriter(saida)
	for _, f := range findings {
		_, _ = fmt.Fprintln(w, f.String())
	}
	if len(findings) == 0 {
		_, _ = fmt.Fprintln(w, "dmpf-modsync: conforme")
	} else {
		_, _ = fmt.Fprintf(w, "\ndmpf-modsync: REPROVADO com %d divergência(s)\n", len(findings))
	}
	if err := w.Flush(); err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-modsync: escrever relatório: %v\n", err)
		return exitFalha
	}
	if len(findings) > 0 {
		return exitReprovado
	}
	return exitConforme
}
