package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func executarGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-c", "user.name=teste", "-c", "user.email=teste@exemplo.test", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func workspaceDeTeste(t *testing.T, versionado bool) string {
	t.Helper()
	proxy, err := filepath.Abs("../../modsync/testdata/proxy")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPROXY", "file://"+filepath.ToSlash(proxy))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "")

	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../modsync/testdata/workspace")); err != nil {
		t.Fatal(err)
	}
	if versionado {
		executarGit(t, dir, "init", "-q")
		executarGit(t, dir, "add", ".")
		executarGit(t, dir, "commit", "-q", "-m", "fixture")
	}
	return dir
}

func rodar(o opcoes) (int, string) {
	var saida, erros bytes.Buffer
	code := run(context.Background(), o, &saida, &erros)
	return code, saida.String() + erros.String()
}

func TestCheckSemSyncSaiComUm(t *testing.T) {
	code, out := rodar(opcoes{raiz: workspaceDeTeste(t, true), conferir: true})
	if code != exitReprovado || !strings.Contains(out, "libs/b: ") || !strings.Contains(out, "REPROVADO") {
		t.Fatalf("exit %d, esperado %d com divergência em libs/b:\n%s", code, exitReprovado, out)
	}
}

func TestWriteEntaoCheckSaiComZero(t *testing.T) {
	dir := workspaceDeTeste(t, true)
	if code, out := rodar(opcoes{raiz: dir, gravar: true}); code != exitConforme {
		t.Fatalf("--write: exit %d:\n%s", code, out)
	}
	code, out := rodar(opcoes{raiz: dir, conferir: true})
	if code != exitConforme || !strings.Contains(out, "dmpf-modsync: conforme") {
		t.Fatalf("--check depois do --write: exit %d:\n%s", code, out)
	}
}

func TestModoAusenteOuDuploFalha(t *testing.T) {
	dir := workspaceDeTeste(t, true)
	for _, o := range []opcoes{{raiz: dir}, {raiz: dir, gravar: true, conferir: true}} {
		if code, out := rodar(o); code != exitFalha {
			t.Errorf("%+v: exit %d, esperado %d:\n%s", o, code, exitFalha, out)
		}
	}
}

func TestRaizForaDeRepositorioGitFalha(t *testing.T) {
	if code, out := rodar(opcoes{raiz: workspaceDeTeste(t, false), conferir: true}); code != exitFalha {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitFalha, out)
	}
}
