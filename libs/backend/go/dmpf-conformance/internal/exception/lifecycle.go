package exception

import (
	"fmt"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

// lifecycleDiagnostics cobre a consistência interna das datas e do histórico
// (X005). Vencimento é decidido em Expired, porque depende de `now`.
func lifecycleDiagnostics(x Exception) []rule.Diagnostic {
	var out []rule.Diagnostic
	add := func(detail string) {
		out = append(out, rule.Diagnostic{Code: rule.CodeX005, Target: x.ID, Detail: detail})
	}

	for _, campo := range x.InvalidDates {
		add(fmt.Sprintf("%s não é data RFC3339 nem AAAA-MM-DD", campo))
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
		// Renovar é conceder de novo, com vigência nova (GOV-34): renewed depois
		// de valid_until sem data de fim atualizada seria renovação automática.
		if h.Event == EventRenewed && x.ValidUntil != 0 && h.At > x.ValidUntil {
			add(fmt.Sprintf("history[%d]: renewed depois de valid_until sem vigência nova", i))
		}
	}
	return out
}

// Expired decide o vencimento de GOV-34: a exceção vale até `valid_until`, e
// renovar é gravar um `valid_until` novo — renovação automática não existe.
func Expired(x Exception, now Instant) bool {
	return x.ValidUntil != 0 && now > x.ValidUntil
}

// Closed: revoked ou converged encerram a exceção. Ela deixa de autorizar e de
// vencer, e fica no registro como o histórico que alimenta GOV-36.
func Closed(x Exception) bool {
	return slices.ContainsFunc(x.History, func(h HistoryEntry) bool {
		return h.Event == EventRevoked || h.Event == EventConverged
	})
}

func expiryDiagnostics(x Exception, now Instant) []rule.Diagnostic {
	if Closed(x) || !Expired(x, now) {
		return nil
	}
	return []rule.Diagnostic{{
		Code:   rule.CodeX006,
		Target: x.ID,
		Detail: "exceção vencida sem renovação posterior: deixa de autorizar e a unidade fica não conforme",
	}}
}
