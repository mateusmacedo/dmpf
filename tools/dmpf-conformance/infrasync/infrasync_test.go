package infrasync_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/infrasync"
)

func workspace(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS("testdata/workspace")); err != nil {
		t.Fatal(err)
	}
	return dst
}

func render(t *testing.T, root string) map[string]string {
	t.Helper()
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	files, err := infrasync.Render(root, manifests)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[f.Path] = string(f.Content)
	}
	return out
}

func TestLoadReadsEveryManifestSortedByApp(t *testing.T) {
	manifests, err := infrasync.Load(workspace(t))
	if err != nil {
		t.Fatal(err)
	}
	var apps []string
	for _, m := range manifests {
		apps = append(apps, m.App)
	}
	if got := strings.Join(apps, ","); got != "alpha,beta,edge" {
		t.Fatalf("apps = %s, want alpha,beta,edge", got)
	}
}

func TestLoadRefusesAManifestWhoseAppIsNotItsDirectory(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/alpha/deploy/infra.json", `"app": "alpha"`, `"app": "gamma"`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "gamma") {
		t.Fatalf("Load = %v, want the app/directory mismatch named", err)
	}
}

func TestLoadRefusesAnACLOnAnUndeclaredTopic(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/beta/deploy/infra.json", `"topics": ["alpha.events"]`, `"topics": ["ghost.events"]`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "ghost.events") {
		t.Fatalf("Load = %v, want the undeclared topic named", err)
	}
}

func TestLoadRefusesAnUnknownField(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/alpha/deploy/infra.json", `"app": "alpha",`, `"app": "alpha", "replicas": 3,`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "replicas") {
		t.Fatalf("Load = %v, want the unknown field named", err)
	}
}

func TestRenderProvisionsEveryDatabaseUserTopicAndACL(t *testing.T) {
	files := render(t, workspace(t))
	prov := files["infra/local/compose/provisioning.generated.yml"]
	for _, want := range []string{
		"for pair in alpha:${ALPHA_PG_PASSWORD:-alpha-local}",
		"beta:${BETA_PG_PASSWORD:-beta-local}",
		`rpk security user create "$${user%%:*}"`,
		"alpha:${KAFKA_ALPHA_PASSWORD:-alpha-local}",
		"for user in alpha beta console; do",
		"${KAFKA_ALPHA_TOPIC:-alpha.events}",
		"--allow-principal User:beta --operation read,describe",
		"--topic ${KAFKA_ALPHA_TOPIC:-alpha.events} --group ${KAFKA_GROUP:-beta}",
	} {
		if !strings.Contains(prov, want) {
			t.Errorf("provisioning sem %q:\n%s", want, prov)
		}
	}
	if strings.Contains(prov, "edge:$") {
		t.Error("app sem banco nem Kafka entrou no provisionamento")
	}
}

func TestRenderIssuesACertificatePerGRPCIdentity(t *testing.T) {
	pki := render(t, workspace(t))["infra/local/compose/pki.yml"]
	for _, want := range []string{"server dmpf-alpha-api\n", "server dmpf-beta-api\n", "client edge\n"} {
		if !strings.Contains(pki, want) {
			t.Errorf("pki sem %q", want)
		}
	}
}

func TestRenderListsTheAppsInEveryOverlay(t *testing.T) {
	files := render(t, workspace(t))
	for _, env := range []string{"dev", "hmg"} {
		got := files["infra/k8s/overlays/"+env+"/apps/kustomization.yaml"]
		for _, app := range []string{"alpha", "beta", "edge"} {
			if !strings.Contains(got, "../../../../../apps/backend/"+app+"/deploy/k8s/overlays/"+env+"\n") {
				t.Errorf("%s sem o overlay de %s:\n%s", env, app, got)
			}
		}
	}
	dbs := files["infra/k8s/overlays/dev/databases/kustomization.yaml"]
	if !strings.Contains(dbs, "- alpha=alpha-dev") || strings.Contains(dbs, "edge=") {
		t.Errorf("secret de bancos do dev errado:\n%s", dbs)
	}
}

func TestRenderPutsThePlatformEnvBeforeTheApps(t *testing.T) {
	env := render(t, workspace(t))["infra/local/.env.example"]
	platform := strings.Index(env, "PLATFORM_PORT=1")
	app := strings.Index(env, "ALPHA_PG_PASSWORD=alpha-local")
	if platform < 0 || app < 0 || platform > app {
		t.Fatalf(".env.example sem a plataforma antes das apps:\n%s", env)
	}
	for _, want := range []string{"KAFKA_GROUP=beta", "ITEM_LIMIT=10", "EDGE_IMAGE=edge:local"} {
		if !strings.Contains(env, want) {
			t.Errorf(".env.example sem %q", want)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	root := workspace(t)
	first, second := render(t, root), render(t, root)
	for path, content := range first {
		if second[path] != content {
			t.Fatalf("%s mudou entre duas gerações", path)
		}
	}
}

func TestWriteThenCheckIsClean(t *testing.T) {
	root := workspace(t)
	manifests, _ := infrasync.Load(root)
	files, _ := infrasync.Render(root, manifests)
	if findings := infrasync.Check(root, files); len(findings) == 0 {
		t.Fatal("Check antes do Write não reprovou arquivos ausentes")
	}
	if err := infrasync.Write(root, files); err != nil {
		t.Fatal(err)
	}
	if findings := infrasync.Check(root, files); len(findings) != 0 {
		t.Fatalf("Check depois do Write = %v", findings)
	}
}

func TestCheckReportsDriftFromTheManifests(t *testing.T) {
	root := workspace(t)
	manifests, _ := infrasync.Load(root)
	files, _ := infrasync.Render(root, manifests)
	if err := infrasync.Write(root, files); err != nil {
		t.Fatal(err)
	}
	rewrite(t, root, "apps/backend/alpha/deploy/infra.json", `"local": "alpha-local", "dev"`, `"local": "alpha-trocada", "dev"`)
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	files, _ = infrasync.Render(root, manifests)
	findings := infrasync.Check(root, files)
	if len(findings) == 0 || !strings.Contains(findings[0].String(), "provisioning.generated.yml") {
		t.Fatalf("Check = %v, want drift no provisionamento", findings)
	}
}

func rewrite(t *testing.T, root, rel, old, new string) {
	t.Helper()
	path := filepath.Join(root, rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), old) {
		t.Fatalf("%s não contém %q", rel, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}
