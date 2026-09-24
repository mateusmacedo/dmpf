package conformance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/fsstore"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/golist"
)

// Dentro de um hook o git exporta GIT_DIR sem GIT_WORK_TREE, e `rev-parse
// --show-toplevel` passa a devolver o cwd em vez da raiz do workspace.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if nome, _, _ := strings.Cut(kv, "="); strings.HasPrefix(nome, "GIT_") {
			_ = os.Unsetenv(nome)
		}
	}
	os.Exit(m.Run())
}

// A promessa central do verificador, e um teste real porque ele é decomposto na
// arquitetura
// que verifica: um verificador de pacote único passaria trivialmente, sem duas
// unidades não há aresta a decidir.
func TestVerificadorPassaNoProprioGate(t *testing.T) {
	raiz := raizDoWorkspace(t)

	perfis, err := fsstore.LoadBuildProfiles(
		filepath.Join(raiz, "tools", "dmpf-conformance", "build-profiles.json"))
	if err != nil {
		t.Fatalf("perfis de produção: %v", err)
	}

	modules, err := fsstore.NewInventory(raiz).Modules()
	if err != nil {
		t.Fatalf("inventário: %v", err)
	}
	if len(modules) < 2 {
		t.Fatalf("inventário devolveu %d módulo(s); o workspace tem ao menos conformance e domain", len(modules))
	}

	// Só a designação de shared kernel é lida do baseline; o store não entra na
	// Input de propósito. Passá-lo faria o teste julgar também a autoridade
	// sobre a classificação (T001/T002), quando o que ele promete é a regra de
	// dependência sobre o universo real.
	doc, ok, err := fsstore.NewBaselineStore(raiz).Baseline()
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	var sharedKernel []string
	if ok {
		sharedKernel = doc.SharedKernelUnits
	}

	grafo := golist.New(raiz, modules, perfis)
	rel, err := conformance.Check(conformance.Input{
		Modules:           modules,
		Manifests:         fsstore.NewManifestStore(fsstore.ModuleDirs(modules)),
		Graph:             grafo,
		Closure:           grafo.Closure,
		Standard:          grafo.IsStandard,
		SharedKernelUnits: sharedKernel,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rel.Diagnostics) != 0 {
		for _, d := range rel.Diagnostics {
			t.Errorf("%s", d)
		}
		t.Fatalf("o verificador NÃO passa no próprio gate: %d diagnóstico(s)", len(rel.Diagnostics))
	}
}

// Se o inventário parar de achar módulos, o teste acima passa por vacuidade:
// verificar zero módulo é sempre conforme.
func TestInventarioEncontraOsModulosDoWorkspace(t *testing.T) {
	modules, err := fsstore.NewInventory(raizDoWorkspace(t)).Modules()
	if err != nil {
		t.Fatalf("inventário: %v", err)
	}
	exigidos := []string{
		"github.com/mateusmacedo/dmpf/tools/dmpf-conformance",
		"github.com/mateusmacedo/dmpf/libs/backend/go/domain",
	}
	for _, e := range exigidos {
		var achou bool
		for _, m := range modules {
			if m.Path == e {
				achou = true
				if !m.HasManifest {
					t.Errorf("%s sem manifesto no inventário", e)
				}
				if !m.HasProduction {
					t.Errorf("%s sem código de produção no inventário", e)
				}
			}
		}
		if !achou {
			t.Errorf("módulo %s ausente do inventário", e)
		}
	}
}

func raizDoWorkspace(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("fora de um repositório git: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// "Zero diagnósticos" também é o resultado de não ter olhado aresta nenhuma:
// aqui as arestas citadas precisam existir MESMO no grafo do módulo.
func TestAutoverificacaoExercitaAsCelulasQuePromete(t *testing.T) {
	raiz := raizDoWorkspace(t)
	const m = "github.com/mateusmacedo/dmpf/tools/dmpf-conformance"

	perfis, err := fsstore.LoadBuildProfiles(
		filepath.Join(raiz, "tools", "dmpf-conformance", "build-profiles.json"))
	if err != nil {
		t.Fatalf("perfis: %v", err)
	}
	modules, err := fsstore.NewInventory(raiz).Modules()
	if err != nil {
		t.Fatalf("inventário: %v", err)
	}
	edges, err := golist.New(raiz, modules, perfis).Edges()
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}

	presente := map[[2]string]bool{}
	for _, e := range edges {
		presente[[2]string{e.From, e.To}] = true
	}

	exigidas := []struct {
		celula   int
		de, para string
		aresta   [2]string
	}{
		{1, "domain", "domain", [2]string{m + "/internal/manifest", m + "/internal/rule"}},
		{7, "application", "domain", [2]string{m + "/internal/conformance", m + "/internal/rule"}},
		{10, "application", "port", [2]string{m + "/internal/conformance", m + "/internal/port"}},
		{14, "app", "application", [2]string{m + "/cmd/conformance", m + "/internal/conformance"}},
		{17, "app", "provider", [2]string{m + "/cmd/conformance", m + "/internal/golist"}},
		{19, "port", "domain", [2]string{m + "/internal/port", m + "/internal/rule"}},
		{25, "provider", "domain", [2]string{m + "/internal/golist", m + "/internal/rule"}},
		{28, "provider", "port", [2]string{m + "/internal/golist", m + "/internal/port"}},
		{29, "provider", "provider", [2]string{m + "/internal/golist", m + "/internal/fsstore"}},
	}
	for _, e := range exigidas {
		if !presente[e.aresta] {
			t.Errorf("célula %d (%s -> %s) não é exercitada: aresta %s -> %s ausente do grafo",
				e.celula, e.de, e.para, e.aresta[0], e.aresta[1])
		}
	}

	// Célula 4 é ciclo de import em Go, então nem compila — o vetor fica para o
	// dia em que a estrutura mudar.
	if presente[[2]string{m + "/internal/rule", m + "/internal/port"}] ||
		presente[[2]string{m + "/internal/manifest", m + "/internal/port"}] {
		t.Error("aresta domain -> port presente no grafo do próprio módulo (ADR-014)")
	}
}
