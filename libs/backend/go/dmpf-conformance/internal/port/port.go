// Package port é bloco `port`: as capacidades de fronteira que o verificador
// consome. Nenhuma unidade `domain` (rule, manifest, baseline) importa este
// package — a aresta domain → port é proibida sem exceção (ADR-014), e o
// desenho a torna impossível em vez de apenas proibida.
package port

import (
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/manifest"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// GraphSource entrega o grafo de packages resolvido pelo toolchain — nunca o
// texto do import (RFC §3.5). Edges são as arestas diretas; Deps é o fechamento
// transitivo, usado apenas na avaliação de pureza.
type GraphSource interface {
	Packages() ([]rule.Package, error)
	Edges() ([]Edge, error)
}

// Edge é uma aresta package → package. SourceFile nomeia o arquivo que a
// introduz e serve apenas à mensagem do diagnóstico, nunca à decisão.
type Edge struct {
	From       string
	To         string
	SourceFile string
	Unresolved bool
	Detail     string
}

// ManifestSource entrega os manifestos já decodificados. A decodificação é
// wire.codec e pertence ao provider; o domínio recebe modelo puro.
type ManifestSource interface {
	Documents() ([]manifest.Document, error)
}

// InventorySource entrega o inventário independente de módulos, reconciliando
// go.mod rastreados, projetos Nx `stack:go` e membros do go.work.
type InventorySource interface {
	Modules() ([]rule.Module, error)
}
