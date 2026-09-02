package baseline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/baseline"
)

// TestMudancaNoBaselineMarcaOProjetoComoAfetado: editar a cópia versionada
// precisa fazer o projeto entrar na lista de afetados.
//
// A cópia vive fora do módulo, então o cálculo de afetados não a associa a
// ninguém por padrão — e sem essa associação, alterá-la não dispara a
// verificação. O gate ficaria cego justamente para o arquivo que ele existe
// para proteger.
func TestMudancaNoBaselineMarcaOProjetoComoAfetado(t *testing.T) {
	raiz := raizDoRepo(t)

	declarados := inputsDoProjeto(t, raiz)
	if !strings.Contains(declarados, filepath.Dir(baseline.Path)) {
		t.Fatalf("nenhum target declara %s nos inputs; editar a cópia versionada não dispara a verificação",
			filepath.Dir(baseline.Path))
	}
}

// inputsDoProjeto concatena os inputs de todos os targets, para procurar a
// referência à pasta da cópia versionada sem depender de qual target a declara.
func inputsDoProjeto(t *testing.T, raiz string) string {
	t.Helper()
	p := filepath.Join(raiz, "libs", "backend", "go", "dmpf-conformance", "project.json")
	raw, err := os.ReadFile(p) //nolint:gosec // caminho fixo do próprio repositório
	if err != nil {
		t.Skipf("project.json ilegível: %v", err)
	}
	var doc struct {
		Targets map[string]struct {
			Inputs []string `json:"inputs"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decodificar project.json: %v", err)
	}
	var todos []string
	for _, tg := range doc.Targets {
		todos = append(todos, tg.Inputs...)
	}
	return strings.Join(todos, " ")
}

func raizDoRepo(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("fora de um repositório git: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestBaselineViveForaDeTodoModulo confirma no repositório real o que o teste
// de caminho afirma no abstrato: a cópia não cai dentro de nenhum módulo Go.
//
// Dentro de um, o mesmo commit que muda a classificação mexeria nos dois lados,
// e a comparação nunca acusaria nada.
func TestBaselineViveForaDeTodoModulo(t *testing.T) {
	raiz := raizDoRepo(t)
	out, err := exec.Command("git", "-C", raiz, "ls-files", "--", "*go.mod", "go.mod").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}

	for _, linha := range strings.Split(string(out), "\n") {
		if linha == "" || strings.Contains(linha, "/testdata/") {
			continue
		}
		dir := filepath.Dir(linha)
		if dir != "." && strings.HasPrefix(baseline.Path, dir+"/") {
			t.Errorf("a cópia versionada está dentro do módulo %s", dir)
		}
	}
}
