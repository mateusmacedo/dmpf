package exception

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

// Registry é onde a exceção foi declarada. GOV-35 amarra classe a registro: E1
// vive no manifesto da unidade, E2 e E3 no BOM, e a troca é X007.
type Registry string

const (
	RegistryManifest Registry = "manifest"
	RegistryBOM      Registry = "bom"
)

// Subject é a unidade e a dependência que a exceção pede, já classificadas.
// Block vazio significa não resolvido, e então N1 decide só pela identidade —
// Capability vazia, não: dentro de um bloco conhecido ela é dependência que o
// verificador não conseguiu classificar, e não classificar não autoriza.
type Subject struct {
	Block      rule.Block
	Capability rule.Capability
}

// Admit devolve os diagnósticos que RECUSAM o pedido; vazio significa admitida.
//
// Devolver tudo de uma vez, e não o primeiro achado, é deliberado: quem escreve
// a exceção corrige uma rodada só, em vez de descobrir os itens um por vez.
func Admit(x Exception, now Instant) []rule.Diagnostic {
	return AdmitIn(x, RegistryManifest, Subject{}, now)
}

// AdmitIn é Admit ciente do registro que comporta a exceção (X007) e do sujeito
// que ela pede (a metade de N1 que depende de bloco e capability).
func AdmitIn(x Exception, reg Registry, s Subject, now Instant) []rule.Diagnostic {
	var out []rule.Diagnostic

	out = append(out, requiredDiagnostics(x)...)
	out = append(out, kindDiagnostics(x)...)
	out = append(out, deniedBy(x.Object)...)
	out = append(out, blockPolicyDiagnostics(x, s)...)
	out = append(out, lifecycleDiagnostics(x)...)
	out = append(out, expiryDiagnostics(x, now)...)
	out = append(out, registryDiagnostics(x, reg)...)

	for i := range out {
		if out[i].CanonicalKey == "" {
			out[i].CanonicalKey = x.Object.Unit
		}
	}
	rule.SortDiagnostics(out)
	return out
}

// GOV-30: sem ADR, dono, justificativa e plano de convergência, o pedido não
// descreve dívida com saída — descreve permissão permanente.
func requiredDiagnostics(x Exception) []rule.Diagnostic {
	var out []rule.Diagnostic
	add := func(detail string) {
		out = append(out, rule.Diagnostic{Code: rule.CodeX001, Target: x.ID, Detail: detail})
	}

	if !isNominalID(x.ID) {
		add(fmt.Sprintf("id %q não é nominal: GOV-33 exige X-<minúsculas, dígitos e hífens>", x.ID))
	}
	if !isADRRef(x.ADR) {
		add(fmt.Sprintf("adr %q ausente ou fora do formato ADR-NNN", x.ADR))
	}
	// A equipe é o dono, não a pessoa: quem abriu o pedido pode sair, e a
	// dívida continua precisando de alguém que responda por ela.
	if !strings.HasPrefix(x.Owner, "team:") || len(x.Owner) <= len("team:") {
		add(fmt.Sprintf("owner %q não nomeia uma equipe (team:<nome>)", x.Owner))
	}
	if strings.TrimSpace(x.Justification) == "" {
		add("justification vazia")
	}
	if strings.TrimSpace(x.Object.Unit) == "" || strings.TrimSpace(x.Object.Identity) == "" {
		add("object sem unit ou identity: a exceção nomeia o par (unidade, objeto), GOV-33")
	}
	if x.ValidUntil == 0 && !slices.Contains(x.InvalidDates, "valid_until") {
		add("valid_until ausente: sem data de fim a exceção é inválida (GOV-34)")
	}
	if x.ReviewBy == 0 && !slices.Contains(x.InvalidDates, "review_by") {
		add("review_by ausente: a revisão tem data obrigatória (GOV-34)")
	}
	out = append(out, convergenceDiagnostics(x)...)
	return out
}

func convergenceDiagnostics(x Exception) []rule.Diagnostic {
	add := func(detail string) rule.Diagnostic {
		return rule.Diagnostic{Code: rule.CodeX001, Target: x.ID, Detail: detail}
	}
	if !x.PresentConvergence {
		return []rule.Diagnostic{add("convergence ausente: exceção sem saída declarada é permissão permanente")}
	}

	c := x.Convergence
	switch c.Kind {
	case ConvergencePlan:
		var out []rule.Diagnostic
		if c.Deadline == 0 {
			out = append(out, add("convergence plan sem deadline"))
		}
		if strings.TrimSpace(c.Condition) == "" {
			out = append(out, add("convergence plan sem condition"))
		}
		return out
	case ConvergenceReview:
		var out []rule.Diagnostic
		if c.ReviewBy == 0 {
			out = append(out, add("convergence review sem review_by"))
		}
		// Aprovação dual: uma só autoridade decidindo sobre a própria dívida é
		// o que o ramo de revisão existe para impedir.
		for _, required := range []Approver{ApproverArchitecture, ApproverPlatform} {
			if !slices.Contains(c.ApprovedBy, required) {
				out = append(out, add(fmt.Sprintf("convergence review sem aprovação de %s", required)))
			}
		}
		if strings.TrimSpace(c.ReplanningCondition) == "" {
			out = append(out, add("convergence review sem replanning_condition"))
		}
		return out
	default:
		return []rule.Diagnostic{add(fmt.Sprintf("convergence.kind %q fora de plan|review", c.Kind))}
	}
}

func kindDiagnostics(x Exception) []rule.Diagnostic {
	if IsKind(x.Object.Kind) {
		return nil
	}
	return []rule.Diagnostic{{
		Code:   rule.CodeX002,
		Target: x.ID,
		Detail: fmt.Sprintf("object.kind %q fora das classes E1–E3", x.Object.Kind),
	}}
}

func registryDiagnostics(x Exception, reg Registry) []rule.Diagnostic {
	if !IsKind(x.Object.Kind) {
		return nil
	}
	want := RegistryBOM
	if x.Object.Kind == KindExternalDependency {
		want = RegistryManifest
	}
	if want == reg {
		return nil
	}
	return []rule.Diagnostic{{
		Code:   rule.CodeX007,
		Target: x.ID,
		Detail: fmt.Sprintf("exceção %s declarada no %s; GOV-35 a comporta no %s", x.Object.Kind, reg, want),
	}}
}

// GOV-33 quer identidade legível e estável, não hash nem número de chamado.
func isNominalID(id string) bool {
	if !strings.HasPrefix(id, "X-") || len(id) <= len("X-") {
		return false
	}
	for _, r := range id[len("X-"):] {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '-' {
			return false
		}
	}
	return true
}

func isADRRef(ref string) bool {
	if len(ref) != len("ADR-000") || !strings.HasPrefix(ref, "ADR-") {
		return false
	}
	for _, r := range ref[len("ADR-"):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
