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

// Inventory reconcilia as três fontes de módulos Go do workspace: os `go.mod`
// rastreados pelo git, os projetos Nx com tag `stack:go` e os membros do
// `go.work`.
//
// Módulo presente em QUALQUER fonte entra no universo. Descobrir só pelo
// `go.work` deixaria de fora um módulo omitido dele — com `go.mod`, código de
// produção e projeto Nx — e ele nunca geraria o DMPF-U004 que existe para
// detectá-lo.
type Inventory struct {
	root string
}

func NewInventory(root string) *Inventory { return &Inventory{root: root} }

// Modules devolve o inventário reconciliado, ordenado por import path.
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
				// Candidato apontado por uma fonte sem `go.mod` nenhum: não é
				// módulo Go. Única ausência tolerada.
				continue
			}
			// `go.mod` presente e ilegível ou sem diretiva `module` torna a
			// execução NÃO VERIFICÁVEL. Sumir com o diretório esconderia tanto
			// o U004 quanto os U001 dos packages dele — o verificador ficaria
			// verde por não ter olhado.
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

// ModuleDirs projeta o inventário no formato que o ManifestStore consome.
func ModuleDirs(mods []rule.Module) []ModuleDir {
	out := make([]ModuleDir, 0, len(mods))
	for _, m := range mods {
		out = append(out, ModuleDir{Path: m.Path, Dir: m.Dir})
	}
	return out
}

// gitTrackedGoMods é a fonte que não depende de nenhuma configuração do
// workspace: se o `go.mod` está versionado, o módulo existe.
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

// goWorkMembers lê os `use` do go.work. É a fonte mais fácil de burlar — basta
// omitir a linha — e por isso nunca é a única.
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

// nxGoProjects encontra os projetos com tag `stack:go` lendo os `project.json`
// rastreados. A tag é a declaração do workspace de que aquele diretório é um
// módulo Go de produção.
//
// A leitura é do disco, não por `nx show project`: invocar o Nx por projeto
// custava dezenas de segundos e amarraria o verificador a um toolchain de
// JavaScript que ele não precisa para decidir nada.
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

// excluidoDoUniverso aplica as exclusões FECHADAS de RFC §10.3 à descoberta de
// módulos. `testdata/` e `vendor/` têm significado no toolchain: o primeiro
// guarda fixtures, o segundo dependências copiadas. Um módulo sintético de
// fixture não é código de produção, e tratá-lo como tal faria o verificador
// reprovar o repositório pela violação que a fixture existe para demonstrar.
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

// modulePath extrai a diretiva `module` do go.mod. É o import path canônico do
// módulo, e a chave pela qual o universo o identifica.
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

// hasProductionCode reporta se o módulo tem ao menos um .go que não seja teste,
// aplicando as exclusões fechadas de RFC §10.3. É o predicado que separa
// "módulo sem manifesto" (U004) de "diretório sem código".
func hasProductionCode(dir string) (bool, error) {
	achou := false
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		// Erro de leitura NÃO é ausência de código: propagar é o que impede o
		// módulo de passar por vazio só porque o diretório não pôde ser lido.
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
