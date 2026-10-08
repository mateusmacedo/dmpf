package infrasync_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/infrasync"
)

var layoutDoPlatform = infrasync.Layout{AppsDir: "apps/backend", Edge: "bff", SpiffeTrustDomain: "dmpf", ComposeProfile: "dmpf"}

func escreverDmpfJSON(t *testing.T, root, conteudo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "dmpf.json"), []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSemDmpfJSONEhODoPlatform(t *testing.T) {
	got, err := infrasync.LoadLayout(t.TempDir())
	if err != nil || got != layoutDoPlatform {
		t.Fatalf("LoadLayout = %+v, %v; esperado %+v", got, err, layoutDoPlatform)
	}
}

func TestLayoutLeODmpfJSONEIgnoraCamposDeOutrosLeitores(t *testing.T) {
	root := t.TempDir()
	escreverDmpfJSON(t, root, `{"schema":"dmpf/workspace@1","modulePrefix":"example.com/acme","appsDir":"services","edge":"gateway","spiffeTrustDomain":"acme.test","composeProfile":"acme"}`)

	got, err := infrasync.LoadLayout(root)
	want := infrasync.Layout{AppsDir: "services", Edge: "gateway", SpiffeTrustDomain: "acme.test", ComposeProfile: "acme"}
	if err != nil || got != want {
		t.Fatalf("LoadLayout = %+v, %v; esperado %+v", got, err, want)
	}
}

func TestLayoutCampoAusenteUsaODoPlatform(t *testing.T) {
	root := t.TempDir()
	escreverDmpfJSON(t, root, `{"schema":"dmpf/workspace@1","appsDir":"services"}`)

	got, err := infrasync.LoadLayout(root)
	want := layoutDoPlatform
	want.AppsDir = "services"
	if err != nil || got != want {
		t.Fatalf("LoadLayout = %+v, %v; esperado %+v", got, err, want)
	}
}

func TestLayoutRecusaValorQueEscapariaDoArquivoGerado(t *testing.T) {
	casos := map[string]string{
		"json inválido":       `{ isto nao e json`,
		"schema errado":       `{"schema":"dmpf/workspace@2"}`,
		"appsDir com ..":      `{"schema":"dmpf/workspace@1","appsDir":"../fora"}`,
		"appsDir absoluto":    `{"schema":"dmpf/workspace@1","appsDir":"/srv/apps"}`,
		"edge fora de DNS":    `{"schema":"dmpf/workspace@1","edge":"Edge_1"}`,
		"trust domain espaço": `{"schema":"dmpf/workspace@1","spiffeTrustDomain":"acme test"}`,
		"profile vazio":       `{"schema":"dmpf/workspace@1","composeProfile":""}`,
	}
	for nome, conteudo := range casos {
		t.Run(nome, func(t *testing.T) {
			root := t.TempDir()
			escreverDmpfJSON(t, root, conteudo)
			if l, err := infrasync.LoadLayout(root); err == nil {
				t.Fatalf("dmpf.json inválido aceito: %+v", l)
			}
		})
	}
}

func workspaceComLayout(t *testing.T, dmpfJSON string) string {
	t.Helper()
	root := workspace(t)
	if err := os.Rename(filepath.Join(root, "apps", "backend"), filepath.Join(root, "services")); err != nil {
		t.Fatal(err)
	}
	escreverDmpfJSON(t, root, dmpfJSON)
	return root
}

const layoutAcme = `{"schema":"dmpf/workspace@1","appsDir":"services","edge":"gateway","spiffeTrustDomain":"acme.test","composeProfile":"acme"}`

func TestRenderSegueOLayoutDoDmpfJSON(t *testing.T) {
	files := render(t, workspaceComLayout(t, layoutAcme))

	esperados := map[string][]string{
		"infra/local/compose/pki.yml":                    {"spiffe://acme.test/$$1", "profiles: [acme]", "services/*/deploy/infra.json"},
		"infra/local/compose/swagger-ui.yml":             {"acme-gateway:", "profiles: [acme]"},
		"infra/local/compose/provisioning.generated.yml": {"profiles: [acme, provisioning]"},
		"infra/local/.env.example":                       {"(profile acme)"},
		"infra/k8s/overlays/dev/apps/kustomization.yaml": {"../../../../../services/alpha/deploy/k8s/overlays/dev\n"},
	}
	for path, trechos := range esperados {
		for _, trecho := range trechos {
			if !strings.Contains(files[path], trecho) {
				t.Errorf("%s sem %q:\n%s", path, trecho, files[path])
			}
		}
	}
	for path, conteudo := range files {
		for _, fixo := range []string{"spiffe://dmpf", "[dmpf", "dmpf-bff", "apps/backend"} {
			if strings.Contains(conteudo, fixo) {
				t.Errorf("%s ainda fixa %q", path, fixo)
			}
		}
	}
}

func TestRenderSemBordaNaoDependeDeServicoInexistente(t *testing.T) {
	swagger := render(t, workspaceComLayout(t, `{"schema":"dmpf/workspace@1","appsDir":"services","edge":""}`))["infra/local/compose/swagger-ui.yml"]
	if strings.Contains(swagger, "depends_on") {
		t.Errorf("swagger-ui depende de borda que o workspace não tem:\n%s", swagger)
	}
}

func TestCheckDevSecretsSegueOAppsDir(t *testing.T) {
	root := workspaceComLayout(t, layoutAcme)
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if findings := infrasync.CheckDevSecrets(root, manifests); len(findings) != 0 {
		t.Fatalf("CheckDevSecrets com appsDir services: %v", findings)
	}
}

func TestRenderGeraODescarteDasAppsNoAlloy(t *testing.T) {
	alloy := render(t, workspace(t))["infra/observability/alloy/alloy-kubernetes.alloy"]
	if !strings.Contains(alloy, `regex         = "alpha|beta|edge"`) {
		t.Fatalf("Alloy sem a lista de apps gerada dos manifestos:\n%s", alloy)
	}
}
