package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// Inventory reconcilia go.mod rastreados, projetos Nx `stack:go` e membros do
// go.work. Presente em QUALQUER fonte entra: descobrir só pelo `go.work`
// deixaria de fora o módulo omitido dele, que nunca geraria o DMPF-U004 que
// existe para detectá-lo.
type Inventory struct {
	root string
}

func NewInventory(root string) *Inventory { return &Inventory{root: root} }

func (i *Inventory) Modules() ([]rule.Module, error) {
	dirs := map[string]struct{}{}
	membros := map[string]bool{}

	fromGit, err := i.gitTrackedGoMods()
	if err != nil {
		return nil, err
	}
	fromWork, err := i.goWorkMembers()
	if err != nil {
		return nil, err
	}
	fromNx, err := i.nxGoProjects()
	if err != nil {
		return nil, err
	}
	for _, d := range fromWork {
		membros[filepath.Clean(d)] = true
	}
	for _, src := range [][]string{fromGit, fromWork, fromNx} {
		for _, d := range src {
			if excluidoDoUniverso(d) {
				continue
			}
			dirs[filepath.Clean(d)] = struct{}{}
		}
	}

	var out []rule.Module
	for dir := range dirs {
		abs := filepath.Join(i.root, dir)
		path, err := modulePath(filepath.Join(abs, "go.mod"))
		if err != nil {
			if os.IsNotExist(err) {
				continue // única ausência tolerada: não é módulo Go
			}
			// Sumir com o diretório esconderia o U004 e os U001 dos packages
			// dele: o verificador ficaria verde por não ter olhado.
			return nil, fmt.Errorf("módulo em %s não é verificável: %w", dir, err)
		}
		temCodigo, err := hasProductionCode(abs)
		if err != nil {
			return nil, fmt.Errorf("varrer código de produção em %s: %w", dir, err)
		}
		manifesto, err := fileExists(filepath.Join(abs, ManifestFileName))
		if err != nil {
			return nil, fmt.Errorf("verificar manifesto em %s: %w", dir, err)
		}
		out = append(out, rule.Module{
			Path:            path,
			Dir:             abs,
			HasManifest:     manifesto,
			HasProduction:   temCodigo,
			WorkspaceMember: membros[dir],
		})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Path < out[b].Path })
	return out, nil
}

func ModuleDirs(mods []rule.Module) []ModuleDir {
	out := make([]ModuleDir, 0, len(mods))
	for _, m := range mods {
		out = append(out, ModuleDir{Path: m.Path, Dir: m.Dir})
	}
	return out
}

// A fonte que não depende de configuração: `go.mod` versionado, módulo existe.
func (i *Inventory) gitTrackedGoMods() ([]string, error) {
	out, err := i.run("git", "ls-files", "--", "*go.mod", "go.mod")
	if err != nil {
		return nil, fmt.Errorf("listar go.mod rastreados: %w", err)
	}
	var dirs []string
	for _, line := range splitLines(out) {
		dirs = append(dirs, filepath.Dir(line))
	}
	return dirs, nil
}

// A fonte mais fácil de burlar (basta omitir a linha), e por isso nunca a única.
func (i *Inventory) goWorkMembers() ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(i.root, "go.work"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ler go.work: %w", err)
	}
	var dirs []string
	dentro := false
	for _, line := range splitLines(string(raw)) {
		l := strings.TrimSpace(line)
		switch {
		case l == "use (":
			dentro = true
		case dentro && l == ")":
			dentro = false
		case dentro && l != "":
			dirs = append(dirs, filepath.Clean(l))
		case strings.HasPrefix(l, "use "):
			dirs = append(dirs, filepath.Clean(strings.TrimSpace(strings.TrimPrefix(l, "use "))))
		}
	}
	return dirs, nil
}

// Leitura do disco, não `nx show project`: invocar o Nx por projeto custava
// dezenas de segundos e amarraria o verificador a um toolchain de JavaScript.
func (i *Inventory) nxGoProjects() ([]string, error) {
	out, err := i.run("git", "ls-files", "--", "*project.json", "project.json")
	if err != nil {
		return nil, fmt.Errorf("listar project.json rastreados: %w", err)
	}
	var dirs []string
	for _, rel := range splitLines(out) {
		if excluidoDoUniverso(rel) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(i.root, rel))
		if err != nil {
			return nil, fmt.Errorf("ler %s: %w", rel, err)
		}
		var p struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decodificar %s: %w", rel, err)
		}
		for _, t := range p.Tags {
			if t == "stack:go" {
				dirs = append(dirs, filepath.Dir(rel))
				break
			}
		}
	}
	return dirs, nil
}

// Módulo sintético sob `testdata/` não é produção: tratá-lo como tal faria o
// verificador reprovar o repositório pela violação que a fixture existe para
// demonstrar.
func excluidoDoUniverso(rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == "testdata" || seg == "vendor" {
			return true
		}
	}
	return false
}

func (i *Inventory) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = i.root
	out, err := cmd.Output()
	return string(out), err
}

// A diretiva `module` é o import path canônico e a chave do universo.
func modulePath(goMod string) (string, error) {
	raw, err := os.ReadFile(filepath.Clean(goMod))
	if err != nil {
		return "", err
	}
	for _, line := range splitLines(string(raw)) {
		l := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(l, "module "); ok {
			return strings.TrimSpace(after), nil
		}
	}
	return "", fmt.Errorf("%s: sem diretiva module", goMod)
}

// Separa "módulo sem manifesto" (U004) de "diretório sem código".
func hasProductionCode(dir string) (bool, error) {
	achou := false
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		// Erro de leitura não é ausência de código: sem propagar, o módulo
		// passaria por vazio só porque o diretório não pôde ser lido.
		if err != nil {
			return err
		}
		if achou {
			return filepath.SkipAll
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			achou = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return achou, nil
}

func fileExists(p string) (bool, error) {
	_, err := os.Stat(p)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
