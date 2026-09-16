package manifest

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

const (
	jan1st2026 exception.Instant = 1_767_225_600_000_000_000
	jan1st2027 exception.Instant = 1_798_761_600_000_000_000
)

func isStdlib(path string) bool {
	_, ok := rule.StdlibCapability(path)
	return ok
}

func TestAdmitidasDomainIORecusadaPorN1(t *testing.T) {
	doc := Document{
		Units: []Unit{
			{ID: "u", Block: "domain", BoundedContext: "ctx", PresentID: true, PresentBlock: true, PresentBoundedContext: true},
		},
		Exceptions: []Exception{
			{
				Owner:      "team:eng",
				ReviewBy:   "2027-01-01",
				ReviewByAt: jan1st2027,
				ValidUntil: jan1st2027,
				ID:         "X-test-time",
				Object: ExceptionObject{
					Kind: "external-dependency", Unit: "u", Identity: "time",
					PresentKind: true, PresentUnit: true, PresentIdentity: true,
				},
				ADR:           "ADR-099",
				Justification: "teste",
				Convergence: ExceptionConvergence{
					Kind:                "review",
					ReviewBy:            jan1st2027,
					ApprovedBy:          []string{"arquitetura", "plataforma"},
					ReplanningCondition: "cond",
					PresentKind:         true,
				},
				History: []ExceptionHistoryEntry{{Event: "granted", At: jan1st2026, By: "team:eng"}},

				PresentID: true, PresentObject: true, PresentADR: true,
				PresentJustification: true, PresentConvergence: true, PresentHistory: true,
				PresentValidUntil: true,
			},
		},
	}

	admitted, xDiags := Admitidas(doc, rule.ExternalPolicy{}, isStdlib, 0)
	if len(admitted) != 0 {
		t.Fatalf("domain + io.clock should not be admitted, got %d admitted", len(admitted))
	}
	found := false
	for _, d := range xDiags {
		if d.Code == rule.CodeX003 && strings.Contains(d.Detail, "io.clock") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected X003 (N1 block policy), got %v", xDiags)
	}
}

func TestAdmitidasReflectAdmitida(t *testing.T) {
	doc := Document{
		Units: []Unit{
			{ID: "u", Block: "contract", BoundedContext: "ctx", PresentID: true, PresentBlock: true, PresentBoundedContext: true},
		},
		Exceptions: []Exception{
			{
				Owner:      "team:eng",
				ReviewBy:   "2027-01-01",
				ReviewByAt: jan1st2027,
				ValidUntil: jan1st2027,
				ID:         "X-test-reflect",
				Object: ExceptionObject{
					Kind: "external-dependency", Unit: "u", Identity: "reflect",
					PresentKind: true, PresentUnit: true, PresentIdentity: true,
				},
				ADR:           "ADR-033",
				Justification: "protoc-gen-go emits reflect",
				Convergence: ExceptionConvergence{
					Kind:                "review",
					ReviewBy:            jan1st2027,
					ApprovedBy:          []string{"arquitetura", "plataforma"},
					ReplanningCondition: "cond",
					PresentKind:         true,
				},
				History: []ExceptionHistoryEntry{{Event: "granted", At: jan1st2026, By: "team:eng"}},

				PresentID: true, PresentObject: true, PresentADR: true,
				PresentJustification: true, PresentConvergence: true, PresentHistory: true,
				PresentValidUntil: true,
			},
		},
	}

	admitted, xDiags := Admitidas(doc, rule.ExternalPolicy{}, isStdlib, 0)
	if len(xDiags) != 0 {
		t.Fatalf("contract + reflect should be admitted, got diags: %v", xDiags)
	}
	if len(admitted) != 1 {
		t.Fatalf("expected 1 admitted, got %d", len(admitted))
	}
}
