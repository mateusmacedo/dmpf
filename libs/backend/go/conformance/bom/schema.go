// Package bom valida o BOM da release do produto DMPF (dmpf/bom@1) contra
// BOM-01 a BOM-10 e admite as exceções E2/E3 pelo mesmo rito das E1.
package bom

import (
	"fmt"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/exception"
)

const (
	SchemaID    = "dmpf/bom@1"
	Product     = "dmpf"
	SlotSemconv = "semantic_conventions_messaging"
)

var sectionNames = []string{
	"runtimes", "generators", "drivers_clients_sdks",
	"compatible_combinations", "deprecations", "cves",
}

type Subject string

const (
	SubjectProduct   Subject = "product"
	SubjectKernel    Subject = "kernel"
	SubjectContract  Subject = "contract"
	SubjectRuntime   Subject = "runtime"
	SubjectGenerator Subject = "generator"
	SubjectDriver    Subject = "driver"
	SubjectClient    Subject = "client"
	SubjectSDK       Subject = "sdk"
	SubjectSemconv   Subject = SlotSemconv
)

func isSubject(s Subject) bool {
	return slices.Contains([]Subject{
		SubjectProduct, SubjectKernel, SubjectContract, SubjectRuntime,
		SubjectGenerator, SubjectDriver, SubjectClient, SubjectSDK, SubjectSemconv,
	}, s)
}

type State string

const (
	StateProposta     State = "proposta"
	StateCandidata    State = "candidata"
	StateCertificada  State = "certificada"
	StateDepreciada   State = "depreciada"
	StateNaoSuportada State = "nao_suportada"
	StateRejeitada    State = "rejeitada"
)

func isState(s State) bool {
	return slices.Contains([]State{
		StateProposta, StateCandidata, StateCertificada,
		StateDepreciada, StateNaoSuportada, StateRejeitada,
	}, s)
}

type Criticality string

const (
	CriticalityCritica Criticality = "critica"
	CriticalityPadrao  Criticality = "padrao"
)

func isCriticality(c Criticality) bool {
	return c == CriticalityCritica || c == CriticalityPadrao
}

type CVEState string

const (
	CVECorrigida CVEState = "corrigida"
	CVEMitigada  CVEState = "mitigada"
	CVEAberta    CVEState = "aberta"
)

func isCVEState(s CVEState) bool {
	return s == CVECorrigida || s == CVEMitigada || s == CVEAberta
}

// Sections traz sempre as seis de FND-10 §4.1, na ordem canônica: a ausente
// chega com Present falso, porque ausência e vazio declarado não são a mesma
// coisa (BOM-01).
type Document struct {
	Schema   string
	Product  string
	Release  string
	Tag      string
	Sections []Section

	Semconv    Entry
	Exceptions []exception.Exception
	Metrics    Metrics

	PresentSemconv    bool
	PresentExceptions bool
	PresentMetrics    bool
}

type Section struct {
	Name    string
	Entries []Entry
	Reason  string
	Present bool
}

type Entry struct {
	Subject        Subject
	Identity       string
	Version        string
	State          State
	Criticality    Criticality
	CompatibleWith []Compatibility
	RegistryRef    *RegistryRef
	EvidenceURI    string
	EvidenceDigest string
	ApprovedBy     string
	CertifiedAt    string
	ValidUntil     string
	Promoted       *Promotion
	DeprecatedAt   string
	Successor      string
	CVE            []CVE
	Owner          string

	PresentCompatibleWith bool
	PresentCVE            bool
}

type Compatibility struct {
	Identity string `json:"identity"`
	Version  string `json:"version"`
	Evidence string `json:"evidence"`
}

type RegistryRef struct {
	File     string `json:"file"`
	Selector string `json:"selector"`
}

type Promotion struct {
	By         string `json:"by"`
	ReviewedBy string `json:"reviewed_by"`
	PR         string `json:"pr"`
}

type CVE struct {
	ID    string   `json:"id"`
	State CVEState `json:"state"`
	Owner string   `json:"owner"`
}

type Metrics struct {
	Vigentes                int
	Renovacoes              map[string]int
	VencidasSemConvergencia int
}

type located struct {
	section string
	path    string
	entry   Entry
}

func (d Document) located() []located {
	var out []located
	for _, s := range d.Sections {
		for i, e := range s.Entries {
			out = append(out, located{section: s.Name, path: fmt.Sprintf("%s.entries[%d]", s.Name, i), entry: e})
		}
	}
	if d.PresentSemconv {
		out = append(out, located{section: SlotSemconv, path: SlotSemconv, entry: d.Semconv})
	}
	return out
}

// A seção fica fora da chave: a entrada que muda de seção ao ser depreciada
// continua sendo a mesma entrada para a máquina de BOM-07.
func (l located) key() string {
	return string(l.entry.Subject) + "\x00" + l.entry.Identity + "\x00" + l.entry.Version
}
