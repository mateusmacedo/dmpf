package fsstore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/fsstore"
)

func escrever(t *testing.T, conteudo string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "build-profiles.json")
	if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Sem perfil declarado não se sabe o que excluir por build tag, e não saber
// reprova: nenhum destes casos vira conjunto vazio válido.
func TestPerfisInvalidosInterrompem(t *testing.T) {
	casos := map[string]string{
		"json inválido":     "{ isto nao e json",
		"schema errado":     `{"schema":"dmpf/build-profiles@2","profiles":[{"id":"a","goos":"linux","goarch":"amd64"}]}`,
		"lista vazia":       `{"schema":"dmpf/build-profiles@1","profiles":[]}`,
		"perfil sem goos":   `{"schema":"dmpf/build-profiles@1","profiles":[{"id":"a","goarch":"amd64"}]}`,
		"perfil sem id":     `{"schema":"dmpf/build-profiles@1","profiles":[{"goos":"linux","goarch":"amd64"}]}`,
		"perfil sem goarch": `{"schema":"dmpf/build-profiles@1","profiles":[{"id":"a","goos":"linux"}]}`,
	}
	for nome, conteudo := range casos {
		t.Run(nome, func(t *testing.T) {
			if perfis, err := fsstore.LoadBuildProfiles(escrever(t, conteudo)); err == nil {
				t.Fatalf("configuração inválida aceita: %+v", perfis)
			}
		})
	}
}

func TestArquivoDePerfisAusenteInterrompe(t *testing.T) {
	if _, err := fsstore.LoadBuildProfiles(filepath.Join(t.TempDir(), "nao-existe.json")); err == nil {
		t.Fatal("arquivo ausente aceito")
	}
}

func TestPerfilValido(t *testing.T) {
	perfis, err := fsstore.LoadBuildProfiles(escrever(t,
		`{"schema":"dmpf/build-profiles@1","profiles":[{"id":"a","goos":"linux","goarch":"amd64","cgo_enabled":false,"tags":[]}]}`))
	if err != nil {
		t.Fatalf("perfil válido rejeitado: %v", err)
	}
	if len(perfis) != 1 || perfis[0].ID != "a" {
		t.Errorf("perfis = %+v", perfis)
	}
}
