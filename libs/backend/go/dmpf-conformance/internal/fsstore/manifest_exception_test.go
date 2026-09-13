package fsstore

import (
	"slices"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/manifest"
)

var wantValidFrom = exception.Instant(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano())

func TestDecodeExceptionLegacyOnly(t *testing.T) {
	raw := []byte(`{"schema":"dmpf/units@1","units":[],"exceptions":[{
		"unit":"u","dependency":"d","reason":"r","owner":"team:o","review_by":"2027-01-01"
	}]}`)
	doc, err := DecodeManifest("test.json", "mod", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Exceptions) != 1 {
		t.Fatalf("got %d exceptions, want 1", len(doc.Exceptions))
	}
	x := doc.Exceptions[0]

	if x.Unit != "u" || x.Dependency != "d" || x.Reason != "r" || x.Owner != "team:o" || x.ReviewBy != "2027-01-01" {
		t.Fatalf("legacy fields: %+v", x)
	}
	if x.PresentID || x.PresentADR || x.PresentConvergence || x.PresentValidFrom || x.PresentHistory {
		t.Fatal("new Present* should be false for legacy-only")
	}
	if x.Object.Unit != "u" || x.Object.Identity != "d" {
		t.Fatalf("legacy should fill object: got unit=%q identity=%q", x.Object.Unit, x.Object.Identity)
	}
	if x.Justification != "r" || !x.PresentJustification {
		t.Fatalf("legacy reason should fill justification: got %q present=%v", x.Justification, x.PresentJustification)
	}
}

func TestDecodeExceptionNewOnly(t *testing.T) {
	raw := []byte(`{"schema":"dmpf/units@1","units":[],"exceptions":[{
		"unit":"","dependency":"","reason":"","owner":"","review_by":"",
		"id":"x:y",
		"object":{"kind":"external-dependency","unit":"u2","identity":"d2"},
		"adr":"ADR-001","justification":"j",
		"convergence":{"kind":"review","review_by":"2027-01-01","approved_by":["arquitetura","plataforma"],"replanning_condition":"rc"},
		"valid_from":"2026-01-01T00:00:00Z",
		"history":[{"event":"granted","at":"2026-01-01T00:00:00Z","by":"team:o"}]
	}]}`)
	doc, err := DecodeManifest("test.json", "mod", raw)
	if err != nil {
		t.Fatal(err)
	}
	x := doc.Exceptions[0]

	if !x.PresentID || x.ID != "x:y" {
		t.Fatalf("id: got %q present=%v", x.ID, x.PresentID)
	}
	if !x.PresentObject || x.Object.Kind != "external-dependency" || x.Object.Unit != "u2" || x.Object.Identity != "d2" {
		t.Fatalf("object: %+v present=%v", x.Object, x.PresentObject)
	}
	if !x.PresentADR || x.ADR != "ADR-001" {
		t.Fatalf("adr: got %q present=%v", x.ADR, x.PresentADR)
	}
	if !x.PresentJustification || x.Justification != "j" {
		t.Fatalf("justification: got %q present=%v", x.Justification, x.PresentJustification)
	}
	if !x.PresentConvergence || x.Convergence.Kind != "review" {
		t.Fatalf("convergence: %+v present=%v", x.Convergence, x.PresentConvergence)
	}
	if len(x.Convergence.ApprovedBy) != 2 {
		t.Fatalf("approved_by: got %v", x.Convergence.ApprovedBy)
	}
	if !x.PresentValidFrom || x.ValidFrom != wantValidFrom {
		t.Fatalf("valid_from: got %d present=%v", x.ValidFrom, x.PresentValidFrom)
	}
	if !x.PresentHistory || len(x.History) != 1 || x.History[0].Event != "granted" {
		t.Fatalf("history: %+v present=%v", x.History, x.PresentHistory)
	}
}

func TestDecodeExceptionDual(t *testing.T) {
	raw := []byte(`{"schema":"dmpf/units@1","units":[],"exceptions":[{
		"unit":"u","dependency":"d","reason":"r","owner":"team:o","review_by":"2027-01-01",
		"id":"u:d",
		"object":{"kind":"external-dependency","unit":"u","identity":"d"},
		"adr":"ADR-033","justification":"r",
		"convergence":{"kind":"review","review_by":"2027-01-01","approved_by":["arquitetura","plataforma"],"replanning_condition":"rc"},
		"valid_from":"2026-09-12T00:00:00Z",
		"history":[{"event":"granted","at":"2026-09-12T00:00:00Z","by":"team:o"}]
	}]}`)
	doc, err := DecodeManifest("test.json", "mod", raw)
	if err != nil {
		t.Fatal(err)
	}
	x := doc.Exceptions[0]

	if x.Unit != "u" || x.Dependency != "d" {
		t.Fatalf("legacy fields missing: %+v", x)
	}
	if !x.PresentID || !x.PresentObject || !x.PresentADR || !x.PresentConvergence || !x.PresentHistory {
		t.Fatal("dual: all new Present* should be true")
	}
	want := manifest.ExceptionObject{
		Kind: "external-dependency", Unit: "u", Identity: "d",
		PresentKind: true, PresentUnit: true, PresentIdentity: true,
	}
	if x.Object != want {
		t.Fatalf("object: got %+v, want %+v", x.Object, want)
	}
}

func TestDecodeMarcaDataInvalidaSemVirarAusente(t *testing.T) {
	raw := []byte(`{"schema":"dmpf/units@1","units":[],"exceptions":[{
		"unit":"u","dependency":"d","reason":"r","owner":"team:o","review_by":"amanhã",
		"valid_until":"31/12/2026",
		"history":[{"event":"granted","at":"ontem","by":"team:o"}]
	}]}`)
	doc, err := DecodeManifest("test.json", "mod", raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"review_by", "valid_until", "history[0].at"}
	if got := doc.Exceptions[0].InvalidDates; !slices.Equal(got, want) {
		t.Fatalf("datas inválidas marcadas %v, esperado %v", got, want)
	}
}

func TestDecodeRecusaChaveRepetida(t *testing.T) {
	casos := map[string]string{
		"mesma chave":     `{"schema":"dmpf/units@1","exceptions":[{"object":{"identity":"reflect","identity":"unsafe"}}]}`,
		"só a caixa muda": `{"schema":"dmpf/units@1","exceptions":[{"object":{"identity":"reflect"},"OBJECT":{"identity":"unsafe"}}]}`,
	}
	for nome, raw := range casos {
		t.Run(nome, func(t *testing.T) {
			if _, err := DecodeManifest("test.json", "mod", []byte(raw)); err == nil {
				t.Fatal("chave repetida decodificada sem erro")
			}
		})
	}
}

func TestDecodeAceitaMesmaChaveEmObjetosIrmaos(t *testing.T) {
	raw := []byte(`{"schema":"dmpf/units@1","units":[{"id":"a","include":["m/a"]},{"id":"b","include":[]}],"exceptions":[]}`)
	if _, err := DecodeManifest("test.json", "mod", raw); err != nil {
		t.Fatalf("objetos irmãos com as mesmas chaves recusados: %v", err)
	}
}
