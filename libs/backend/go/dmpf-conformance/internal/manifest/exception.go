package manifest

import (
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

type Admitted struct {
	Exc     exception.Exception
	Subject exception.Subject
	Source  Exception
}

func Admitidas(doc Document, policy rule.ExternalPolicy, isStandard func(string) bool, now exception.Instant) ([]Admitted, []rule.Diagnostic) {
	var admitted []Admitted
	var xDiags []rule.Diagnostic

	for _, mx := range doc.Exceptions {
		exc := ToException(mx)
		subj := resolveSubject(mx, doc, policy, isStandard)
		diags := conflictDiagnostics(mx)
		diags = append(diags, exception.AdmitIn(exc, exception.RegistryManifest, subj, now)...)
		if len(diags) == 0 {
			admitted = append(admitted, Admitted{Exc: exc, Subject: subj, Source: mx})
		} else {
			xDiags = append(xDiags, diags...)
		}
	}
	return admitted, xDiags
}

// Quem revisa o PR lê um dos dois pares: divergentes, aprova-se um objeto e
// admite-se outro.
func conflictDiagnostics(mx Exception) []rule.Diagnostic {
	pairs := mx.ConflictPairs()
	if len(pairs) == 0 {
		return nil
	}
	return []rule.Diagnostic{{
		Code:         rule.CodeX001,
		CanonicalKey: mx.Object.Unit,
		Target:       mx.ID,
		Detail:       "representação legada diverge da nova: " + strings.Join(pairs, ", "),
	}}
}

// A autorização sai do objeto que a admissão conferiu, não dos campos legados.
func (a Admitted) entry() rule.ExceptionEntry {
	return rule.ExceptionEntry{
		Unit:       a.Exc.Object.Unit,
		Dependency: a.Exc.Object.Identity,
		Reason:     a.Exc.Justification,
		Owner:      a.Exc.Owner,
		ReviewBy:   a.Source.ReviewBy,
	}
}

func ToException(x Exception) exception.Exception {
	exc := exception.Exception{
		ID:            x.ID,
		ADR:           x.ADR,
		Owner:         x.Owner,
		Justification: x.Justification,
		ValidFrom:     x.ValidFrom,
		ValidUntil:    x.ValidUntil,
		ReviewBy:      x.ReviewByAt,

		PresentID:            x.PresentID,
		PresentADR:           x.PresentADR,
		PresentOwner:         x.Owner != "",
		PresentJustification: x.PresentJustification,
		PresentValidFrom:     x.PresentValidFrom,
		PresentValidUntil:    x.PresentValidUntil,
		PresentReviewBy:      x.ReviewBy != "",
	}

	exc.Object = exception.Object{
		Kind:            exception.Kind(x.Object.Kind),
		Unit:            x.Object.Unit,
		Identity:        x.Object.Identity,
		PresentKind:     x.Object.PresentKind,
		PresentUnit:     x.Object.PresentUnit,
		PresentIdentity: x.Object.PresentIdentity,
	}

	if x.PresentConvergence {
		exc.Convergence = exception.Convergence{
			Kind:                exception.ConvergenceKind(x.Convergence.Kind),
			Deadline:            x.Convergence.Deadline,
			Condition:           x.Convergence.Condition,
			ReviewBy:            x.Convergence.ReviewBy,
			ReplanningCondition: x.Convergence.ReplanningCondition,
			PresentKind:         x.Convergence.PresentKind,
		}
		for _, a := range x.Convergence.ApprovedBy {
			exc.Convergence.ApprovedBy = append(exc.Convergence.ApprovedBy, exception.Approver(a))
		}
		exc.PresentConvergence = true
	}

	if x.PresentHistory {
		for _, h := range x.History {
			exc.History = append(exc.History, exception.HistoryEntry{
				Event:  exception.Event(h.Event),
				At:     h.At,
				By:     h.By,
				Reason: h.Reason,
			})
		}
		exc.PresentHistory = true
	}

	return exc
}

func resolveSubject(mx Exception, doc Document, policy rule.ExternalPolicy, isStandard func(string) bool) exception.Subject {
	var subj exception.Subject
	for _, u := range doc.Units {
		if u.ID == mx.Object.Unit {
			subj.Block = rule.Block(u.Block)
			break
		}
	}
	if mx.Object.Identity != "" {
		cap, _ := rule.ResolveCapability(mx.Object.Identity, policy, isStandard)
		subj.Capability = cap
	}
	return subj
}
