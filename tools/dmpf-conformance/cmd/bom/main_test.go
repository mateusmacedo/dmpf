package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const agoraFixo = "2026-09-12T00:00:00Z"

func workspaceDeTeste(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../bom/testdata/workspace")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func rodar(o opcoes) (int, string) {
	var saida, erros bytes.Buffer
	code := run(o, &saida, &erros)
	return code, saida.String() + erros.String()
}

func TestSaiComZeroSemDiagnostico(t *testing.T) {
	code, out := rodar(opcoes{raiz: workspaceDeTeste(t), agora: agoraFixo})
	if code != exitConforme || !strings.Contains(out, "dmpf-bom: conforme") {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitConforme, out)
	}
}

func TestSaiComUmQuandoHaDoisBOMsSemRelease(t *testing.T) {
	dir := workspaceDeTeste(t)
	raw, err := os.ReadFile(filepath.Join(dir, "bom", "dmpf", "0.1.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bom", "dmpf", "0.2.0.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := rodar(opcoes{raiz: dir, agora: agoraFixo})
	if code != exitReprovado || !strings.Contains(out, "DMPF-B011") {
		t.Fatalf("exit %d, esperado %d com B011:\n%s", code, exitReprovado, out)
	}
}

func TestSaiComDoisEmErroDeLeitura(t *testing.T) {
	code, out := rodar(opcoes{raiz: workspaceDeTeste(t), release: "9.9.9", agora: agoraFixo})
	if code != exitFalha {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitFalha, out)
	}
}

func TestBaseComFormaDeFlagFalha(t *testing.T) {
	code, out := rodar(opcoes{raiz: workspaceDeTeste(t), agora: agoraFixo, base: "--output=x"})
	if code != exitFalha {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitFalha, out)
	}
}

func TestReleaseForaDeSemverFalha(t *testing.T) {
	code, out := rodar(opcoes{raiz: workspaceDeTeste(t), release: "../escape", agora: agoraFixo})
	if code != exitFalha {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitFalha, out)
	}
}

func executarGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=teste", "-c", "user.email=teste@exemplo.test", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestBaseAvaliaTransicaoContraORefDoGit(t *testing.T) {
	dir := workspaceDeTeste(t)
	arquivo := filepath.Join(dir, "bom", "dmpf", "0.1.0.json")
	atual, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	anterior := strings.Replace(string(atual), `"state": "candidata"`, `"state": "rejeitada"`, 1)
	if anterior == string(atual) {
		t.Fatal("fixture sem entrada candidata para rebaixar no base")
	}
	if err := os.WriteFile(arquivo, []byte(anterior), 0o644); err != nil {
		t.Fatal(err)
	}
	executarGit(t, dir, "init", "-q")
	executarGit(t, dir, "add", ".")
	executarGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(arquivo, atual, 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := rodar(opcoes{raiz: dir, agora: agoraFixo, base: "HEAD"})
	if code != exitReprovado || !strings.Contains(out, "DMPF-B003") {
		t.Fatalf("rejeitada → candidata não reprovou em B003: exit %d\n%s", code, out)
	}

	if code, out := rodar(opcoes{raiz: dir, agora: agoraFixo, base: "ref-inexistente"}); code != exitFalha {
		t.Fatalf("ref inalcançável não falhou: exit %d\n%s", code, out)
	}
}

func criarRelease(t *testing.T, dir, conteudo, release string) {
	t.Helper()
	json := strings.ReplaceAll(conteudo, "0.1.0", release)
	if err := os.WriteFile(filepath.Join(dir, "bom", "dmpf", release+".json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	evidencia := filepath.Join(dir, "bom", "evidence", release)
	if err := os.CopyFS(evidencia, os.DirFS(filepath.Join(dir, "bom", "evidence", "0.1.0"))); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseLatestValidaAMaiorSemver(t *testing.T) {
	dir := workspaceDeTeste(t)
	original, err := os.ReadFile(filepath.Join(dir, "bom", "dmpf", "0.1.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	criarRelease(t, dir, string(original), "0.10.0")
	criarRelease(t, dir, string(original), "0.9.0")

	code, out := rodar(opcoes{raiz: dir, release: "latest", agora: agoraFixo})
	if code != exitConforme {
		t.Fatalf("exit %d, esperado %d:\n%s", code, exitConforme, out)
	}
	if code, out := rodar(opcoes{raiz: dir, agora: agoraFixo}); code != exitReprovado || !strings.Contains(out, "DMPF-B011") {
		t.Fatalf("sem --release, três BOMs não reprovaram em B011: exit %d\n%s", code, out)
	}
}

func TestBaseSemOArquivoComparaComAMaiorSemverDoRef(t *testing.T) {
	dir := workspaceDeTeste(t)
	arquivo := filepath.Join(dir, "bom", "dmpf", "0.1.0.json")
	original, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	anterior := strings.Replace(string(original), `"state": "candidata"`, `"state": "rejeitada"`, 1)
	if err := os.WriteFile(arquivo, []byte(anterior), 0o644); err != nil {
		t.Fatal(err)
	}
	executarGit(t, dir, "init", "-q")
	executarGit(t, dir, "add", ".")
	executarGit(t, dir, "commit", "-q", "-m", "release 0.1.0")
	criarRelease(t, dir, string(original), "0.2.0")

	code, out := rodar(opcoes{raiz: dir, release: "latest", agora: agoraFixo, base: "HEAD"})
	if code != exitReprovado || !strings.Contains(out, "DMPF-B003") {
		t.Fatalf("0.2.0 não foi comparada com a 0.1.0 do ref: exit %d\n%s", code, out)
	}
}

func TestBaseSemBOMTrataTodaEntradaComoNova(t *testing.T) {
	dir := workspaceDeTeste(t)
	executarGit(t, dir, "init", "-q")
	executarGit(t, dir, "add", "go.work")
	executarGit(t, dir, "commit", "-q", "-m", "antes do BOM")

	code, out := rodar(opcoes{raiz: dir, agora: agoraFixo, base: "HEAD"})
	if code != exitReprovado || !strings.Contains(out, "entrada nova em certificada") {
		t.Fatalf("entrada certificada sem BOM no ref não reprovou como nova: exit %d\n%s", code, out)
	}
}
