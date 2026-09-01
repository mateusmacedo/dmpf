package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/manifest"
)

// ManifestFileName é o nome fixo do metadata_container por ownership_module.
const ManifestFileName = "dmpf-units.json"

// wireUnit espelha o JSON do schema dmpf/units@1. Os ponteiros existem para
// distinguir "campo ausente" de "presente com valor zero" — a distinção que
// DMPF-M001 exige e que a ausência de herança (RFC §10.1) torna obrigatória:
// `bounded_context: ""` declarado é erro de valor, não campo omitido.
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

type wireException struct {
	Unit       string `json:"unit"`
	Dependency string `json:"dependency"`
	Reason     string `json:"reason"`
	Owner      string `json:"owner"`
	ReviewBy   string `json:"review_by"`
}

type wireDocument struct {
	Schema     string          `json:"schema"`
	Units      []wireUnit      `json:"units"`
	External   []wireExternal  `json:"external"`
	Exceptions []wireException `json:"exceptions"`
}

// DecodeManifest converte os bytes do dmpf-units.json no modelo puro que o
// domínio valida. Esta função NÃO julga conformidade: erro de sintaxe JSON
// impede a decodificação e é devolvido como erro; tudo o mais vira campo do
// modelo e é decidido por manifest.Validate.
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
		doc.Exceptions = append(doc.Exceptions, manifest.Exception{
			Unit: x.Unit, Dependency: x.Dependency, Reason: x.Reason,
			Owner: x.Owner, ReviewBy: x.ReviewBy,
		})
	}
	return doc, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ManifestStore lê os manifestos dos módulos do inventário.
type ManifestStore struct {
	modules []ModuleDir
}

// ModuleDir associa o import path do módulo ao diretório em que ele vive.
type ModuleDir struct {
	Path string
	Dir  string
}

func NewManifestStore(modules []ModuleDir) *ManifestStore {
	return &ManifestStore{modules: modules}
}

// Documents devolve um Document por módulo que tenha manifesto. Módulo sem
// manifesto NÃO é erro aqui: a ausência é DMPF-U004, decidida no domínio a
// partir do inventário — o provider apenas relata o que existe.
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
