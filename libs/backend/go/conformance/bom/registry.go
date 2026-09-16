package bom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

const semconvFile = "libs/backend/go/observability/otelboot/start.go"

// As matérias da tabela de BOM-06: toolchain de stack, pins de geração e o
// catálogo de telemetria do FND-08.
var registrySubjects = []Subject{SubjectRuntime, SubjectGenerator, SubjectSemconv}

type selectorRule struct {
	accepts func(RegistryRef) bool
	extract func(raw string, ref RegistryRef) (string, bool)
	bind    func(raw string, ref RegistryRef, identity, resolved string) string
}

var selectorRules = []selectorRule{
	{exactRef("go.work", "go"), fromRegexp(`(?m)^go\s+(\S+)\s*$`), identityIs("go")},
	{exactRef("tools/buf.sh", "buf"), fromRegexp(`github\.com/bufbuild/buf/cmd/buf@(\S+)`), baseIs("buf")},
	{prefixedRef("contracts/buf.gen.yaml", "plugin:"), pluginVersion, baseIsSelector("plugin:")},
	{exactRef("nx.json", "golangci-lint"), fromRegexp(`/golangci-lint@([^\s"]+)`), baseIs("golangci-lint")},
	{inModule("go.mod", func(s string) bool { return strings.HasPrefix(s, "require:") && len(s) > len("require:") }), requireVersion, identityIsSelector("require:")},
	{exactRef("package.json", "packageManager"), fromJSON("packageManager"), managerIs},
	{exactRef("package.json", "engines.node"), fromJSON("engines", "node"), identityIs("node")},
	{inModule("package.json", func(s string) bool { return s == "version" }), fromJSON("version"), packageNameIs},
	{prefixedRef("pnpm-workspace.yaml", "catalog:"), catalogVersion, identityIsSelector("catalog:")},
	{exactRef(semconvFile, "semconv"), fromRegexp(`go\.opentelemetry\.io/otel/semconv/(v[^"\s]+)`), identityIs("go.opentelemetry.io/otel/semconv")},
}

func (v *validator) checkRegistries() error {
	for _, l := range v.doc.located() {
		e := l.entry
		if e.RegistryRef == nil {
			if slices.Contains(registrySubjects, e.Subject) {
				v.add(rule.CodeB007, l.path, e.Identity,
					fmt.Sprintf("subject %s exige registry_ref: o BOM referencia o registro autoritativo (BOM-06)", e.Subject))
			}
			continue
		}
		ref := *e.RegistryRef
		resolved, problem, err := resolve(v.in.Root, ref, e.Identity)
		if err != nil {
			return err
		}
		if problem != "" {
			v.add(rule.CodeB007, l.path, e.Identity, problem)
			continue
		}
		if detail := compareVersion(resolved, e.Version); detail != "" {
			v.add(rule.CodeB007, l.path, e.Identity,
				fmt.Sprintf("%s: %s resolve %q em %s", detail, ref.Selector, resolved, ref.File))
		}
	}
	return nil
}

func resolve(fsys fs.FS, ref RegistryRef, identity string) (string, string, error) {
	i := slices.IndexFunc(selectorRules, func(r selectorRule) bool { return r.accepts(ref) })
	if i < 0 || !fs.ValidPath(ref.File) {
		return "", fmt.Sprintf("registry_ref {file: %q, selector: %q} fora do conjunto aceito", ref.File, ref.Selector), nil
	}
	if path.Base(ref.File) == "go.mod" {
		membro, err := inGoWork(fsys, path.Dir(ref.File))
		if err != nil {
			return "", "", err
		}
		if !membro {
			return "", fmt.Sprintf("%s não é módulo do go.work", ref.File), nil
		}
	}
	raw, err := fs.ReadFile(fsys, ref.File)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Sprintf("registro autoritativo %s ausente", ref.File), nil
	}
	if err != nil {
		return "", "", fmt.Errorf("ler %s: %w", ref.File, err)
	}
	r := selectorRules[i]
	value, ok := r.extract(string(raw), ref)
	if !ok {
		return "", fmt.Sprintf("selector %q não resolve em %s", ref.Selector, ref.File), nil
	}
	if problem := r.bind(string(raw), ref, identity, value); problem != "" {
		return "", problem, nil
	}
	return value, "", nil
}

func inGoWork(fsys fs.FS, dir string) (bool, error) {
	raw, err := fs.ReadFile(fsys, "go.work")
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ler go.work: %w", err)
	}
	inUse := false
	for _, line := range strings.Split(withoutComments(string(raw)), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0:
		case fields[0] == "use" && len(fields) > 1 && fields[1] == "(":
			inUse = true
		case fields[0] == "use" && len(fields) > 1:
			if path.Clean(fields[1]) == path.Clean(dir) {
				return true, nil
			}
		case fields[0] == ")":
			inUse = false
		case inUse && path.Clean(fields[0]) == path.Clean(dir):
			return true, nil
		}
	}
	return false, nil
}

func exactRef(file, selector string) func(RegistryRef) bool {
	return func(r RegistryRef) bool { return r.File == file && r.Selector == selector }
}

func prefixedRef(file, prefix string) func(RegistryRef) bool {
	return func(r RegistryRef) bool {
		return r.File == file && strings.HasPrefix(r.Selector, prefix) && len(r.Selector) > len(prefix)
	}
}

func inModule(name string, selector func(string) bool) func(RegistryRef) bool {
	return func(r RegistryRef) bool { return path.Base(r.File) == name && selector(r.Selector) }
}

func mismatch(ref RegistryRef, registered, identity string) string {
	return fmt.Sprintf("%s#%s é o registro de %q, não de %q", ref.File, ref.Selector, registered, identity)
}

func identityIs(want string) func(string, RegistryRef, string, string) string {
	return func(_ string, ref RegistryRef, identity, _ string) string {
		if identity == want {
			return ""
		}
		return mismatch(ref, want, identity)
	}
}

func baseIs(want string) func(string, RegistryRef, string, string) string {
	return func(_ string, ref RegistryRef, identity, _ string) string {
		if path.Base(identity) == want {
			return ""
		}
		return mismatch(ref, want, identity)
	}
}

func baseIsSelector(prefix string) func(string, RegistryRef, string, string) string {
	return func(_ string, ref RegistryRef, identity, _ string) string {
		if want := strings.TrimPrefix(ref.Selector, prefix); path.Base(identity) != want {
			return mismatch(ref, want, identity)
		}
		return ""
	}
}

func identityIsSelector(prefix string) func(string, RegistryRef, string, string) string {
	return func(_ string, ref RegistryRef, identity, _ string) string {
		if want := strings.TrimPrefix(ref.Selector, prefix); identity != want {
			return mismatch(ref, want, identity)
		}
		return ""
	}
}

func managerIs(_ string, ref RegistryRef, identity, resolved string) string {
	if strings.HasPrefix(resolved, identity+"@") {
		return ""
	}
	manager, _, _ := strings.Cut(resolved, "@")
	return mismatch(ref, manager, identity)
}

func packageNameIs(raw string, ref RegistryRef, identity, _ string) string {
	name, _ := fromJSON("name")(raw, ref)
	if name == identity {
		return ""
	}
	return mismatch(ref, name, identity)
}

// Linha de comentário não é registro: um pin antigo comentado acima do atual
// seria lido no lugar dele.
func withoutComments(raw string) string {
	lines := strings.Split(raw, "\n")
	out := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func fromRegexp(pattern string) func(string, RegistryRef) (string, bool) {
	re := regexp.MustCompile(pattern)
	return func(raw string, _ RegistryRef) (string, bool) {
		m := re.FindStringSubmatch(withoutComments(raw))
		if m == nil {
			return "", false
		}
		return m[1], true
	}
}

func pluginVersion(raw string, ref RegistryRef) (string, bool) {
	name := strings.TrimPrefix(ref.Selector, "plugin:")
	return fromRegexp(`(?m)(?:^|[\s/"'])`+regexp.QuoteMeta(name)+`@([^\s"']+)`)(raw, ref)
}

// A diretiva corrente decide: o mesmo pacote aparece em `replace`, `exclude` e
// `retract` com outra versão, e só a de `require` é a resolvida.
func requireVersion(raw string, ref RegistryRef) (string, bool) {
	pkg := strings.TrimPrefix(ref.Selector, "require:")
	directive := ""
	for _, line := range strings.Split(raw, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0:
			continue
		case fields[0] == ")":
			directive = ""
			continue
		case fields[len(fields)-1] == "(":
			directive = fields[0]
			continue
		}
		if directive == "" {
			if fields[0] != "require" {
				continue
			}
			fields = fields[1:]
		} else if directive != "require" {
			continue
		}
		if len(fields) >= 2 && fields[0] == pkg {
			return fields[1], true
		}
	}
	return "", false
}

func fromJSON(keys ...string) func(string, RegistryRef) (string, bool) {
	return func(raw string, _ RegistryRef) (string, bool) {
		var node any
		if err := json.Unmarshal([]byte(raw), &node); err != nil {
			return "", false
		}
		for _, k := range keys {
			obj, ok := node.(map[string]any)
			if !ok {
				return "", false
			}
			node = obj[k]
		}
		s, ok := node.(string)
		return s, ok && s != ""
	}
}

// Leitura por linha, sem biblioteca de YAML: o mapa `catalog` do
// pnpm-workspace.yaml é plano, e a spec veda dependência Go nova.
func catalogVersion(raw string, ref RegistryRef) (string, bool) {
	pkg := strings.TrimPrefix(ref.Selector, "catalog:")
	inCatalog := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			inCatalog = trimmed == "catalog:"
			continue
		}
		if !inCatalog {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if ok && unquote(strings.TrimSpace(key)) == pkg {
			v := unquote(strings.TrimSpace(withoutYAMLComment(value)))
			return v, v != ""
		}
	}
	return "", false
}

func withoutYAMLComment(s string) string {
	var quote rune
	for i, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			return s[:i]
		}
	}
	return s
}

func unquote(s string) string {
	if u, err := strconv.Unquote(s); err == nil {
		return u
	}
	return strings.Trim(s, "'")
}

// Sem normalizar, `v1.72.0` × `1.72.0`, `pnpm@11.14.0` × `11.14.0` e o hash do
// corepack (`+sha512…`) acusariam divergência que não existe.
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.LastIndex(v, "@"); i >= 0 {
		v = v[i+1:]
	}
	v, _, _ = strings.Cut(v, "+")
	return strings.TrimPrefix(v, "v")
}

func compareVersion(resolved, declared string) string {
	r, d := normalizeVersion(resolved), normalizeVersion(declared)
	if base, ok := strings.CutPrefix(r, "^"); ok {
		if caretSatisfies(base, d) {
			return ""
		}
		return fmt.Sprintf("version %q fora da faixa do registro", declared)
	}
	if isRange(r) {
		return "faixa não suportada pelo resolvedor (só ^ ou versão exata)"
	}
	if r != d {
		return fmt.Sprintf("version %q diverge do registro", declared)
	}
	return ""
}

func caretSatisfies(base, version string) bool {
	b, okBase := numericParts(base)
	got, okGot := numericParts(version)
	if !okBase || !okGot || len(got) != 3 {
		return false
	}
	for len(b) < 3 {
		b = append(b, 0)
	}
	if got[0] != b[0] || (b[0] == 0 && got[1] != b[1]) {
		return false
	}
	return slices.Compare(got, b) >= 0
}

func numericParts(v string) ([]int, bool) {
	core, _, _ := strings.Cut(v, "-")
	fields := strings.Split(core, ".")
	if len(fields) > 3 {
		return nil, false
	}
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
