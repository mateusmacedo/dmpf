package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/baseline"
)

type BaselineStore struct {
	root string
}

func NewBaselineStore(root string) *BaselineStore { return &BaselineStore{root: root} }

// documentoWire isola a chave opcional em RawMessage: só assim dá para
// distinguir chave ausente (nil) de `null` (bytes "null") de lista, já que
// `[]string` sozinho colapsa ausente e `null` no mesmo `nil`.
type documentoWire struct {
	Schema            string           `json:"schema"`
	Digest            string           `json:"digest"`
	Entries           []baseline.Entry `json:"entries"`
	SharedKernelUnits json.RawMessage  `json:"shared_kernel_units"`
}

// `null` é inválido — fail-closed, em vez de virar "ausente" em silêncio.
func decodificarBaseline(raw []byte) (baseline.Document, error) {
	var wire documentoWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return baseline.Document{}, err
	}
	doc := baseline.Document{Schema: wire.Schema, Digest: wire.Digest, Entries: wire.Entries}

	if wire.SharedKernelUnits == nil {
		return doc, nil
	}
	if string(wire.SharedKernelUnits) == "null" {
		return baseline.Document{}, fmt.Errorf("shared_kernel_units: null não é permitido (omita a chave ou use [])")
	}
	var units []string
	if err := json.Unmarshal(wire.SharedKernelUnits, &units); err != nil {
		return baseline.Document{}, fmt.Errorf("decodificar shared_kernel_units: %w", err)
	}
	if units == nil {
		units = []string{}
	}
	doc.SharedKernelUnits = units
	doc.HasSharedKernelUnits = true
	return doc, nil
}

// Ausência devolve `false` sem erro: repositório que ainda não adotou o
// baseline não está quebrado, e o que fazer com isso é decisão do domínio.
func (s *BaselineStore) Baseline() (baseline.Document, bool, error) {
	p := filepath.Join(s.root, baseline.Path)
	raw, err := os.ReadFile(p) //nolint:gosec // caminho fixo, relativo à raiz do repo
	if err != nil {
		if os.IsNotExist(err) {
			return baseline.Document{}, false, nil
		}
		return baseline.Document{}, false, fmt.Errorf("ler %s: %w", baseline.Path, err)
	}
	doc, err := decodificarBaseline(raw)
	if err != nil {
		return baseline.Document{}, false, fmt.Errorf("decodificar %s: %w", baseline.Path, err)
	}
	return doc, true, nil
}

// Só o comando de regeneração escreve: um gate que conserta o próprio insumo
// não é gate.
func (s *BaselineStore) Escrever(doc baseline.Document) error {
	p := filepath.Join(s.root, baseline.Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if doc.SharedKernelUnits == nil {
		doc.SharedKernelUnits = []string{}
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(raw, '\n'), 0o644) //nolint:gosec // artefato versionado, legível por todos
}
