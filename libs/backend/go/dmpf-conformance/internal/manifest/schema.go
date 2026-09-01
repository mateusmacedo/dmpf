// Package manifest é bloco `domain`: valida o schema dmpf/units@1 sobre um
// modelo JÁ DECODIFICADO. A decodificação é wire.codec, vedada em `domain`
// (RFC §6.2), e vive no provider fsstore.
package manifest

// SchemaID é o único valor aceito no campo `schema` (RFC §10.1).
const SchemaID = "dmpf/units@1"

// Unit é uma verification_unit declarada. Os campos são os de RFC §10.1, já
// decodificados. Present* distingue "ausente" de "presente com valor zero" —
// a distinção que M001 exige e que a ausência de herança torna obrigatória
// (nenhum campo é derivado de diretório pai, de módulo ou de outra unidade).
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

// Document é um dmpf-units.json decodificado, com a origem preservada para a
// mensagem do diagnóstico.
type Document struct {
	Path       string
	Module     string
	Schema     string
	Units      []Unit
	External   []External
	Exceptions []Exception
}

// External é uma entrada da allowlist de capabilities (RFC §6.3).
type External struct {
	Package     string
	Entrypoints []string
	Capability  string
	Versions    string
}

// Exception é uma exceção NOMINAL à política de um bloco (RFC §6.4): o par
// (unidade, dependência), com justificativa, owner e data de revisão. Exceção
// por categoria, prefixo ou diretório é proibida pela própria forma do registro.
type Exception struct {
	Unit          string
	Package       string
	Justification string
	Owner         string
	ReviewBy      string
}
