package infrasync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	layoutFile   = "dmpf.json"
	layoutSchema = "dmpf/workspace@1"
)

// Layout é o desenho do workspace que os arquivos gerados citam. Sem dmpf.json
// valem os valores do platform, e a saída não muda nenhum byte.
type Layout struct {
	AppsDir           string
	Edge              string
	SpiffeTrustDomain string
	ComposeProfile    string
}

var platformLayout = Layout{AppsDir: "apps/backend", Edge: "bff", SpiffeTrustDomain: "dmpf", ComposeProfile: "dmpf"}

var (
	trustDomainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)
	profilePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)
)

type layoutDoc struct {
	Schema            string  `json:"schema"`
	AppsDir           *string `json:"appsDir"`
	Edge              *string `json:"edge"`
	SpiffeTrustDomain *string `json:"spiffeTrustDomain"`
	ComposeProfile    *string `json:"composeProfile"`
}

// O dmpf.json é compartilhado com o plugin Nx: campos que só ele lê são
// aceitos e ignorados. Edge vazia declara um workspace sem borda pública.
func LoadLayout(root string) (Layout, error) {
	raw, err := os.ReadFile(filepath.Join(root, layoutFile))
	if errors.Is(err, os.ErrNotExist) {
		return platformLayout, nil
	}
	if err != nil {
		return Layout{}, fmt.Errorf("%s: %w", layoutFile, err)
	}
	var doc layoutDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Layout{}, fmt.Errorf("%s: %w", layoutFile, err)
	}
	if doc.Schema != layoutSchema {
		return Layout{}, fmt.Errorf("%s: schema %q, esperado %q", layoutFile, doc.Schema, layoutSchema)
	}
	l := platformLayout
	for _, c := range []struct {
		dst *string
		src *string
	}{{&l.AppsDir, doc.AppsDir}, {&l.Edge, doc.Edge}, {&l.SpiffeTrustDomain, doc.SpiffeTrustDomain}, {&l.ComposeProfile, doc.ComposeProfile}} {
		if c.src != nil {
			*c.dst = *c.src
		}
	}
	if problemas := l.problems(); len(problemas) > 0 {
		return Layout{}, fmt.Errorf("%s: %s", layoutFile, strings.Join(problemas, "; "))
	}
	return l, nil
}

func (l Layout) problems() []string {
	var out []string
	if l.AppsDir == "" || path.IsAbs(l.AppsDir) || path.Clean(l.AppsDir) != l.AppsDir || l.AppsDir == "." ||
		strings.HasPrefix(l.AppsDir, "../") || l.AppsDir == ".." {
		out = append(out, fmt.Sprintf("appsDir %q precisa ser caminho relativo limpo dentro do workspace", l.AppsDir))
	}
	for _, segmento := range strings.Split(l.AppsDir, "/") {
		if !namePattern.MatchString(segmento) {
			out = append(out, fmt.Sprintf("appsDir %q tem segmento %q fora de [A-Za-z0-9._-]", l.AppsDir, segmento))
			break
		}
	}
	if l.Edge != "" && !dnsLabelPattern.MatchString(l.Edge) {
		out = append(out, fmt.Sprintf("edge %q não é rótulo DNS minúsculo", l.Edge))
	}
	if !trustDomainPattern.MatchString(l.SpiffeTrustDomain) {
		out = append(out, fmt.Sprintf("spiffeTrustDomain %q fora de [a-z0-9.-]", l.SpiffeTrustDomain))
	}
	if !profilePattern.MatchString(l.ComposeProfile) {
		out = append(out, fmt.Sprintf("composeProfile %q fora de [a-z0-9_.-]", l.ComposeProfile))
	}
	return out
}

func (l Layout) manifestGlob() string { return l.AppsDir + "/*/deploy/infra.json" }

func (l Layout) devOverlay(app string) string {
	return l.AppsDir + "/" + app + "/deploy/k8s/overlays/dev/kustomization.yaml"
}
