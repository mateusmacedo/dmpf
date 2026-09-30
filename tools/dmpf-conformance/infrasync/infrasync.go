// Package infrasync lê o manifesto deploy/infra.json de cada app e gera os
// arquivos de infra/ que agregam todas as apps: o provisionamento de banco e de
// Kafka, os certificados do PKI local, o Swagger UI, o .env.example do Compose e
// as listas de apps dos overlays Kubernetes.
package infrasync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	schema       = "dmpf/infra@1"
	manifestGlob = "apps/backend/*/deploy/infra.json"
	platformEnv  = "infra/local/env.platform.example"
	devOverlay   = "apps/backend/%s/deploy/k8s/overlays/dev/kustomization.yaml"
)

var (
	envNamePattern  = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	namePattern     = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	dnsLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	valuePattern    = regexp.MustCompile(`^[A-Za-z0-9._:/@+=,-]+$`)
	aclOperations   = []string{"all", "read", "write", "create", "delete", "alter", "describe", "describe_configs", "alter_configs"}
)

type Manifest struct {
	Schema   string            `json:"schema"`
	App      string            `json:"app"`
	Image    Setting           `json:"image"`
	Database *Database         `json:"database,omitempty"`
	Kafka    *Kafka            `json:"kafka,omitempty"`
	GRPC     GRPC              `json:"grpc"`
	OpenAPI  bool              `json:"openapi,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

type Setting struct {
	Env   string `json:"env"`
	Local string `json:"local"`
}

type Database struct {
	PasswordEnv string `json:"passwordEnv"`
	Local       string `json:"local"`
	Dev         string `json:"dev"`
}

type Kafka struct {
	PasswordEnv string  `json:"passwordEnv"`
	Local       string  `json:"local"`
	Topics      []Topic `json:"topics"`
	ACLs        []ACL   `json:"acls"`
}

type Topic struct {
	Name string `json:"name"`
	Env  string `json:"env"`
}

type ACL struct {
	Operations []string `json:"operations"`
	Topics     []string `json:"topics"`
	Group      *Topic   `json:"group,omitempty"`
}

type GRPC struct {
	Server string `json:"server,omitempty"`
	Client string `json:"client,omitempty"`
}

type File struct {
	Path    string
	Content []byte
}

type Finding struct {
	Path   string
	Detail string
}

func (f Finding) String() string { return f.Path + ": " + f.Detail }

// Load lê os manifestos em ordem de app e recusa o que não gera infra coerente.
func Load(root string) ([]Manifest, error) {
	paths, err := filepath.Glob(filepath.Join(root, manifestGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var manifests []Manifest
	for _, p := range paths {
		m, err := read(p)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	if err := errors.Join(checkUnique(manifests), checkTopics(manifests)); err != nil {
		return nil, err
	}
	return manifests, nil
}

func read(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("%s: conteúdo depois do objeto do manifesto", path)
	}
	dir := filepath.Base(filepath.Dir(filepath.Dir(path)))
	var problems []string
	if m.Schema != schema {
		problems = append(problems, fmt.Sprintf("schema %q, esperado %q", m.Schema, schema))
	}
	if m.App != dir {
		problems = append(problems, fmt.Sprintf("app %q difere do diretório %q", m.App, dir))
	}
	if m.Image.Env == "" || m.Image.Local == "" {
		problems = append(problems, "image exige env e local")
	}
	if d := m.Database; d != nil && (d.PasswordEnv == "" || d.Local == "" || d.Dev == "") {
		problems = append(problems, "database exige passwordEnv, local e dev")
	}
	if k := m.Kafka; k != nil {
		if k.PasswordEnv == "" || k.Local == "" {
			problems = append(problems, "kafka exige passwordEnv e local")
		}
		for _, t := range k.Topics {
			if t.Name == "" || t.Env == "" {
				problems = append(problems, "tópico exige name e env")
			}
		}
		for _, acl := range k.ACLs {
			if len(acl.Operations) == 0 {
				problems = append(problems, "ACL exige operations")
			}
			if len(acl.Topics) == 0 && acl.Group == nil {
				problems = append(problems, "ACL exige topics ou group")
			}
			if g := acl.Group; g != nil && (g.Name == "" || g.Env == "") {
				problems = append(problems, "group da ACL exige name e env")
			}
		}
	}
	problems = append(problems, formatProblems(m)...)
	if len(problems) > 0 {
		return Manifest{}, fmt.Errorf("%s: %s", path, strings.Join(problems, "; "))
	}
	return m, nil
}

// formatProblems recusa o valor que sairia do seu lugar nos arquivos gerados, onde
// ele entra sem escape em shell, SQL, YAML e comandos do rpk.
func formatProblems(m Manifest) []string {
	var problems []string
	check := func(field, value string, pattern *regexp.Regexp, secret bool) {
		if value == "" || pattern.MatchString(value) {
			return
		}
		if secret {
			problems = append(problems, fmt.Sprintf("%s fora do formato %s", field, pattern))
			return
		}
		problems = append(problems, fmt.Sprintf("%s %q fora do formato %s", field, value, pattern))
	}
	check("image.env", m.Image.Env, envNamePattern, false)
	check("image.local", m.Image.Local, valuePattern, false)
	if d := m.Database; d != nil {
		check("database.passwordEnv", d.PasswordEnv, envNamePattern, false)
		check("database.local", d.Local, valuePattern, true)
		check("database.dev", d.Dev, valuePattern, true)
	}
	if k := m.Kafka; k != nil {
		check("kafka.passwordEnv", k.PasswordEnv, envNamePattern, false)
		check("kafka.local", k.Local, valuePattern, true)
		for _, t := range k.Topics {
			check("topic.name", t.Name, namePattern, false)
			check("topic.env", t.Env, envNamePattern, false)
		}
		for _, acl := range k.ACLs {
			for _, op := range acl.Operations {
				if !slices.Contains(aclOperations, op) {
					problems = append(problems, fmt.Sprintf("operations %q fora de %v", op, aclOperations))
				}
			}
			for _, t := range acl.Topics {
				check("acl.topic", t, namePattern, false)
			}
			if g := acl.Group; g != nil {
				check("group.name", g.Name, namePattern, false)
				check("group.env", g.Env, envNamePattern, false)
			}
		}
	}
	check("grpc.server", m.GRPC.Server, dnsLabelPattern, false)
	check("grpc.client", m.GRPC.Client, dnsLabelPattern, false)
	for _, key := range slices.Sorted(maps.Keys(m.Env)) {
		check("env", key, envNamePattern, false)
		check("env."+key, m.Env[key], valuePattern, false)
	}
	return problems
}

func checkTopics(manifests []Manifest) error {
	declared := topicIndex(manifests)
	var problems []string
	for _, m := range manifests {
		if m.Kafka == nil {
			continue
		}
		for _, acl := range m.Kafka.ACLs {
			for _, t := range acl.Topics {
				if _, ok := declared[t]; !ok {
					problems = append(problems, fmt.Sprintf("%s: ACL em tópico não declarado por nenhuma app: %s", m.App, t))
				}
			}
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func checkUnique(manifests []Manifest) error {
	var problems []string
	owners := map[string]string{}
	for _, m := range manifests {
		if m.Kafka == nil {
			continue
		}
		for _, t := range m.Kafka.Topics {
			if owner, taken := owners[t.Name]; taken {
				problems = append(problems, fmt.Sprintf("tópico %s declarado por %s e por %s", t.Name, owner, m.App))
				continue
			}
			owners[t.Name] = m.App
		}
	}
	declared := map[string]bool{}
	for _, line := range (view{Manifests: manifests}).EnvLines() {
		key, _, _ := strings.Cut(line, "=")
		if declared[key] {
			problems = append(problems, fmt.Sprintf("variável %s declarada mais de uma vez no .env.example", key))
		}
		declared[key] = true
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func topicIndex(manifests []Manifest) map[string]Topic {
	index := map[string]Topic{}
	for _, m := range manifests {
		if m.Kafka != nil {
			for _, t := range m.Kafka.Topics {
				index[t.Name] = t
			}
		}
	}
	return index
}

// Render gera os arquivos agregados, sempre na mesma ordem e com o mesmo conteúdo
// para os mesmos manifestos.
func Render(root string, manifests []Manifest) ([]File, error) {
	platform, err := os.ReadFile(filepath.Join(root, platformEnv))
	if err != nil {
		return nil, fmt.Errorf("plataforma do .env.example: %w", err)
	}
	var files []File
	for _, t := range templates {
		v := view{Manifests: manifests, Overlay: t.env, topics: topicIndex(manifests)}
		var buf bytes.Buffer
		if err := parsed.ExecuteTemplate(&buf, t.template, v); err != nil {
			return nil, fmt.Errorf("%s: %w", t.path, err)
		}
		content := buf.Bytes()
		if t.path == envExamplePath {
			content = append(append(bytes.TrimRight(platform, "\n"), '\n', '\n'), content...)
		}
		files = append(files, File{Path: t.path, Content: content})
	}
	return files, nil
}

func Write(root string, files []File) error {
	for _, f := range files {
		path := filepath.Join(root, f.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func Check(root string, files []File) []Finding {
	var findings []Finding
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(root, f.Path))
		switch {
		case errors.Is(err, os.ErrNotExist):
			findings = append(findings, Finding{f.Path, "ausente; rode infrasync --write"})
		case err != nil:
			findings = append(findings, Finding{f.Path, err.Error()})
		case !bytes.Equal(got, f.Content):
			findings = append(findings, Finding{f.Path, "diverge dos manifestos deploy/infra.json; rode infrasync --write"})
		}
	}
	return findings
}

// CheckDevSecrets confere que o Secret do overlay dev de cada app com banco usa a
// senha database.dev do manifesto, a mesma com que o Job de bancos cria o role.
func CheckDevSecrets(root string, manifests []Manifest) []Finding {
	var findings []Finding
	for _, m := range manifests {
		if m.Database == nil {
			continue
		}
		rel := fmt.Sprintf(devOverlay, m.App)
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			findings = append(findings, Finding{rel, err.Error()})
			continue
		}
		if !bytes.Contains(raw, []byte("PG_DSN=postgres://"+m.App+":"+m.Database.Dev+"@")) {
			findings = append(findings, Finding{rel, "o PG_DSN não usa a senha database.dev do deploy/infra.json"})
		}
	}
	return findings
}

type view struct {
	Manifests []Manifest
	Overlay   string
	topics    map[string]Topic
}

func (v view) WithDatabase() []Manifest {
	return slices.DeleteFunc(slices.Clone(v.Manifests), func(m Manifest) bool { return m.Database == nil })
}

func (v view) WithKafka() []Manifest {
	return slices.DeleteFunc(slices.Clone(v.Manifests), func(m Manifest) bool { return m.Kafka == nil })
}

func (v view) WithOpenAPI() []Manifest {
	return slices.DeleteFunc(slices.Clone(v.Manifests), func(m Manifest) bool { return !m.OpenAPI })
}

func (v view) Topics() []Topic {
	var out []Topic
	for _, m := range v.WithKafka() {
		out = append(out, m.Kafka.Topics...)
	}
	return out
}

// Ref é a referência Compose ao tópico: a variável da app dona, com o nome como default.
func (v view) Ref(name string) string {
	t := v.topics[name]
	return fmt.Sprintf("${%s:-%s}", t.Env, t.Name)
}

func (v view) EnvLines() []string {
	var lines []string
	for _, m := range v.Manifests {
		lines = append(lines, m.Image.Env+"="+m.Image.Local)
	}
	for _, m := range v.Manifests {
		if m.Database != nil {
			lines = append(lines, m.Database.PasswordEnv+"="+m.Database.Local)
		}
	}
	for _, m := range v.Manifests {
		if m.Kafka == nil {
			continue
		}
		lines = append(lines, m.Kafka.PasswordEnv+"="+m.Kafka.Local)
		for _, t := range m.Kafka.Topics {
			lines = append(lines, t.Env+"="+t.Name)
		}
		for _, acl := range m.Kafka.ACLs {
			if acl.Group != nil {
				lines = append(lines, acl.Group.Env+"="+acl.Group.Name)
			}
		}
	}
	for _, m := range v.Manifests {
		keys := make([]string, 0, len(m.Env))
		for k := range m.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, k+"="+m.Env[k])
		}
	}
	return lines
}
