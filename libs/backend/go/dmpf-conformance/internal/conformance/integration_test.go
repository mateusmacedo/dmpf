package conformance_test

import (
	"path/filepath"
	"slices"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/conformance"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/fsstore"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/golist"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

const fixtures = "../golist/testdata"

var perfilLinuxAmd64 = []fsstore.BuildProfile{
	{ID: "linux-amd64", GOOS: "linux", GOARCH: "amd64", CGOEnabled: false},
}

// Inventário montado à mão porque a fixture não é um repositório git: sob teste
// aqui estão a extração e a decisão, não a descoberta.
func rodar(t *testing.T, cenario string, mods ...string) conformance.Report {
	t.Helper()

	var modules []rule.Module
	for _, m := range mods {
		dir, err := filepath.Abs(filepath.Join(fixtures, cenario, filepath.Base(m)))
		if err != nil {
			t.Fatal(err)
		}
		modules = append(modules, rule.Module{
			Path: m, Dir: dir, HasManifest: true, HasProduction: true,
		})
	}

	grafo := golist.New("", modules, perfilLinuxAmd64)
	rel, err := conformance.Check(conformance.Input{
		Modules:   modules,
		Manifests: fsstore.NewManifestStore(fsstore.ModuleDirs(modules)),
		Graph:     grafo,
		Closure:   grafo.Closure,
		Standard:  grafo.IsStandard,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return rel
}

func codigos(ds []rule.Diagnostic) []rule.Code {
	out := make([]rule.Code, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
}

// O vetor que justifica este verificador existir: o `domain` do mod-a alcança
// `net/http` por dentro do mod-b, e o `depguard` passa verde nos dois porque
// analisa um módulo por vez e seleciona por nome de diretório.
func TestArestaTransitivaEntreModulos(t *testing.T) {
	rel := rodar(t, "transitivo", "exemplo.test/mod-a", "exemplo.test/mod-b")

	if len(rel.Diagnostics) == 0 {
		t.Fatal("cenário transitivo passou verde: o verificador não atravessou a fronteira de módulo")
	}
	if !slices.Contains(codigos(rel.Diagnostics), rule.CodeE001) {
		t.Fatalf("esperado E001 no alcance de net/http; got %v", rel.Diagnostics)
	}

	var achou bool
	for _, d := range rel.Diagnostics {
		if d.Code == rule.CodeE001 && d.Target == "net/http" &&
			d.CanonicalKey == "exemplo.test/mod-b/util" {
			achou = true
			if d.SourceFile == "" {
				t.Error("diagnóstico sem o arquivo de origem que introduz a aresta")
			}
		}
	}
	if !achou {
		t.Errorf("E001 não aponta mod-b/util -> net/http: %v", rel.Diagnostics)
	}
}

// A aresta interna precisa ser DECIDIDA, não só não reprovar: afirmar ausência
// passaria também quando a extração parasse de produzir arestas internas.
func TestArestaInternaEntreModulosProduzD001(t *testing.T) {
	rel := rodar(t, "arestainterna", "exemplo.test/ai-a", "exemplo.test/ai-b")

	var achou bool
	for _, d := range rel.Diagnostics {
		if d.Code == rule.CodeD001 &&
			d.CanonicalKey == "exemplo.test/ai-a/domain" &&
			d.Target == "exemplo.test/ai-b/provider" {
			achou = true
		}
		if d.Code == rule.CodeE001 && d.Target == "exemplo.test/ai-b/provider" {
			t.Error("aresta interna tratada como dependência externa")
		}
	}
	if !achou {
		t.Fatalf("aresta interna domain -> provider entre módulos não produziu D001: %v", rel.Diagnostics)
	}
}

// O positivo do par: domain -> domain no mesmo contexto é permitido.
func TestArestaInternaPermitidaNaoReprova(t *testing.T) {
	rel := rodar(t, "transitivo", "exemplo.test/mod-a", "exemplo.test/mod-b")
	for _, d := range rel.Diagnostics {
		if d.CanonicalKey == "exemplo.test/mod-a/domain" && d.Target == "exemplo.test/mod-b/util" {
			t.Errorf("domain -> domain no mesmo context reprovou (célula 1 é P): %s", d)
		}
	}
}

// Com um módulo só, o alcance fica invisível — é o que o golangci-lint por
// módulo faz, e o motivo de o lint sozinho ser declaradamente parcial.
func TestModuloSozinhoNaoVeATransitividade(t *testing.T) {
	rel := rodar(t, "transitivo", "exemplo.test/mod-a")
	for _, d := range rel.Diagnostics {
		if d.Code == rule.CodeE001 && d.Target == "net/http" {
			t.Fatal("net/http detectado com um módulo só: a fixture perdeu a propriedade que documenta")
		}
	}
}
