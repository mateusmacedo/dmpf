// Command dmpf-bom valida o BOM da release do produto DMPF (dmpf/bom@1) e
// reprova o PR quando encontra diagnóstico.
//
// Composition root: lê o relógio, o disco e o git, e passa tudo por valor.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/bom"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

const (
	exitConforme  = 0
	exitReprovado = 1
	exitFalha     = 2
)

const (
	dirBOM             = "bom/dmpf"
	releaseMaisRecente = "latest"
)

func main() {
	raiz := flag.String("root", ".", "raiz do workspace")
	release := flag.String("release", "", "release a validar: <semver>, ou latest para a maior semver de bom/dmpf/ (default: o único arquivo)")
	agora := flag.String("now", "", "instante RFC3339 contra o qual as validades vencem (default: relógio)")
	base := flag.String("base", "", "ref git do BOM anterior: o mesmo arquivo ou, se ausente, a maior semver no ref (B003)")
	flag.Parse()

	os.Exit(run(opcoes{raiz: *raiz, release: *release, agora: *agora, base: *base}, os.Stdout, os.Stderr))
}

type opcoes struct {
	raiz    string
	release string
	agora   string
	base    string
}

func run(o opcoes, saida, erros io.Writer) int {
	diags, err := validar(o)
	if err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-bom: %v\n", err)
		return exitFalha
	}

	w := bufio.NewWriter(saida)
	for _, d := range diags {
		_, _ = fmt.Fprintln(w, d.String())
	}
	if len(diags) == 0 {
		_, _ = fmt.Fprintln(w, "dmpf-bom: conforme")
	} else {
		_, _ = fmt.Fprintf(w, "\ndmpf-bom: REPROVADO com %d diagnóstico(s)\n", len(diags))
	}
	if err := w.Flush(); err != nil {
		_, _ = fmt.Fprintf(erros, "dmpf-bom: escrever relatório: %v\n", err)
		return exitFalha
	}

	if len(diags) > 0 {
		return exitReprovado
	}
	return exitConforme
}

func validar(o opcoes) ([]rule.Diagnostic, error) {
	now, err := instanteDe(o.agora)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(o.base, "-") {
		return nil, fmt.Errorf("--base %q não é ref", o.base)
	}
	abs, err := filepath.Abs(o.raiz)
	if err != nil {
		return nil, err
	}
	// os.Root recusa symlink que escape da raiz; os.DirFS o seguiria.
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	fsys := root.FS()

	arquivo, ambiguos, err := resolverArquivo(fsys, o.release)
	if err != nil {
		return nil, err
	}
	if len(ambiguos) > 0 {
		return []rule.Diagnostic{{
			Code:         rule.CodeB011,
			CanonicalKey: dirBOM,
			Detail:       fmt.Sprintf("%d BOMs sem --release (%s): cada release tem exatamente um", len(ambiguos), strings.Join(ambiguos, ", ")),
		}}, nil
	}

	raw, err := fs.ReadFile(fsys, arquivo)
	if err != nil {
		return nil, err
	}
	doc, err := bom.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", arquivo, err)
	}
	anterior, err := baseEm(abs, o.base, arquivo)
	if err != nil {
		return nil, err
	}
	return bom.Validate(doc, bom.Input{File: arquivo, Now: now, Root: fsys, Base: anterior})
}

func instanteDe(v string) (exception.Instant, error) {
	if v == "" {
		return exception.Instant(time.Now().UnixNano()), nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0, fmt.Errorf("--now %q não é RFC3339: %w", v, err)
	}
	return exception.Instant(t.UnixNano()), nil
}

func resolverArquivo(fsys fs.FS, release string) (string, []string, error) {
	switch {
	case release == releaseMaisRecente:
	case release != "":
		if !bom.ValidRelease(release) {
			return "", nil, fmt.Errorf("--release %q não é semver nem %s", release, releaseMaisRecente)
		}
		return path.Join(dirBOM, release+".json"), nil, nil
	}
	encontrados, err := fs.Glob(fsys, dirBOM+"/*.json")
	if err != nil {
		return "", nil, err
	}
	nomes := make([]string, 0, len(encontrados))
	for _, f := range encontrados {
		nomes = append(nomes, path.Base(f))
	}
	if release == releaseMaisRecente {
		maior, ok := bom.LatestRelease(semExtensao(nomes))
		if !ok {
			return "", nil, fmt.Errorf("nenhum BOM com nome semver em %s", dirBOM)
		}
		return path.Join(dirBOM, maior+".json"), nil, nil
	}
	switch len(nomes) {
	case 0:
		return "", nil, fmt.Errorf("nenhum BOM em %s", dirBOM)
	case 1:
		return path.Join(dirBOM, nomes[0]), nil, nil
	}
	return "", nomes, nil
}

func semExtensao(nomes []string) []string {
	out := make([]string, 0, len(nomes))
	for _, n := range nomes {
		out = append(out, strings.TrimSuffix(n, ".json"))
	}
	return out
}

// A release nova não existe no ref: a comparação passa a ser com o BOM da maior
// semver de lá. Sem BOM nenhum no ref, o documento vazio faz toda entrada cair
// na regra de estado inicial.
func baseEm(abs, ref, arquivo string) (*bom.Document, error) {
	if ref == "" {
		return nil, nil
	}
	if _, err := git(abs, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		return nil, fmt.Errorf("ref %s inalcançável: %w", ref, err)
	}
	alvo := arquivo
	if _, err := git(abs, "cat-file", "-e", ref+":"+arquivo); err != nil {
		listagem, err := git(abs, "ls-tree", "--name-only", ref, dirBOM+"/")
		if err != nil {
			return nil, fmt.Errorf("listar %s em %s: %w", dirBOM, ref, err)
		}
		var nomes []string
		for _, linha := range strings.Split(strings.TrimSpace(string(listagem)), "\n") {
			if strings.HasSuffix(linha, ".json") {
				nomes = append(nomes, path.Base(linha))
			}
		}
		maior, ok := bom.LatestRelease(semExtensao(nomes))
		if !ok {
			return &bom.Document{}, nil
		}
		alvo = path.Join(dirBOM, maior+".json")
	}
	saida, err := git(abs, "show", ref+":"+alvo)
	if err != nil {
		return nil, fmt.Errorf("ler %s em %s: %w", alvo, ref, err)
	}
	doc, err := bom.Decode(saida)
	if err != nil {
		return nil, fmt.Errorf("%s em %s: %w", alvo, ref, err)
	}
	return &doc, nil
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Output()
}
