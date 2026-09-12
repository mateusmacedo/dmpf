package exception

import (
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

// lifecycleDiagnostics cobre a consistência interna das datas e do histórico
// (X005). Vencimento é decidido em expired, porque depende de `now`.
func lifecycleDiagnostics(x Exception) []rule.Diagnostic {
	var out []rule.Diagnostic
	add := func(detail string) {
		out = append(out, rule.Diagnostic{Code: rule.CodeX005, Target: x.ID, Detail: detail})
	}

	if x.ValidFrom != 0 && x.ValidUntil != 0 && x.ValidFrom > x.ValidUntil {
		add("valid_from posterior a valid_until")
	}
	// A revisão existe para decidir antes do vencimento; marcada depois, a
	// exceção expira sem que ninguém tenha olhado para ela.
	if x.ReviewBy != 0 && x.ValidUntil != 0 && x.ReviewBy > x.ValidUntil {
		add("review_by posterior a valid_until")
	}

	if len(x.History) == 0 {
		add("history vazio: a exceção precisa registrar ao menos a concessão")
		return out
	}
	if x.History[0].Event != EventGranted {
		add(fmt.Sprintf("history[0] é %q, e a concessão é sempre o primeiro evento", x.History[0].Event))
	}
	for i, h := range x.History {
		if !IsEvent(h.Event) {
			add(fmt.Sprintf("history[%d]: evento %q fora do conjunto fechado", i, h.Event))
		}
		if h.By == "" {
			add(fmt.Sprintf("history[%d]: evento sem autor", i))
		}
		// Renovar sem motivo transforma o histórico em carimbo: as métricas de
		// GOV-36 contariam renovações sem dizer por que a dívida persistiu.
		if h.Event == EventRenewed && h.Reason == "" {
			add(fmt.Sprintf("history[%d]: renewed sem reason", i))
		}
	}
	return out
}

// expired decide o vencimento de GOV-34: a exceção vale até `valid_until`, e
// uma renovação registrada DEPOIS dessa data a reabre.
func expired(x Exception, now Instant) bool {
	if x.ValidUntil == 0 || now <= x.ValidUntil {
		return false
	}
	for _, h := range x.History {
		if h.Event == EventRenewed && h.At > x.ValidUntil {
			return false
		}
	}
	return true
}

func expiryDiagnostics(x Exception, now Instant) []rule.Diagnostic {
	if !expired(x, now) {
		return nil
	}
	return []rule.Diagnostic{{
		Code:   rule.CodeX006,
		Target: x.ID,
		Detail: "exceção vencida sem renovação posterior: deixa de autorizar e a unidade fica não conforme",
	}}
}
