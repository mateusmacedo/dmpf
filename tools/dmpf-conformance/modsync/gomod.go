package modsync

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type versaoDeModulo struct {
	Path    string
	Version string
}

func (v versaoDeModulo) chave() string {
	if v.Version == "" {
		return v.Path
	}
	return v.Path + " " + v.Version
}

type replaceDeclarado struct {
	Old versaoDeModulo
	New versaoDeModulo
}

type goMod struct {
	Module struct {
		Path string
	}
	Require []versaoDeModulo
	Replace []replaceDeclarado
}

func (m goMod) versoesRequeridas() map[string]string {
	versoes := make(map[string]string, len(m.Require))
	for _, r := range m.Require {
		versoes[r.Path] = r.Version
	}
	return versoes
}

type goWork struct {
	Go        string
	Toolchain string
	Godebug   []json.RawMessage
	Use       []struct {
		DiskPath string
	}
	Replace []replaceDeclarado
}

func lerGoMod(ctx context.Context, dir string) (goMod, error) {
	saida, err := executar(ctx, dir, "go", "mod", "edit", "-json")
	if err != nil {
		return goMod{}, err
	}
	var m goMod
	if err := json.Unmarshal(saida, &m); err != nil {
		return goMod{}, fmt.Errorf("go mod edit -json: %w", err)
	}
	return m, nil
}

func lerGoWork(ctx context.Context, abs string) (goWork, error) {
	saida, err := executar(ctx, abs, "go", "work", "edit", "-json", filepath.Join(abs, "go.work"))
	if err != nil {
		return goWork{}, err
	}
	var w goWork
	if err := json.Unmarshal(saida, &w); err != nil {
		return goWork{}, fmt.Errorf("go work edit -json: %w", err)
	}
	return w, nil
}

func modulosDoWorkspace(ctx context.Context, abs string) ([]Module, error) {
	work, err := lerGoWork(ctx, abs)
	if err != nil {
		return nil, err
	}
	modules := make([]Module, 0, len(work.Use))
	for _, u := range work.Use {
		dir := path.Clean(filepath.ToSlash(u.DiskPath))
		if path.IsAbs(dir) || dir == ".." || strings.HasPrefix(dir, "../") {
			return nil, fmt.Errorf("go.work: use %s aponta para fora do repositório", u.DiskPath)
		}
		m, err := lerGoMod(ctx, filepath.Join(abs, filepath.FromSlash(dir)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		modules = append(modules, Module{Path: m.Module.Path, Dir: dir})
	}
	slices.SortFunc(modules, func(a, b Module) int { return cmp.Compare(a.Dir, b.Dir) })
	return modules, nil
}

func gravarGoWork(ctx context.Context, abs string, plan Plan) error {
	work, err := lerGoWork(ctx, abs)
	if err != nil {
		return err
	}
	if len(work.Godebug) > 0 {
		return errors.New("go.work com godebug: regravação do bloco replace não suportada")
	}
	irmaos := make(map[string]bool, len(plan.Modules))
	for _, mp := range plan.Modules {
		irmaos[mp.Module.Path] = true
	}

	var b strings.Builder
	fmt.Fprintf(&b, "go %s\n", work.Go)
	if work.Toolchain != "" {
		fmt.Fprintf(&b, "toolchain %s\n", work.Toolchain)
	}
	b.WriteString("\nuse (\n")
	for _, u := range work.Use {
		fmt.Fprintf(&b, "\t%s\n", u.DiskPath)
	}
	b.WriteString(")\n")
	var linhas []string
	for _, r := range plan.Replaces {
		linhas = append(linhas, fmt.Sprintf("%s %s => %s", r.Path, r.Version, r.Dir))
	}
	for _, r := range work.Replace {
		if !irmaos[r.Old.Path] {
			linhas = append(linhas, strings.TrimSpace(fmt.Sprintf("%s => %s %s", r.Old.chave(), r.New.Path, r.New.Version)))
		}
	}
	if len(linhas) > 0 {
		b.WriteString("\nreplace (\n")
		for _, l := range linhas {
			fmt.Fprintf(&b, "\t%s\n", l)
		}
		b.WriteString(")\n")
	}

	arquivo := filepath.Join(abs, "go.work")
	info, err := os.Stat(arquivo)
	if err != nil {
		return err
	}
	if err := os.WriteFile(arquivo, []byte(b.String()), info.Mode().Perm()); err != nil {
		return err
	}
	_, err = executar(ctx, abs, "go", "work", "edit", "-fmt", arquivo)
	return err
}

func buildList(ctx context.Context, abs string) (map[string]string, error) {
	saida, err := executar(ctx, abs, "go", "list", "-m", "-json", "all")
	if err != nil {
		return nil, err
	}
	lista := map[string]string{}
	dec := json.NewDecoder(bytes.NewReader(saida))
	for {
		var m struct {
			Path    string
			Version string
			Main    bool
		}
		if err := dec.Decode(&m); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list -m -json: %w", err)
		}
		if !m.Main {
			lista[m.Path] = m.Version
		}
	}
	return lista, nil
}

func executar(ctx context.Context, dir, nome string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, nome, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", nome, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
