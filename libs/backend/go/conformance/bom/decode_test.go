package bom

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/exception"
)

func TestDecodeDistingueAusenteDeVazio(t *testing.T) {
	doc, err := Decode([]byte(`{
		"runtimes": {"entries": [], "reason": ""},
		"generators": {"entries": [
			{"subject": "generator", "compatible_with": [], "cve": []},
			{"subject": "generator"}
		]},
		"exceptions": [],
		"metrics": {"vigentes": 0}
	}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(doc.Sections) != len(sectionNames) {
		t.Fatalf("%d seções decodificadas, esperado %d", len(doc.Sections), len(sectionNames))
	}
	if !doc.Sections[0].Present || doc.Sections[2].Present {
		t.Errorf("presença das seções: runtimes=%v drivers=%v", doc.Sections[0].Present, doc.Sections[2].Present)
	}
	vazia, ausente := doc.Sections[1].Entries[0], doc.Sections[1].Entries[1]
	if !vazia.PresentCVE || !vazia.PresentCompatibleWith {
		t.Error("cve e compatible_with vazios decodificados como ausentes")
	}
	if ausente.PresentCVE || ausente.PresentCompatibleWith {
		t.Error("cve e compatible_with ausentes decodificados como presentes")
	}
	if !doc.PresentExceptions || !doc.PresentMetrics || doc.PresentSemconv {
		t.Errorf("presença: exceptions=%v metrics=%v semconv=%v", doc.PresentExceptions, doc.PresentMetrics, doc.PresentSemconv)
	}
}

func TestDecodeExcecaoComDatas(t *testing.T) {
	doc, err := Decode([]byte(`{"exceptions": [{
		"id": "X-bom-e2",
		"object": {"kind": "bom-combination", "identity": "go@1.27.0"},
		"valid_until": "2026-12-31",
		"history": [{"event": "granted", "at": "2026-09-01T00:00:00Z", "by": "team:plataforma"}]
	}]}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	x := doc.Exceptions[0]
	if x.Object.Kind != exception.KindBOMCombination || x.Object.PresentUnit {
		t.Errorf("object decodificado errado: %+v", x.Object)
	}
	if !x.PresentValidUntil || x.ValidUntil == 0 || x.History[0].At == 0 {
		t.Errorf("datas não viraram instante: valid_until=%d at=%d", x.ValidUntil, x.History[0].At)
	}
	if x.PresentADR || x.PresentConvergence {
		t.Error("campos ausentes decodificados como presentes")
	}
}

func TestDecodeRecusaJSONInvalido(t *testing.T) {
	if _, err := Decode([]byte(`{"schema":`)); err == nil {
		t.Fatal("JSON truncado decodificado sem erro")
	}
}

func TestDecodeRecusaChaveRepetida(t *testing.T) {
	if _, err := Decode([]byte(`{"release": "0.1.0", "Release": "0.2.0"}`)); err == nil {
		t.Fatal("chave repetida decodificada sem erro")
	}
}

func TestDecodeMarcaDataInvalidaDaExcecao(t *testing.T) {
	doc, err := Decode([]byte(`{"exceptions": [{"valid_until": "31/12/2026", "review_by": "2026-13-01"}]}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want := []string{"valid_until", "review_by"}
	if got := doc.Exceptions[0].InvalidDates; !slices.Equal(got, want) {
		t.Fatalf("datas inválidas %v, esperado %v", got, want)
	}
}
