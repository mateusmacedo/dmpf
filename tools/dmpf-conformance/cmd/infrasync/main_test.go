package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func workspaceDeTeste(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../infrasync/testdata/workspace")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func rodar(o opcoes) (int, string) {
	var saida, erros bytes.Buffer
	code := run(o, &saida, &erros)
	return code, saida.String() + erros.String()
}

func TestCheckSemWriteSaiComUm(t *testing.T) {
	code, out := rodar(opcoes{raiz: workspaceDeTeste(t), conferir: true})
	if code != exitReprovado || !strings.Contains(out, "provisioning.generated.yml: ausente") || !strings.Contains(out, "REPROVADO") {
		t.Fatalf("exit %d, esperado %d com o provisionamento ausente:\n%s", code, exitReprovado, out)
	}
}

func TestWriteEntaoCheckSaiComZero(t *testing.T) {
	dir := workspaceDeTeste(t)
	if code, out := rodar(opcoes{raiz: dir, gravar: true}); code != exitConforme {
		t.Fatalf("--write: exit %d:\n%s", code, out)
	}
	code, out := rodar(opcoes{raiz: dir, conferir: true})
	if code != exitConforme || !strings.Contains(out, "dmpf-infrasync: conforme") {
		t.Fatalf("--check depois do --write: exit %d:\n%s", code, out)
	}
}

func TestModoAusenteOuDuploFalha(t *testing.T) {
	dir := workspaceDeTeste(t)
	for _, o := range []opcoes{{raiz: dir}, {raiz: dir, gravar: true, conferir: true}} {
		if code, out := rodar(o); code != exitFalha {
			t.Errorf("%+v: exit %d, esperado %d:\n%s", o, code, exitFalha, out)
		}
	}
}

func TestManifestoInvalidoFalha(t *testing.T) {
	dir := workspaceDeTeste(t)
	if err := os.WriteFile(dir+"/apps/backend/alpha/deploy/infra.json", []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := rodar(opcoes{raiz: dir, conferir: true}); code != exitFalha {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitFalha, out)
	}
}
