// Package manifest valida o schema dmpf/units@1 sobre modelo JÁ DECODIFICADO:
// decodificar é acesso a formato de wire, vedado neste bloco, e vive no fsstore.
package manifest

const SchemaID = "dmpf/units@1"

// Present* distingue "campo ausente" de "presente com valor zero": sem herança,
// não há de onde derivar o que falta, e `bounded_context: ""` declarado é erro
// de valor, não omissão.
type Unit struct {
	ID                       string
	Block                    string
	BoundedContext           string
	Include                  []string
	PublicIntegrationSurface bool

	PresentID                       bool
	PresentBlock                    bool
	PresentBoundedContext           bool
	PresentInclude                  bool
	PresentPublicIntegrationSurface bool
}

type Document struct {
	Path       string
	Module     string
	Schema     string
	Units      []Unit
	External   []External
	Exceptions []Exception
}

type External struct {
	Package     string
	Entrypoints []string
	Capability  string
	Versions    string
}

// Os campos são `dependency` e `reason`, como no exemplo canônico do schema —
// não `package` e `justification`.
type Exception struct {
	Unit       string
	Dependency string
	Reason     string
	Owner      string
	ReviewBy   string
}
