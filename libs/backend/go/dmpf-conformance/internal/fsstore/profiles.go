// Package fsstore é bloco `provider`: lê o disco e decodifica JSON. A
// decodificação é capability `wire.codec`, vedada em `domain` e em `port`
// (RFC §6.2) — por isso o domínio recebe modelo puro e nunca bytes.
package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// BuildProfile é um alvo de produção: a combinação que define o que "build tag
// usada em produção" significa operacionalmente (RFC §10.3). Sem esse conjunto
// declarado, "produção" seria o perfil implícito do runner, e trocar o runner
// mudaria o veredicto em silêncio.
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

// LoadBuildProfiles lê o conjunto de perfis de produção. Conjunto vazio ou
// arquivo ausente é erro, não default silencioso: sem perfil declarado o
// verificador não sabe o que excluir por build tag, e não saber reprova.
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
