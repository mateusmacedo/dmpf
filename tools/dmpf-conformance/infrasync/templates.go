package infrasync

import (
	"embed"
	"strings"
	"text/template"
)

const envExamplePath = "infra/local/.env.example"

//go:embed templates/*.tmpl
var templateFS embed.FS

var parsed = template.Must(template.New("").Funcs(template.FuncMap{
	"join": strings.Join,
}).ParseFS(templateFS, "templates/*.tmpl"))

type target struct {
	path     string
	template string
	env      string
}

var templates = []target{
	{"infra/local/compose/provisioning.generated.yml", "provisioning.yml.tmpl", ""},
	{"infra/local/compose/pki.yml", "pki.yml.tmpl", ""},
	{"infra/local/compose/swagger-ui.yml", "swagger-ui.yml.tmpl", ""},
	{envExamplePath, "env.tmpl", ""},
	{"infra/k8s/overlays/dev/apps/kustomization.yaml", "apps-kustomization.yaml.tmpl", "dev"},
	{"infra/k8s/overlays/hmg/apps/kustomization.yaml", "apps-kustomization.yaml.tmpl", "hmg"},
	{"infra/k8s/overlays/dev/databases/kustomization.yaml", "databases-kustomization.yaml.tmpl", ""},
	{"infra/k8s/overlays/dev/databases/job.yaml", "databases-job.yaml.tmpl", ""},
	{"infra/observability/alloy/alloy-kubernetes.alloy", "alloy-kubernetes.alloy.tmpl", ""},
}
