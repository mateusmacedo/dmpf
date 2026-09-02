// Package port reúne as capacidades de fronteira do verificador.
//
// Nenhuma unidade `domain` importa este package. A aresta é proibida sem
// exceção, e como `port` importa `manifest`, que importa `rule`, tentá-la vira
// ciclo de import: o compilador recusa antes de o gate opinar.
package port

import (
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/baseline"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/manifest"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// GraphSource entrega o grafo como o compilador o resolve, nunca o texto que
// está escrito no import.
type GraphSource interface {
	Packages() ([]rule.Package, error)
	Edges() ([]Edge, error)
}

type Edge struct {
	From string
	To   string
	// SourceFile e Detail alimentam só a mensagem: a decisão é sobre o package
	// que o compilador resolveu, nunca sobre o texto do arquivo.
	SourceFile string
	Unresolved bool
	Detail     string
}

// ManifestSource entrega manifestos já decodificados: ler formato de wire é
// acesso vedado nos blocos que consomem esta porta.
type ManifestSource interface {
	Documents() ([]manifest.Document, error)
}

type InventorySource interface {
	Modules() ([]rule.Module, error)
}

type BaselineStore interface {
	// O bool distingue ausência de erro: repositório sem baseline ainda não o
	// adotou, e o que fazer com isso é decisão do domínio.
	Baseline() (baseline.Document, bool, error)

	// BaselineEm lê o baseline como ele estava no ref: é comparando o de antes
	// com o de agora que se descobre o que mudou de classificação.
	BaselineEm(ref string) (baseline.Document, bool, error)

	CommitsQueTocaram(base string) ([]baseline.Commit, error)
}
