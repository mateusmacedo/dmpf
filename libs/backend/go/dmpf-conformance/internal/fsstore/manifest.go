package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/manifest"
)

const ManifestFileName = "dmpf-units.json"

// Os ponteiros distinguem "campo ausente" de "presente com valor zero", que é
// o que DMPF-M001 exige.
type wireUnit struct {
	ID                       *string  `json:"id"`
	Block                    *string  `json:"block"`
	BoundedContext           *string  `json:"bounded_context"`
	Include                  []string `json:"include"`
	PublicIntegrationSurface *bool    `json:"public_integration_surface"`
}

type wireExternal struct {
	Package     string   `json:"package"`
	Entrypoints []string `json:"entrypoints"`
	Capability  string   `json:"capability"`
	Versions    string   `json:"versions"`
}

type wireExceptionObject struct {
	Kind     *string `json:"kind"`
	Unit     *string `json:"unit"`
	Identity *string `json:"identity"`
}

type wireExceptionConvergence struct {
	Kind                *string  `json:"kind"`
	Deadline            *string  `json:"deadline"`
	Condition           *string  `json:"condition"`
	ReviewBy            *string  `json:"review_by"`
	ApprovedBy          []string `json:"approved_by"`
	ReplanningCondition *string  `json:"replanning_condition"`
}

type wireExceptionHistoryEntry struct {
	Event  string `json:"event"`
	At     string `json:"at"`
	By     string `json:"by"`
	Reason string `json:"reason"`
}

type wireException struct {
	Unit       string `json:"unit"`
	Dependency string `json:"dependency"`
	Reason     string `json:"reason"`
	Owner      string `json:"owner"`
	ReviewBy   string `json:"review_by"`

	ID            *string                     `json:"id"`
	Object        *wireExceptionObject        `json:"object"`
	ADR           *string                     `json:"adr"`
	Justification *string                     `json:"justification"`
	Convergence   *wireExceptionConvergence   `json:"convergence"`
	ValidFrom     *string                     `json:"valid_from"`
	ValidUntil    *string                     `json:"valid_until"`
	History       []wireExceptionHistoryEntry `json:"history"`
}

type wireDocument struct {
	Schema     string          `json:"schema"`
	Units      []wireUnit      `json:"units"`
	External   []wireExternal  `json:"external"`
	Exceptions []wireException `json:"exceptions"`
}

// Não julga conformidade: só erro de sintaxe JSON vira erro aqui; o resto vira
// campo do modelo e é decidido por manifest.Validate.
func DecodeManifest(path, module string, raw []byte) (manifest.Document, error) {
	var w wireDocument
	if err := json.Unmarshal(raw, &w); err != nil {
		return manifest.Document{}, fmt.Errorf("decodificar %s: %w", path, err)
	}

	doc := manifest.Document{Path: path, Module: module, Schema: w.Schema}
	for _, u := range w.Units {
		doc.Units = append(doc.Units, manifest.Unit{
			ID:                       deref(u.ID),
			Block:                    deref(u.Block),
			BoundedContext:           deref(u.BoundedContext),
			Include:                  u.Include,
			PublicIntegrationSurface: u.PublicIntegrationSurface != nil && *u.PublicIntegrationSurface,

			PresentID:             u.ID != nil,
			PresentBlock:          u.Block != nil,
			PresentBoundedContext: u.BoundedContext != nil,
			PresentInclude:        u.Include != nil,
		})
	}
	for _, e := range w.External {
		doc.External = append(doc.External, manifest.External{
			Package: e.Package, Entrypoints: e.Entrypoints,
			Capability: e.Capability, Versions: e.Versions,
		})
	}
	for _, x := range w.Exceptions {
		exc := manifest.Exception{
			Unit: x.Unit, Dependency: x.Dependency, Reason: x.Reason,
			Owner: x.Owner, ReviewBy: x.ReviewBy,
			ReviewByAt: parseInstant(x.ReviewBy),
		}
		if x.ID != nil {
			exc.ID = *x.ID
			exc.PresentID = true
		}
		if x.Object != nil {
			exc.Object = manifest.ExceptionObject{
				Kind:            deref(x.Object.Kind),
				Unit:            deref(x.Object.Unit),
				Identity:        deref(x.Object.Identity),
				PresentKind:     x.Object.Kind != nil,
				PresentUnit:     x.Object.Unit != nil,
				PresentIdentity: x.Object.Identity != nil,
			}
			exc.PresentObject = true
		}
		if x.ADR != nil {
			exc.ADR = *x.ADR
			exc.PresentADR = true
		}
		if x.Justification != nil {
			exc.Justification = *x.Justification
			exc.PresentJustification = true
		}
		if x.Convergence != nil {
			exc.Convergence = manifest.ExceptionConvergence{
				Kind:                deref(x.Convergence.Kind),
				Deadline:            parseInstant(deref(x.Convergence.Deadline)),
				Condition:           deref(x.Convergence.Condition),
				ReviewBy:            parseInstant(deref(x.Convergence.ReviewBy)),
				ApprovedBy:          x.Convergence.ApprovedBy,
				ReplanningCondition: deref(x.Convergence.ReplanningCondition),
				PresentKind:         x.Convergence.Kind != nil,
			}
			exc.PresentConvergence = true
		}
		if x.ValidFrom != nil {
			exc.ValidFrom = parseInstant(*x.ValidFrom)
			exc.PresentValidFrom = true
		}
		if x.ValidUntil != nil {
			exc.ValidUntil = parseInstant(*x.ValidUntil)
			exc.PresentValidUntil = true
		}
		if x.History != nil {
			for _, h := range x.History {
				exc.History = append(exc.History, manifest.ExceptionHistoryEntry{
					Event: h.Event, At: parseInstant(h.At), By: h.By, Reason: h.Reason,
				})
			}
			exc.PresentHistory = true
		}
		if !exc.PresentObject {
			exc.Object = manifest.ExceptionObject{
				Unit:            x.Unit,
				Identity:        x.Dependency,
				PresentUnit:     x.Unit != "",
				PresentIdentity: x.Dependency != "",
			}
		}
		if !exc.PresentJustification && x.Reason != "" {
			exc.Justification = x.Reason
			exc.PresentJustification = true
		}
		doc.Exceptions = append(doc.Exceptions, exc)
	}
	return doc, nil
}

// RFC3339 primeiro, data simples depois: as duas formas aparecem nos manifestos
// e o bloco domain recebe o instante pronto.
func parseInstant(s string) exception.Instant {
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return exception.Instant(t.UnixNano())
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return exception.Instant(t.UnixNano())
	}
	return 0
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

type ManifestStore struct {
	modules []ModuleDir
}

type ModuleDir struct {
	Path string
	Dir  string
}

func NewManifestStore(modules []ModuleDir) *ManifestStore {
	return &ManifestStore{modules: modules}
}

// Módulo sem manifesto não é erro aqui: a ausência é DMPF-U004, decidida no
// domínio — o provider apenas relata o que existe.
func (s *ManifestStore) Documents() ([]manifest.Document, error) {
	var out []manifest.Document
	for _, m := range s.modules {
		p := filepath.Join(m.Dir, ManifestFileName)
		raw, err := os.ReadFile(p) //nolint:gosec // caminho derivado do inventário do próprio repo
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("ler %s: %w", p, err)
		}
		doc, err := DecodeManifest(p, m.Path, raw)
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, nil
}
