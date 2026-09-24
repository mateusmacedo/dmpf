package bom

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/fsstore"
)

type wireDocument struct {
	Schema  string `json:"schema"`
	Product string `json:"product"`
	Release string `json:"release"`
	Tag     string `json:"tag"`

	Runtimes               *wireSection `json:"runtimes"`
	Generators             *wireSection `json:"generators"`
	DriversClientsSDKs     *wireSection `json:"drivers_clients_sdks"`
	CompatibleCombinations *wireSection `json:"compatible_combinations"`
	Deprecations           *wireSection `json:"deprecations"`
	CVEs                   *wireSection `json:"cves"`

	Semconv    *wireEntry       `json:"semantic_conventions_messaging"`
	Exceptions *[]wireException `json:"exceptions"`
	Metrics    *wireMetrics     `json:"metrics"`
}

type wireSection struct {
	Entries []wireEntry `json:"entries"`
	Reason  string      `json:"reason"`
}

type wireEntry struct {
	Subject        string           `json:"subject"`
	Identity       string           `json:"identity"`
	Version        string           `json:"version"`
	State          string           `json:"state"`
	Criticality    string           `json:"criticality"`
	CompatibleWith *[]Compatibility `json:"compatible_with"`
	RegistryRef    *RegistryRef     `json:"registry_ref"`
	EvidenceURI    string           `json:"evidence_uri"`
	EvidenceDigest string           `json:"evidence_digest"`
	ApprovedBy     string           `json:"approved_by"`
	CertifiedAt    string           `json:"certified_at"`
	ValidUntil     string           `json:"valid_until"`
	Promoted       *Promotion       `json:"promoted"`
	DeprecatedAt   string           `json:"deprecated_at"`
	Successor      string           `json:"successor"`
	CVE            *[]CVE           `json:"cve"`
	Owner          string           `json:"owner"`
}

type wireException struct {
	ID            *string          `json:"id"`
	Object        *wireObject      `json:"object"`
	ADR           *string          `json:"adr"`
	Owner         *string          `json:"owner"`
	Justification *string          `json:"justification"`
	Convergence   *wireConvergence `json:"convergence"`
	ValidFrom     *string          `json:"valid_from"`
	ValidUntil    *string          `json:"valid_until"`
	ReviewBy      *string          `json:"review_by"`
	History       *[]wireHistory   `json:"history"`
}

type wireObject struct {
	Kind     *string `json:"kind"`
	Unit     *string `json:"unit"`
	Identity *string `json:"identity"`
}

type wireConvergence struct {
	Kind                *string  `json:"kind"`
	Deadline            string   `json:"deadline"`
	Condition           string   `json:"condition"`
	ReviewBy            string   `json:"review_by"`
	ApprovedBy          []string `json:"approved_by"`
	ReplanningCondition string   `json:"replanning_condition"`
}

type wireHistory struct {
	Event  string `json:"event"`
	At     string `json:"at"`
	By     string `json:"by"`
	Reason string `json:"reason"`
}

type wireMetrics struct {
	Vigentes                int            `json:"vigentes"`
	Renovacoes              map[string]int `json:"renovacoes"`
	VencidasSemConvergencia int            `json:"vencidas_sem_convergencia"`
}

func Decode(raw []byte) (Document, error) {
	var w wireDocument
	if err := json.Unmarshal(raw, &w); err != nil {
		return Document{}, fmt.Errorf("decodificar BOM: %w", err)
	}
	if chave, repetida := fsstore.DuplicateKey(raw); repetida {
		return Document{}, fmt.Errorf("decodificar BOM: chave %q repetida no mesmo objeto", chave)
	}

	doc := Document{Schema: w.Schema, Product: w.Product, Release: w.Release, Tag: w.Tag}
	sections := []*wireSection{
		w.Runtimes, w.Generators, w.DriversClientsSDKs,
		w.CompatibleCombinations, w.Deprecations, w.CVEs,
	}
	for i, s := range sections {
		sec := Section{Name: sectionNames[i], Present: s != nil}
		if s != nil {
			sec.Reason = s.Reason
			for _, e := range s.Entries {
				sec.Entries = append(sec.Entries, e.entry())
			}
		}
		doc.Sections = append(doc.Sections, sec)
	}

	if w.Semconv != nil {
		doc.Semconv = w.Semconv.entry()
		doc.PresentSemconv = true
	}
	if w.Exceptions != nil {
		doc.PresentExceptions = true
		for _, x := range *w.Exceptions {
			doc.Exceptions = append(doc.Exceptions, x.exception())
		}
	}
	if w.Metrics != nil {
		doc.PresentMetrics = true
		doc.Metrics = Metrics{
			Vigentes:                w.Metrics.Vigentes,
			Renovacoes:              w.Metrics.Renovacoes,
			VencidasSemConvergencia: w.Metrics.VencidasSemConvergencia,
		}
	}
	return doc, nil
}

func (w wireEntry) entry() Entry {
	e := Entry{
		Subject:        Subject(w.Subject),
		Identity:       w.Identity,
		Version:        w.Version,
		State:          State(w.State),
		Criticality:    Criticality(w.Criticality),
		RegistryRef:    w.RegistryRef,
		EvidenceURI:    w.EvidenceURI,
		EvidenceDigest: w.EvidenceDigest,
		ApprovedBy:     w.ApprovedBy,
		CertifiedAt:    w.CertifiedAt,
		ValidUntil:     w.ValidUntil,
		Promoted:       w.Promoted,
		DeprecatedAt:   w.DeprecatedAt,
		Successor:      w.Successor,
		Owner:          w.Owner,
	}
	if w.CompatibleWith != nil {
		e.CompatibleWith = *w.CompatibleWith
		e.PresentCompatibleWith = true
	}
	if w.CVE != nil {
		e.CVE = *w.CVE
		e.PresentCVE = true
	}
	return e
}

func (w wireException) exception() exception.Exception {
	var invalid []string
	instante := func(campo, valor string, parse func(string) (time.Time, bool)) exception.Instant {
		t, ok := parse(valor)
		if !ok {
			if valor != "" {
				invalid = append(invalid, campo)
			}
			return 0
		}
		return exception.Instant(t.UnixNano())
	}

	x := exception.Exception{
		ID:            deref(w.ID),
		ADR:           deref(w.ADR),
		Owner:         deref(w.Owner),
		Justification: deref(w.Justification),
		ValidFrom:     instante("valid_from", deref(w.ValidFrom), parseDate),
		ValidUntil:    instante("valid_until", deref(w.ValidUntil), parseUntil),
		ReviewBy:      instante("review_by", deref(w.ReviewBy), parseDate),

		PresentID:            w.ID != nil,
		PresentADR:           w.ADR != nil,
		PresentOwner:         w.Owner != nil,
		PresentJustification: w.Justification != nil,
		PresentConvergence:   w.Convergence != nil,
		PresentValidFrom:     w.ValidFrom != nil,
		PresentValidUntil:    w.ValidUntil != nil,
		PresentReviewBy:      w.ReviewBy != nil,
		PresentHistory:       w.History != nil,
	}
	if o := w.Object; o != nil {
		x.Object = exception.Object{
			Kind:            exception.Kind(deref(o.Kind)),
			Unit:            deref(o.Unit),
			Identity:        deref(o.Identity),
			PresentKind:     o.Kind != nil,
			PresentUnit:     o.Unit != nil,
			PresentIdentity: o.Identity != nil,
		}
	}
	if c := w.Convergence; c != nil {
		x.Convergence = exception.Convergence{
			Kind:                exception.ConvergenceKind(deref(c.Kind)),
			Deadline:            instante("convergence.deadline", c.Deadline, parseDate),
			Condition:           c.Condition,
			ReviewBy:            instante("convergence.review_by", c.ReviewBy, parseDate),
			ReplanningCondition: c.ReplanningCondition,
			PresentKind:         c.Kind != nil,
		}
		for _, a := range c.ApprovedBy {
			x.Convergence.ApprovedBy = append(x.Convergence.ApprovedBy, exception.Approver(a))
		}
	}
	if w.History != nil {
		for i, h := range *w.History {
			x.History = append(x.History, exception.HistoryEntry{
				Event:  exception.Event(h.Event),
				At:     instante(fmt.Sprintf("history[%d].at", i), h.At, parseDate),
				By:     h.By,
				Reason: h.Reason,
			})
		}
	}
	x.InvalidDates = invalid
	return x
}

func parseDate(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// Data sem hora vale o dia inteiro: `valid_until: 2026-09-12` ainda vale ao
// meio-dia do dia 12.
func parseUntil(s string) (time.Time, bool) {
	t, ok := parseDate(s)
	if ok && len(s) == len(time.DateOnly) {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t, ok
}

func instant(s string) exception.Instant {
	t, ok := parseDate(s)
	if !ok {
		return 0
	}
	return exception.Instant(t.UnixNano())
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
