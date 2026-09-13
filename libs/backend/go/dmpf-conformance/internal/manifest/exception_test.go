package manifest

import (
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

func instant(s string) exception.Instant {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return exception.Instant(t.UnixNano())
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return exception.Instant(t.UnixNano())
}

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
				ReviewByAt: instant("2027-01-01"),
				ValidUntil: instant("2027-01-01"),
				ID:         "X-test-time",
				Object: ExceptionObject{
					Kind: "external-dependency", Unit: "u", Identity: "time",
					PresentKind: true, PresentUnit: true, PresentIdentity: true,
				},
				ADR:           "ADR-099",
				Justification: "teste",
				Convergence: ExceptionConvergence{
					Kind:                "review",
					ReviewBy:            instant("2027-01-01"),
					ApprovedBy:          []string{"arquitetura", "plataforma"},
					ReplanningCondition: "cond",
					PresentKind:         true,
				},
				History: []ExceptionHistoryEntry{{Event: "granted", At: instant("2026-01-01T00:00:00Z"), By: "team:eng"}},

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
				ReviewByAt: instant("2027-01-01"),
				ValidUntil: instant("2027-01-01"),
				ID:         "X-test-reflect",
				Object: ExceptionObject{
					Kind: "external-dependency", Unit: "u", Identity: "reflect",
					PresentKind: true, PresentUnit: true, PresentIdentity: true,
				},
				ADR:           "ADR-033",
				Justification: "protoc-gen-go emits reflect",
				Convergence: ExceptionConvergence{
					Kind:                "review",
					ReviewBy:            instant("2027-01-01"),
					ApprovedBy:          []string{"arquitetura", "plataforma"},
					ReplanningCondition: "cond",
					PresentKind:         true,
				},
				History: []ExceptionHistoryEntry{{Event: "granted", At: instant("2026-01-01T00:00:00Z"), By: "team:eng"}},

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
