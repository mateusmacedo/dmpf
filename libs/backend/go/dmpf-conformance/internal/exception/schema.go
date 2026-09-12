// Package exception decide a admissão de um pedido de exceção sobre modelo JÁ
// DECODIFICADO, como o manifest: decodificar é acesso a formato de wire, vedado
// neste bloco, e vive no fsstore.
package exception

import "slices"

// Instant é nanossegundo desde o epoch Unix, não time.Time: o verificador
// classifica o package time inteiro como io.clock, capability que o bloco
// domain não tem. O zero significa ausente — não 1970.
type Instant int64

// Kind é o conjunto E1–E3 de GOV-31. Fechado: pedido cujo objeto não cai em
// nenhuma das três não é exceção prevista, e X002 o recusa.
type Kind string

const (
	KindExternalDependency   Kind = "external-dependency"
	KindBOMCombination       Kind = "bom-combination"
	KindGovernanceInstrument Kind = "governance-instrument"
)

func Kinds() []Kind {
	return []Kind{KindExternalDependency, KindBOMCombination, KindGovernanceInstrument}
}

func IsKind(k Kind) bool {
	return slices.Contains(Kinds(), k)
}

// Event é o conjunto fechado de GOV-34. O histórico é a fonte das métricas de
// GOV-36; evento fora do conjunto tornaria a contagem indefinida.
type Event string

const (
	EventGranted   Event = "granted"
	EventRenewed   Event = "renewed"
	EventRevoked   Event = "revoked"
	EventConverged Event = "converged"
)

func Events() []Event {
	return []Event{EventGranted, EventRenewed, EventRevoked, EventConverged}
}

func IsEvent(e Event) bool {
	return slices.Contains(Events(), e)
}

// ConvergenceKind distingue os dois ramos de GOV-30: prazo com condição, ou
// revisão com aprovação dual.
type ConvergenceKind string

const (
	ConvergencePlan   ConvergenceKind = "plan"
	ConvergenceReview ConvergenceKind = "review"
)

// Approver nomeia as duas autoridades que o ramo de revisão exige, ambas.
type Approver string

const (
	ApproverArchitecture Approver = "arquitetura"
	ApproverPlatform     Approver = "plataforma"
)

// Object é o que a exceção pede para autorizar. `Unit` é o ID da unidade no
// manifesto; `Identity` é o objeto nominal — um import path em E1, uma
// combinação em E2, um instrumento em E3.
type Object struct {
	Kind     Kind
	Unit     string
	Identity string

	PresentKind     bool
	PresentUnit     bool
	PresentIdentity bool
}

// União dos dois ramos num struct só: qual deles vale é `Kind`, e o outro
// conjunto de campos fica zerado. Struct separado por ramo exigiria interface
// ou type switch para um par de alternativas fechado.
type Convergence struct {
	Kind ConvergenceKind

	Deadline  Instant
	Condition string

	ReviewBy            Instant
	ApprovedBy          []Approver
	ReplanningCondition string

	PresentKind bool
}

type HistoryEntry struct {
	Event  Event
	At     Instant
	By     string
	Reason string
}

// Present* distingue "campo ausente" de "presente com valor zero", como no
// manifest: sem herança, não há de onde derivar o que falta.
type Exception struct {
	ID            string
	Object        Object
	ADR           string
	Owner         string
	Justification string
	Convergence   Convergence
	ValidFrom     Instant
	ValidUntil    Instant
	ReviewBy      Instant
	History       []HistoryEntry

	PresentID            bool
	PresentADR           bool
	PresentOwner         bool
	PresentJustification bool
	PresentConvergence   bool
	PresentValidFrom     bool
	PresentValidUntil    bool
	PresentReviewBy      bool
	PresentHistory       bool
}
