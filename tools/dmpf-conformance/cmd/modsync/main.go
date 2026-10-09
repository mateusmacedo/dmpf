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
	"strings"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/modsync"
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
	var requires listaDeRequires
	flag.Var(&requires, "require", "<módulo>@<versão> que dá dono a import fora da build list; repetível, só com --write")
	flag.Parse()

	os.Exit(run(context.Background(), opcoes{raiz: *raiz, gravar: *gravar, conferir: *conferir, requires: requires}, os.Stdout, os.Stderr))
}

type listaDeRequires []string

func (l *listaDeRequires) String() string     { return strings.Join(*l, ",") }
func (l *listaDeRequires) Set(v string) error { *l = append(*l, v); return nil }

type opcoes struct {
	raiz     string
	gravar   bool
	conferir bool
	requires []string
}

func run(ctx context.Context, o opcoes, saida, erros io.Writer) int {
	if o.gravar == o.conferir {
		_, _ = fmt.Fprintln(erros, "dmpf-modsync: use exatamente um entre --write e --check")
		return exitFalha
	}
	if o.conferir && len(o.requires) > 0 {
		_, _ = fmt.Fprintln(erros, "dmpf-modsync: --require só vale com --write")
		return exitFalha
	}
	requires := make([]modsync.Requirement, 0, len(o.requires))
	for _, r := range o.requires {
		req, err := modsync.ParseRequirement(r)
		if err != nil {
			_, _ = fmt.Fprintf(erros, "dmpf-modsync: %v\n", err)
			return exitFalha
		}
		requires = append(requires, req)
	}
	plan, err := modsync.Derive(ctx, o.raiz)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-modsync: %v\n", err)
		return exitFalha
	}
	if o.gravar {
		if err := modsync.Write(ctx, o.raiz, plan, requires...); err != nil {
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
