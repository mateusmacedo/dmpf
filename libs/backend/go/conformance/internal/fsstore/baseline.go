package fsstore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/baseline"
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

// Único ponto de Unmarshal de baseline.Document: Baseline() e BaselineEm() o
// compartilham para que a leitura do ref base nunca divirja da do arquivo atual.
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

// BaselineEm lê o baseline como ele estava no ref informado.
//
// É o que permite comparar a classificação de antes com a de agora. Sem isso só
// dá para confrontar baseline e manifesto no mesmo ponto — e quem altera os dois
// de forma coerente passa despercebido, que é justamente o caso que a exigência
// de aval existe para pegar.
//
// Ausência devolve `false` sem erro: o ref pode ser anterior à adoção do
// baseline, e aí tudo o que existe agora é criação de unidade.
func (s *BaselineStore) BaselineEm(ref string) (baseline.Document, bool, error) {
	if ref == "" {
		return baseline.Document{}, false, nil
	}
	saida, err := s.git("show", ref+":"+baseline.Path)
	if err != nil {
		// `git show` falha tanto quando o arquivo não existe no ref quanto
		// quando o ref é inalcançável. Sem separar os dois, um clone raso
		// viraria "baseline ausente" e a mudança passaria por criação.
		if _, errRef := s.git("rev-parse", "--verify", ref+"^{commit}"); errRef != nil {
			return baseline.Document{}, false, fmt.Errorf("ref %s inalcançável: %w", ref, errRef)
		}
		return baseline.Document{}, false, nil
	}
	doc, err := decodificarBaseline([]byte(saida))
	if err != nil {
		return baseline.Document{}, false, fmt.Errorf("decodificar o baseline em %s: %w", ref, err)
	}
	return doc, true, nil
}

func (s *BaselineStore) CommitsQueTocaram(base string) ([]baseline.Commit, error) {
	if base == "" {
		return nil, nil
	}
	saida, err := s.git("log", "--format=%H%x1f%an%x1f%s", "--name-only", base+"..HEAD")
	if err != nil {
		// Devolver vazio faria o domínio ler "nenhuma mistura"; o erro faz ele
		// reportar "não verificado".
		return nil, fmt.Errorf("ler histórico de %s..HEAD: %w", base, err)
	}

	var out []baseline.Commit
	var atual *baseline.Commit
	for _, linha := range strings.Split(saida, "\n") {
		if linha == "" {
			continue
		}
		if campos := strings.Split(linha, "\x1f"); len(campos) == 3 {
			if atual != nil {
				out = append(out, *atual)
			}
			atual = &baseline.Commit{SHA: campos[0], Autor: campos[1], Assunto: campos[2]}
			continue
		}
		if atual != nil {
			atual.Arquivos = append(atual.Arquivos, linha)
		}
	}
	if atual != nil {
		out = append(out, *atual)
	}
	return out, nil
}

func (s *BaselineStore) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.root
	out, err := cmd.Output()
	return string(out), err
}
