package modsync_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/modsync"
)

const (
	prefixo = "example.test/ws/libs/"
	externo = "example.test/ext"
)

func ambienteGo(t *testing.T) {
	t.Helper()
	proxy, err := filepath.Abs("testdata/proxy")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPROXY", "file://"+filepath.ToSlash(proxy))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "")
}

func executarGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-c", "user.name=teste", "-c", "user.email=teste@exemplo.test", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func executarGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
}

func workspaceVersionado(t *testing.T) string {
	t.Helper()
	ambienteGo(t)
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/workspace")); err != nil {
		t.Fatal(err)
	}
	executarGit(t, dir, "init", "-q")
	executarGit(t, dir, "add", ".")
	executarGit(t, dir, "commit", "-q", "-m", "fixture")
	return dir
}

func derivar(t *testing.T, raiz string) modsync.Plan {
	t.Helper()
	plan, err := modsync.Derive(context.Background(), raiz)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	return plan
}

func escrever(t *testing.T, raiz string) {
	t.Helper()
	if err := modsync.Write(context.Background(), raiz, derivar(t, raiz)); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func checar(t *testing.T, raiz string) []modsync.Finding {
	t.Helper()
	findings, err := modsync.Check(context.Background(), raiz, derivar(t, raiz))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return findings
}

func ler(t *testing.T, raiz, arquivo string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(arquivo)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func irmaosDe(t *testing.T, plan modsync.Plan, dir string) []modsync.Requirement {
	t.Helper()
	for _, m := range plan.Modules {
		if m.Module.Dir == dir {
			return m.Siblings
		}
	}
	t.Fatalf("nenhum módulo em %s", dir)
	return nil
}

func achadosDe(findings []modsync.Finding, onde string) string {
	var detalhes []string
	for _, f := range findings {
		if f.Module == onde {
			detalhes = append(detalhes, f.Detail)
		}
	}
	return strings.Join(detalhes, "\n")
}

func TestDeriveIrmaosImportadosSemTagNascemEmV010(t *testing.T) {
	plan := derivar(t, workspaceVersionado(t))

	casos := map[string][]modsync.Requirement{
		"libs/a":     nil,
		"libs/b":     {{Path: prefixo + "a", Version: "v0.1.0"}},
		"libs/c":     {{Path: prefixo + "a", Version: "v0.1.0"}, {Path: prefixo + "b", Version: "v0.1.0"}},
		"libs/ctx/d": {{Path: prefixo + "b", Version: "v0.1.0"}},
		"libs/f":     nil,
	}
	if len(plan.Modules) != len(casos) {
		t.Fatalf("%d módulos, esperados %d: %+v", len(plan.Modules), len(casos), plan.Modules)
	}
	for dir, want := range casos {
		if got := irmaosDe(t, plan, dir); !slices.Equal(got, want) {
			t.Errorf("%s: irmãos %+v, esperados %+v", dir, got, want)
		}
	}

	wantReplaces := []modsync.Replacement{
		{Path: prefixo + "a", Version: "v0.1.0", Dir: "./libs/a"},
		{Path: prefixo + "b", Version: "v0.1.0", Dir: "./libs/b"},
	}
	if !slices.Equal(plan.Replaces, wantReplaces) {
		t.Fatalf("replaces %+v, esperados %+v", plan.Replaces, wantReplaces)
	}
}

func TestDeriveVersaoPelaMaiorTagDeReleaseAlcancavel(t *testing.T) {
	raiz := workspaceVersionado(t)
	for _, tag := range []string{"libs/a/v0.2.0", "libs/a/v0.10.0", "libs/a/v0.10.1-rc.1", "libs/a/vlixo", "libs/ab/v9.0.0"} {
		executarGit(t, raiz, "tag", tag)
	}
	executarGit(t, raiz, "checkout", "-q", "-b", "lateral")
	executarGit(t, raiz, "commit", "-q", "--allow-empty", "-m", "lateral")
	executarGit(t, raiz, "tag", "libs/a/v0.11.0")
	executarGit(t, raiz, "checkout", "-q", "-")

	plan := derivar(t, raiz)

	want := []modsync.Requirement{{Path: prefixo + "a", Version: "v0.10.0"}}
	if got := irmaosDe(t, plan, "libs/b"); !slices.Equal(got, want) {
		t.Fatalf("irmãos de libs/b %+v, esperados %+v", got, want)
	}
	if !slices.Contains(plan.Replaces, modsync.Replacement{Path: prefixo + "a", Version: "v0.10.0", Dir: "./libs/a"}) {
		t.Fatalf("replace de a em v0.10.0 ausente: %+v", plan.Replaces)
	}
}

func TestDerivePreservaVersaoDeRequireExistente(t *testing.T) {
	raiz := workspaceVersionado(t)
	executarGo(t, filepath.Join(raiz, "libs", "ctx", "d"), "mod", "edit", "-require="+prefixo+"b@v0.0.3")

	plan := derivar(t, raiz)

	want := []modsync.Requirement{{Path: prefixo + "b", Version: "v0.0.3"}}
	if got := irmaosDe(t, plan, "libs/ctx/d"); !slices.Equal(got, want) {
		t.Fatalf("irmãos de libs/ctx/d %+v, esperados %+v", got, want)
	}
	wantReplaces := []modsync.Replacement{
		{Path: prefixo + "a", Version: "v0.1.0", Dir: "./libs/a"},
		{Path: prefixo + "b", Version: "v0.0.3", Dir: "./libs/b"},
		{Path: prefixo + "b", Version: "v0.1.0", Dir: "./libs/b"},
	}
	if !slices.Equal(plan.Replaces, wantReplaces) {
		t.Fatalf("replaces %+v, esperados %+v", plan.Replaces, wantReplaces)
	}
}

func TestWriteEntaoCheckConformeEIdempotente(t *testing.T) {
	raiz := workspaceVersionado(t)
	escrever(t, raiz)

	if findings := checar(t, raiz); len(findings) != 0 {
		t.Fatalf("achados depois do Write: %v", findings)
	}

	goModC := ler(t, raiz, "libs/c/go.mod")
	for _, trecho := range []string{prefixo + "a v0.1.0", prefixo + "b v0.1.0", externo + " v1.2.0"} {
		if !strings.Contains(goModC, trecho) {
			t.Errorf("go.mod de libs/c sem %q:\n%s", trecho, goModC)
		}
	}
	if strings.Contains(goModC, "replace") {
		t.Errorf("go.mod de libs/c com replace:\n%s", goModC)
	}
	goWork := ler(t, raiz, "go.work")
	for _, trecho := range []string{"replace (", prefixo + "a v0.1.0 => ./libs/a", prefixo + "b v0.1.0 => ./libs/b"} {
		if !strings.Contains(goWork, trecho) {
			t.Errorf("go.work sem %q:\n%s", trecho, goWork)
		}
	}

	escrever(t, raiz)
	if depois := ler(t, raiz, "go.work"); depois != goWork {
		t.Errorf("segundo Write mudou o go.work:\nantes:\n%s\ndepois:\n%s", goWork, depois)
	}
	if depois := ler(t, raiz, "libs/c/go.mod"); depois != goModC {
		t.Errorf("segundo Write mudou o go.mod de libs/c:\nantes:\n%s\ndepois:\n%s", goModC, depois)
	}
}

func TestCheckApontaRequireFaltanteDeIrmaoEExterno(t *testing.T) {
	raiz := workspaceVersionado(t)
	findings := checar(t, raiz)

	esperados := map[string][]string{
		"libs/b":     {prefixo + "a"},
		"libs/c":     {prefixo + "a", prefixo + "b", externo},
		"libs/ctx/d": {prefixo + "b"},
		"go.work":    {prefixo + "a", prefixo + "b"},
	}
	for onde, trechos := range esperados {
		detalhes := achadosDe(findings, onde)
		for _, trecho := range trechos {
			if !strings.Contains(detalhes, trecho) {
				t.Errorf("%s sem achado para %q: %q", onde, trecho, detalhes)
			}
		}
	}
	if detalhes := achadosDe(findings, "libs/b"); strings.Contains(detalhes, externo+" ") {
		t.Errorf("libs/b já requer %s: %q", externo, detalhes)
	}
	for _, dir := range []string{"libs/a", "libs/f"} {
		if detalhes := achadosDe(findings, dir); detalhes != "" {
			t.Errorf("%s não deveria ter achado: %q", dir, detalhes)
		}
	}
}

func TestCheckReprovaReplaceNoGoModEBlocoDoGoWorkDivergente(t *testing.T) {
	raiz := workspaceVersionado(t)
	escrever(t, raiz)
	executarGo(t, filepath.Join(raiz, "libs", "b"), "mod", "edit", "-replace="+prefixo+"a=../a")
	executarGo(t, raiz, "work", "edit", "-dropreplace="+prefixo+"b@v0.1.0", "-replace=example.test/externo@v1.0.0=./libs/f")

	findings := checar(t, raiz)

	if detalhes := achadosDe(findings, "libs/b"); !strings.Contains(detalhes, "go.mod") || !strings.Contains(detalhes, prefixo+"a") {
		t.Errorf("replace no go.mod de libs/b não apontado: %q", detalhes)
	}
	detalhesWork := achadosDe(findings, "go.work")
	for _, trecho := range []string{prefixo + "b v0.1.0", "example.test/externo"} {
		if !strings.Contains(detalhesWork, trecho) {
			t.Errorf("go.work sem achado para %q: %q", trecho, detalhesWork)
		}
	}
}

func TestWriteFalhaComImportExternoSemModuloNaBuildList(t *testing.T) {
	raiz := workspaceVersionado(t)
	arquivo := filepath.Join(raiz, "libs", "f", "novo.go")
	if err := os.WriteFile(arquivo, []byte("package f\n\nimport _ \"example.test/desconhecido\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := modsync.Write(context.Background(), raiz, derivar(t, raiz))
	if err == nil || !strings.Contains(err.Error(), "example.test/desconhecido") {
		t.Fatalf("Write não reprovou import sem módulo na build list: %v", err)
	}
	if detalhes := achadosDe(checar(t, raiz), "libs/f"); !strings.Contains(detalhes, "example.test/desconhecido") {
		t.Errorf("Check não apontou o import sem módulo: %q", detalhes)
	}
}

func TestDeriveRecusaUseForaDoRepositorio(t *testing.T) {
	raiz := workspaceVersionado(t)
	executarGo(t, raiz, "work", "edit", "-use=../fora")

	_, err := modsync.Derive(context.Background(), raiz)
	if err == nil || !strings.Contains(err.Error(), "fora do repositório") {
		t.Fatalf("use fora do repositório não recusado: %v", err)
	}
}

func TestDeriveRecusaCloneRaso(t *testing.T) {
	origem := workspaceVersionado(t)
	executarGit(t, origem, "commit", "-q", "--allow-empty", "-m", "segundo")
	destino := t.TempDir()
	raso := filepath.Join(destino, "raso")
	executarGit(t, destino, "clone", "-q", "--depth", "1", "file://"+origem, raso)

	_, err := modsync.Derive(context.Background(), raso)
	if err == nil || !strings.Contains(err.Error(), "raso") {
		t.Fatalf("clone raso não recusado: %v", err)
	}
}
