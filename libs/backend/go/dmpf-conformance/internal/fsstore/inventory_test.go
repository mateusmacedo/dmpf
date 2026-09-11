package fsstore_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/fsstore"
)

// O inventário descobre a partir do que o git rastreia: sem commit, não há fonte.
func repoTemporario(t *testing.T, arquivos map[string]string) string {
	t.Helper()
	raiz := t.TempDir()
	for nome, conteudo := range arquivos {
		p := filepath.Join(raiz, nome)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "-qm", "fixture"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = raiz
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return raiz
}

// Dentro de um hook o git exporta GIT_DIR e GIT_INDEX_FILE; herdá-los faz o
// `git init` do fixture gravar na branch real e o `git ls-files` sob teste ler o
// repositório errado (aconteceu no pre-push do Lefthook).
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if nome, _, _ := strings.Cut(kv, "="); strings.HasPrefix(nome, "GIT_") {
			_ = os.Unsetenv(nome)
		}
	}
	os.Exit(m.Run())
}

const goModValido = "module exemplo.test/m\n\ngo 1.26.4\n"

func TestInventarioAceitaModuloValido(t *testing.T) {
	raiz := repoTemporario(t, map[string]string{
		"m/go.mod": goModValido,
		"m/p/p.go": "package p\n\nfunc F() {}\n",
	})
	mods, err := fsstore.NewInventory(raiz).Modules()
	if err != nil {
		t.Fatalf("Modules: %v", err)
	}
	if len(mods) != 1 || mods[0].Path != "exemplo.test/m" {
		t.Fatalf("inventário = %+v", mods)
	}
	if mods[0].HasManifest {
		t.Error("módulo sem dmpf-units.json marcado com manifesto")
	}
	if !mods[0].HasProduction {
		t.Error("módulo com .go de produção marcado sem produção")
	}
}

// Sumir esconderia o U004 do módulo e os U001 dos packages dele: o verificador
// ficaria verde por não ter olhado.
func TestGoModSemDiretivaTornaExecucaoNaoVerificavel(t *testing.T) {
	raiz := repoTemporario(t, map[string]string{
		"m/go.mod": "go 1.26.4\n", // sem a diretiva `module`
		"m/p/p.go": "package p\n",
	})
	mods, err := fsstore.NewInventory(raiz).Modules()
	if err == nil {
		t.Fatalf("go.mod inválido não interrompeu a execução; inventário = %+v", mods)
	}
	if !strings.Contains(err.Error(), "não é verificável") {
		t.Errorf("erro não diz que a execução ficou não verificável: %v", err)
	}
}

func TestProjectJsonInvalidoTornaExecucaoNaoVerificavel(t *testing.T) {
	raiz := repoTemporario(t, map[string]string{
		"m/go.mod":       goModValido,
		"m/p/p.go":       "package p\n",
		"m/project.json": "{ isto nao e json",
	})
	if _, err := fsstore.NewInventory(raiz).Modules(); err == nil {
		t.Fatal("project.json inválido passou batido")
	}
}

// Tratar fixture como produção faria o verificador reprovar o repositório pela
// violação que ela existe para demonstrar.
func TestModuloDeTestdataFicaForaDoInventario(t *testing.T) {
	raiz := repoTemporario(t, map[string]string{
		"m/go.mod":            goModValido,
		"m/p/p.go":            "package p\n",
		"m/testdata/f/go.mod": "module exemplo.test/fixture\n\ngo 1.26.4\n",
		"m/testdata/f/q/q.go": "package q\n",
	})
	mods, err := fsstore.NewInventory(raiz).Modules()
	if err != nil {
		t.Fatalf("Modules: %v", err)
	}
	for _, m := range mods {
		if strings.Contains(m.Dir, "testdata") {
			t.Errorf("módulo de testdata no inventário: %s", m.Path)
		}
	}
	if len(mods) != 1 {
		t.Errorf("esperado 1 módulo, got %d: %+v", len(mods), mods)
	}
}

// A marca decide o GOWORK da extração, e com ele a resolução de um require
// entre membros sem `replace` local.
func TestMembroDoGoWorkEMarcado(t *testing.T) {
	raiz := repoTemporario(t, map[string]string{
		"go.work":  "go 1.26.4\n\nuse ./m\n",
		"m/go.mod": goModValido,
		"m/p/p.go": "package p\n",
		"f/go.mod": "module exemplo.test/f\n\ngo 1.26.4\n",
		"f/p/p.go": "package p\n",
	})
	mods, err := fsstore.NewInventory(raiz).Modules()
	if err != nil {
		t.Fatalf("Modules: %v", err)
	}
	marcado := map[string]bool{}
	for _, m := range mods {
		marcado[m.Path] = m.WorkspaceMember
	}
	if !marcado["exemplo.test/m"] {
		t.Error("membro do go.work não marcado")
	}
	if marcado["exemplo.test/f"] {
		t.Error("módulo fora do go.work marcado como membro")
	}
}

// Devolver inventário vazio seria o pior resultado: verificar zero módulo é
// sempre conforme.
func TestRaizInexistenteNaoPassaPorVazio(t *testing.T) {
	mods, err := fsstore.NewInventory(filepath.Join(t.TempDir(), "nao-existe")).Modules()
	if err == nil {
		t.Fatalf("raiz inexistente devolveu inventário sem erro: %+v", mods)
	}
}

// Sem git, a fonte que não depende de configuração some: seguir com as outras
// daria universo parcial apresentado como completo.
func TestDiretorioSemRepositorioGitInterrompe(t *testing.T) {
	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "go.mod"), []byte(goModValido), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fsstore.NewInventory(raiz).Modules(); err == nil {
		t.Fatal("diretório sem repositório git não interrompeu")
	}
}
