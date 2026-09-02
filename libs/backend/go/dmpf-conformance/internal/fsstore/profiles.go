// Package fsstore lê o disco e decodifica JSON — `wire.codec`, vedado em
// `domain` e `port`. Por isso o domínio recebe modelo, nunca bytes.
package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Define o que conta como "produção" ao decidir quais arquivos entram. Sem o
// conjunto declarado, trocar o runner mudaria o veredicto em silêncio.
type BuildProfile struct {
	ID         string   `json:"id"`
	GOOS       string   `json:"goos"`
	GOARCH     string   `json:"goarch"`
	CGOEnabled bool     `json:"cgo_enabled"`
	Tags       []string `json:"tags"`
}

const buildProfilesSchema = "dmpf/build-profiles@1"

type buildProfilesDoc struct {
	Schema   string         `json:"schema"`
	Profiles []BuildProfile `json:"profiles"`
}

// Conjunto vazio ou arquivo ausente é erro, não default: sem perfil declarado
// o verificador não sabe o que excluir por build tag, e não saber reprova.
func LoadBuildProfiles(path string) ([]BuildProfile, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("ler perfis de produção em %s: %w", path, err)
	}
	var doc buildProfilesDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decodificar %s: %w", path, err)
	}
	if doc.Schema != buildProfilesSchema {
		return nil, fmt.Errorf("%s: schema %q, esperado %q", path, doc.Schema, buildProfilesSchema)
	}
	if len(doc.Profiles) == 0 {
		return nil, fmt.Errorf("%s: nenhum perfil de produção declarado", path)
	}
	for i, p := range doc.Profiles {
		if p.ID == "" || p.GOOS == "" || p.GOARCH == "" {
			return nil, fmt.Errorf("%s: profiles[%d] sem id, goos ou goarch", path, i)
		}
	}
	return doc.Profiles, nil
}
