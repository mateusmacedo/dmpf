package bom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

// Só o header do envelope de SPEC-JPP31095: é ele que nomeia as versões
// resolvidas da execução, e é contra elas que compatible_with é conferido.
type evidence struct {
	Header struct {
		GoVersion string `json:"goversion"`
		Modules   []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
		} `json:"modules"`
		Externals []struct {
			Package string `json:"package"`
			Version string `json:"version"`
		} `json:"externals"`
	} `json:"header"`
}

func (e evidence) exercised(identity, version string) bool {
	want := normalizeVersion(version)
	if identity == "go" && strings.TrimPrefix(e.Header.GoVersion, "go") == want {
		return true
	}
	for _, m := range e.Header.Modules {
		if m.Path == identity && normalizeVersion(m.Version) == want {
			return true
		}
	}
	for _, x := range e.Header.Externals {
		if x.Package == identity && normalizeVersion(x.Version) == want {
			return true
		}
	}
	return false
}

var evidenceNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func (v *validator) checkCompatibility() error {
	anchored := v.anchoredEvidence()
	for _, l := range v.doc.located() {
		for i, c := range l.entry.CompatibleWith {
			where := fmt.Sprintf("%s.compatible_with[%d]", l.path, i)
			target := c.Identity + "@" + c.Version
			if !evidenceNameRe.MatchString(c.Evidence) {
				v.add(rule.CodeB006, where, target, fmt.Sprintf(
					"evidence %q não nomeia um subject de bom/evidence/%s/: combinação presumida não é exercitada (BOM-04)",
					c.Evidence, v.doc.Release))
				continue
			}
			if !anchored[c.Evidence] {
				v.add(rule.CodeB006, where, target,
					fmt.Sprintf("evidence %q não é ancorada por evidence_digest de nenhuma entrada deste BOM", c.Evidence))
				continue
			}
			ev, problem, err := v.loadEvidence(c.Evidence)
			if err != nil {
				return err
			}
			if problem != "" {
				v.add(rule.CodeB006, where, target, problem)
				continue
			}
			// BOM-04 fala da combinação: os dois lados precisam constar da mesma execução.
			for _, lado := range []struct{ identity, version string }{
				{l.entry.Identity, l.entry.Version}, {c.Identity, c.Version},
			} {
				if !ev.exercised(lado.identity, lado.version) {
					v.add(rule.CodeB006, where, target, fmt.Sprintf("%s@%s sem execução registrada em %s",
						lado.identity, lado.version, v.evidenceFile(c.Evidence)))
				}
			}
		}
	}
	return nil
}

// Sem digest que a prenda, a evidência de uma combinação poderia ser trocada
// depois sem tocar no BOM: só conta subject que alguma entrada ancora (BOM-03).
func (v *validator) anchoredEvidence() map[string]bool {
	out := map[string]bool{}
	for _, l := range v.doc.located() {
		if l.entry.EvidenceDigest == "" {
			continue
		}
		if file, ok := evidencePath(l.entry.EvidenceURI, v.doc.Release); ok {
			out[strings.TrimSuffix(path.Base(file), ".json")] = true
		}
	}
	return out
}

func (v *validator) evidenceFile(subject string) string {
	return path.Join("bom", "evidence", v.doc.Release, subject+".json")
}

func (v *validator) loadEvidence(subject string) (evidence, string, error) {
	file := v.evidenceFile(subject)
	raw, err := fs.ReadFile(v.in.Root, file)
	if errors.Is(err, fs.ErrNotExist) {
		return evidence{}, fmt.Sprintf("evidência %s ausente", file), nil
	}
	if err != nil {
		return evidence{}, "", fmt.Errorf("ler %s: %w", file, err)
	}
	var ev evidence
	if err := json.Unmarshal(raw, &ev); err != nil {
		return evidence{}, fmt.Sprintf("evidência %s ilegível: %v", file, err), nil
	}
	return ev, "", nil
}
