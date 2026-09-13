package bom

import (
	"fmt"
	"maps"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

func DeriveMetrics(exceptions []exception.Exception, now exception.Instant) Metrics {
	m := Metrics{Renovacoes: map[string]int{}}
	for _, x := range exceptions {
		var renewals int
		var converged, revokedInTime bool
		for _, h := range x.History {
			switch h.Event {
			case exception.EventRenewed:
				renewals++
			case exception.EventConverged:
				converged = true
			case exception.EventRevoked:
				revokedInTime = revokedInTime || x.ValidUntil == 0 || h.At <= x.ValidUntil
			}
		}
		m.Renovacoes[x.ID] = renewals

		expired := exception.Expired(x, now)
		// Revogada dentro da vigência encerrou, não venceu; revogada depois de
		// vencer é o registro de que venceu sem convergir.
		if expired && !converged && !revokedInTime {
			m.VencidasSemConvergencia++
		}
		if !expired && !exception.Closed(x) {
			m.Vigentes++
		}
	}
	return m
}

func (v *validator) checkMetrics() {
	if !v.doc.PresentMetrics {
		v.add(rule.CodeB010, "metrics", "", "metrics ausente: GOV-36 as publica junto do BOM")
		return
	}
	want := DeriveMetrics(v.doc.Exceptions, v.in.Now)
	got := v.doc.Metrics
	if got.Vigentes != want.Vigentes {
		v.add(rule.CodeB010, "metrics.vigentes", "",
			fmt.Sprintf("declara %d, o histórico deriva %d", got.Vigentes, want.Vigentes))
	}
	if !maps.Equal(got.Renovacoes, want.Renovacoes) {
		v.add(rule.CodeB010, "metrics.renovacoes", "",
			fmt.Sprintf("declara %v, o histórico deriva %v", got.Renovacoes, want.Renovacoes))
	}
	if got.VencidasSemConvergencia != want.VencidasSemConvergencia {
		v.add(rule.CodeB010, "metrics.vencidas_sem_convergencia", "",
			fmt.Sprintf("declara %d, o histórico deriva %d", got.VencidasSemConvergencia, want.VencidasSemConvergencia))
	}
}
