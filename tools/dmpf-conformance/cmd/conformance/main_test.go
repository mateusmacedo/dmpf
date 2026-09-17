package main

import (
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/manifest"
)

func TestInstanteDeAceitaSoRFC3339(t *testing.T) {
	got, err := instanteDe("2026-09-12T00:00:00Z")
	if err != nil {
		t.Fatalf("instanteDe: %v", err)
	}
	want := exception.Instant(time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC).UnixNano())
	if got != want {
		t.Errorf("instante %d, esperado %d", got, want)
	}
	if _, err := instanteDe("2026-09-12"); err == nil {
		t.Error("--now aceitou data sem hora")
	}
}

func documentoDeContrato(exceptions ...manifest.Exception) manifest.Document {
	return manifest.Document{
		Path: "m/dmpf-units.json", Module: "m", Schema: manifest.SchemaID,
		Units: []manifest.Unit{{
			ID: "u", Block: "contract", BoundedContext: "bc", Include: []string{"m/p"},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}},
		Exceptions: exceptions,
	}
}

func TestRegravacaoRecusaExcecaoNaoAdmitida(t *testing.T) {
	soLegado := manifest.Exception{
		Unit: "u", Dependency: "reflect", Reason: "legado", Owner: "team:plataforma", ReviewBy: "2027-01-01",
		Object:        manifest.ExceptionObject{Unit: "u", Identity: "reflect", PresentUnit: true, PresentIdentity: true},
		Justification: "legado", PresentJustification: true,
	}

	err := admitido(documentoDeContrato(soLegado), nil, 0)
	if err == nil || !strings.Contains(err.Error(), "exceção não admitida") {
		t.Fatalf("regravação aceitou exceção só com campos legados: %v", err)
	}
}

func TestRegravacaoAceitaManifestoSemExcecao(t *testing.T) {
	if err := admitido(documentoDeContrato(), nil, 0); err != nil {
		t.Fatalf("regravação recusou manifesto válido: %v", err)
	}
}
