package bom

import (
	"maps"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/exception"
)

func excecaoCom(id, validUntil string, eventos ...exception.HistoryEntry) exception.Exception {
	return exception.Exception{ID: id, ValidUntil: instant(validUntil), History: eventos}
}

func evento(e exception.Event, at string) exception.HistoryEntry {
	return exception.HistoryEntry{Event: e, At: instant(at)}
}

func TestMetricasDerivadasDoHistorico(t *testing.T) {
	granted := evento(exception.EventGranted, "2026-01-01")
	excecoes := []exception.Exception{
		excecaoCom("X-vigente", "2026-12-31", granted,
			evento(exception.EventRenewed, "2026-06-01"), evento(exception.EventRenewed, "2026-08-01")),
		excecaoCom("X-vencida", "2026-09-01", granted),
		excecaoCom("X-convergida", "2026-09-01", granted, evento(exception.EventConverged, "2026-08-15")),
		excecaoCom("X-revogada-na-vigencia", "2026-12-31", granted, evento(exception.EventRevoked, "2026-08-15")),
		excecaoCom("X-revogada-depois-de-vencer", "2026-09-01", granted, evento(exception.EventRevoked, "2026-09-05")),
	}

	got := DeriveMetrics(excecoes, instante(t, agoraFixo))

	if got.Vigentes != 1 {
		t.Errorf("vigentes = %d, esperado 1", got.Vigentes)
	}
	if got.VencidasSemConvergencia != 2 {
		t.Errorf("vencidas_sem_convergencia = %d, esperado 2", got.VencidasSemConvergencia)
	}
	want := map[string]int{
		"X-vigente": 2, "X-vencida": 0, "X-convergida": 0,
		"X-revogada-na-vigencia": 0, "X-revogada-depois-de-vencer": 0,
	}
	if !maps.Equal(got.Renovacoes, want) {
		t.Errorf("renovacoes = %v, esperado %v", got.Renovacoes, want)
	}
}
