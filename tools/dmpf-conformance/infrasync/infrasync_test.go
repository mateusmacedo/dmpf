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

func TestLoadRefusesAnAppOutsideADNSLabel(t *testing.T) {
	root := workspace(t)
	if err := os.CopyFS(filepath.Join(root, "apps/backend/al.pha"), os.DirFS(filepath.Join(root, "apps/backend/alpha"))); err != nil {
		t.Fatal(err)
	}
	rewrite(t, root, "apps/backend/al.pha/deploy/infra.json", `"app": "alpha"`, `"app": "al.pha"`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "al.pha") {
		t.Fatalf("Load = %v, want the app outside a DNS label refused", err)
	}
}

func TestRenderWithoutAppsDropsNoPodByTheAppLabel(t *testing.T) {
	files, err := infrasync.Render(workspace(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "alloy-kubernetes.alloy") && strings.Contains(string(f.Content), `regex         = ""`) {
			t.Fatalf("regex vazio descartaria os pods sem o label de app:\n%s", f.Content)
		}
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

func TestLoadRefusesContentAfterTheManifest(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/edge/deploy/infra.json", `"env": { "HTTP_PORT": "8080" }
}`, `"env": { "HTTP_PORT": "8080" }
}
{ "app": "ghost" }`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "edge") {
		t.Fatalf("Load = %v, want the manifest with trailing content named", err)
	}
}

func TestLoadRefusesATopicDeclaredByTwoApps(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/beta/deploy/infra.json", `{ "name": "beta.events", "env": "KAFKA_BETA_TOPIC" }`, `{ "name": "alpha.events", "env": "KAFKA_BETA_TOPIC" }`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "alpha.events") {
		t.Fatalf("Load = %v, want the topic declared twice named", err)
	}
}

func TestLoadRefusesAVariableDeclaredTwice(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/edge/deploy/infra.json", `"HTTP_PORT": "8080"`, `"ITEM_LIMIT": "5"`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "ITEM_LIMIT") {
		t.Fatalf("Load = %v, want the variable declared twice named", err)
	}
}

func TestLoadRefusesAnACLWithoutOperations(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/alpha/deploy/infra.json", `{ "operations": ["write", "describe"], "topics": ["alpha.events", "alpha.events.dlq"] }`, `{ "operations": [], "topics": ["alpha.events", "alpha.events.dlq"] }`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "operations") {
		t.Fatalf("Load = %v, want the ACL without operations refused", err)
	}
}

func TestLoadRefusesAGroupWithoutEnv(t *testing.T) {
	root := workspace(t)
	rewrite(t, root, "apps/backend/beta/deploy/infra.json", `"group": { "name": "beta", "env": "KAFKA_GROUP" }`, `"group": { "name": "beta", "env": "" }`)
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "group") {
		t.Fatalf("Load = %v, want the group without env refused", err)
	}
}

func TestLoadRefusesAValueThatWouldBreakOutOfTheGeneratedFiles(t *testing.T) {
	for _, tc := range []struct {
		name, file, old, new, field string
	}{
		{"variável de senha com comando", "alpha", `"passwordEnv": "ALPHA_PG_PASSWORD"`, `"passwordEnv": "X; id > /tmp/evil; :"`, "passwordEnv"},
		{"senha de dev com aspas", "alpha", `"dev": "alpha-dev"`, `"dev": "alpha'dev"`, "dev"},
		{"senha local com quebra de linha", "alpha", `"local": "alpha-local", "dev"`, `"local": "alpha\nlocal", "dev"`, "local"},
		{"imagem com separador de comando", "alpha", `"local": "alpha:local"`, `"local": "alpha:local; x"`, "image"},
		{"servidor gRPC fora de rótulo DNS", "alpha", `"server": "dmpf-alpha-api"`, `"server": "api; touch /pki/pwned"`, "grpc"},
		{"operação fora do rpk", "alpha", `"operations": ["write", "describe"]`, `"operations": ["write --allow-principal User:*", "describe"]`, "operations"},
		{"tópico com espaço", "alpha", `{ "name": "alpha.events.dlq", "env": "KAFKA_ALPHA_DLQ" }`, `{ "name": "alpha events", "env": "KAFKA_ALPHA_DLQ" }`, "alpha events"},
		{"chave de ambiente minúscula", "alpha", `"ITEM_LIMIT": "10"`, `"item limit": "10"`, "item limit"},
		{"valor de ambiente com substituição", "alpha", `"ITEM_LIMIT": "10"`, `"ITEM_LIMIT": "$(id)"`, "ITEM_LIMIT"},
		{"grupo com aspas", "beta", `"group": { "name": "beta", "env": "KAFKA_GROUP" }`, `"group": { "name": "beta'", "env": "KAFKA_GROUP" }`, "group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := workspace(t)
			rewrite(t, root, "apps/backend/"+tc.file+"/deploy/infra.json", tc.old, tc.new)
			if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Load = %v, want the value refused naming %q", err, tc.field)
			}
		})
	}
}

func TestCheckDevSecretsReportsADevPasswordThatDiverges(t *testing.T) {
	root := workspace(t)
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if findings := infrasync.CheckDevSecrets(root, manifests); len(findings) != 0 {
		t.Fatalf("CheckDevSecrets = %v, want none while the overlays agree", findings)
	}

	rewrite(t, root, "apps/backend/beta/deploy/k8s/overlays/dev/kustomization.yaml", "beta:beta-dev@", "beta:outra@")
	findings := infrasync.CheckDevSecrets(root, manifests)
	if len(findings) != 1 || !strings.Contains(findings[0].Path, "beta") {
		t.Fatalf("CheckDevSecrets = %v, want the beta overlay named", findings)
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
