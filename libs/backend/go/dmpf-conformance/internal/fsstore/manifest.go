package fsstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if chave, repetida := DuplicateKey(raw); repetida {
		return manifest.Document{}, fmt.Errorf("decodificar %s: chave %q repetida no mesmo objeto", path, chave)
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
		var invalid []string
		instante := func(campo, valor string) exception.Instant {
			at, ok := parseInstant(valor)
			if valor != "" && !ok {
				invalid = append(invalid, campo)
			}
			return at
		}
		ate := func(campo, valor string) exception.Instant {
			at, ok := parseUntil(valor)
			if valor != "" && !ok {
				invalid = append(invalid, campo)
			}
			return at
		}
		exc := manifest.Exception{
			Unit: x.Unit, Dependency: x.Dependency, Reason: x.Reason,
			Owner: x.Owner, ReviewBy: x.ReviewBy,
			ReviewByAt: instante("review_by", x.ReviewBy),
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
				Deadline:            instante("convergence.deadline", deref(x.Convergence.Deadline)),
				Condition:           deref(x.Convergence.Condition),
				ReviewBy:            instante("convergence.review_by", deref(x.Convergence.ReviewBy)),
				ApprovedBy:          x.Convergence.ApprovedBy,
				ReplanningCondition: deref(x.Convergence.ReplanningCondition),
				PresentKind:         x.Convergence.Kind != nil,
			}
			exc.PresentConvergence = true
		}
		if x.ValidFrom != nil {
			exc.ValidFrom = instante("valid_from", *x.ValidFrom)
			exc.PresentValidFrom = true
		}
		if x.ValidUntil != nil {
			exc.ValidUntil = ate("valid_until", *x.ValidUntil)
			exc.PresentValidUntil = true
		}
		if x.History != nil {
			for i, h := range x.History {
				exc.History = append(exc.History, manifest.ExceptionHistoryEntry{
					Event: h.Event, At: instante(fmt.Sprintf("history[%d].at", i), h.At), By: h.By, Reason: h.Reason,
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
		exc.InvalidDates = invalid
		doc.Exceptions = append(doc.Exceptions, exc)
	}
	return doc, nil
}

// RFC3339 primeiro, data simples depois: as duas formas aparecem nos manifestos
// e o bloco domain recebe o instante pronto.
func parseInstant(s string) (exception.Instant, bool) {
	if s == "" {
		return 0, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return exception.Instant(t.UnixNano()), true
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return exception.Instant(t.UnixNano()), true
	}
	return 0, false
}

// Data sem hora vale o dia inteiro: `valid_until: 2027-03-02` ainda vale ao
// meio-dia do dia 2.
func parseUntil(s string) (exception.Instant, bool) {
	at, ok := parseInstant(s)
	if ok && len(s) == len(time.DateOnly) {
		at += exception.Instant(24*time.Hour - time.Nanosecond)
	}
	return at, ok
}

// DuplicateKey acha chave repetida num mesmo objeto, sem distinguir
// maiúsculas: o encoding/json casa assim e fica com a última, enquanto quem
// revisa o PR lê a primeira.
func DuplicateKey(raw []byte) (string, bool) {
	type frame struct {
		keys      map[string]bool
		expectKey bool
	}
	var stack []*frame
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].keys != nil {
			stack[n-1].expectKey = true
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", false
		}
		if n := len(stack); n > 0 && stack[n-1].keys != nil && stack[n-1].expectKey {
			if d, ok := tok.(json.Delim); ok && d == '}' {
				stack = stack[:n-1]
				valueDone()
				continue
			}
			key, ok := tok.(string)
			if !ok {
				return "", false
			}
			if stack[n-1].keys[strings.ToLower(key)] {
				return key, true
			}
			stack[n-1].keys[strings.ToLower(key)] = true
			stack[n-1].expectKey = false
			continue
		}
		switch d := tok.(type) {
		case json.Delim:
			switch d {
			case '{':
				stack = append(stack, &frame{keys: map[string]bool{}, expectKey: true})
			case '[':
				stack = append(stack, &frame{})
			default:
				stack = stack[:len(stack)-1]
				valueDone()
			}
		default:
			valueDone()
		}
	}
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
