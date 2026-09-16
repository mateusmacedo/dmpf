// Package manifest valida o schema dmpf/units@1 sobre modelo JÁ DECODIFICADO:
// decodificar é acesso a formato de wire, vedado neste bloco, e vive no fsstore.
package manifest

import "github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/exception"

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

type ExceptionObject struct {
	Kind     string
	Unit     string
	Identity string

	PresentKind     bool
	PresentUnit     bool
	PresentIdentity bool
}

type ExceptionConvergence struct {
	Kind string

	Deadline  exception.Instant
	Condition string

	ReviewBy            exception.Instant
	ApprovedBy          []string
	ReplanningCondition string

	PresentKind bool
}

type ExceptionHistoryEntry struct {
	Event  string
	At     exception.Instant
	By     string
	Reason string
}

type Exception struct {
	Unit       string
	Dependency string
	Reason     string
	Owner      string
	ReviewBy   string

	ID            string
	Object        ExceptionObject
	ADR           string
	Justification string
	Convergence   ExceptionConvergence
	ValidFrom     exception.Instant
	ValidUntil    exception.Instant
	History       []ExceptionHistoryEntry

	// A data em texto é `review_by` do schema legado, que alimenta
	// rule.ExceptionEntry; ReviewByAt é o mesmo valor já resolvido pelo
	// decoder, porque o bloco domain não alcança io.clock para parseá-lo.
	ReviewByAt   exception.Instant
	InvalidDates []string

	PresentID            bool
	PresentObject        bool
	PresentADR           bool
	PresentJustification bool
	PresentConvergence   bool
	PresentValidFrom     bool
	PresentValidUntil    bool
	PresentHistory       bool
}
