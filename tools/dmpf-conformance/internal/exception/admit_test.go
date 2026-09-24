package exception_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

const (
	dia   = exception.Instant(24 * 60 * 60 * 1_000_000_000)
	agora = exception.Instant(1_757_000_000_000_000_000)
)

// Pedido que satisfaz GOV-30 e GOV-33 inteiros. Cada vetor negativo parte
// daqui e estraga UM item: sem a base, um teste passaria por acidente, porque
// qualquer exceção malformada emite X001 por outro motivo.
func admissivel() exception.Exception {
	return exception.Exception{
		ID: "X-pgx-no-provider",
		Object: exception.Object{
			Kind:            exception.KindExternalDependency,
			Unit:            "kernel/provider-postgres",
			Identity:        "github.com/jackc/pgx/v5",
			PresentKind:     true,
			PresentUnit:     true,
			PresentIdentity: true,
		},
		ADR:           "ADR-035",
		Owner:         "team:plataforma",
		Justification: "o driver não tem substituto puro para a fronteira de UoW",
		Convergence: exception.Convergence{
			Kind:        exception.ConvergencePlan,
			Deadline:    agora + 90*dia,
			Condition:   "quando o kernel expuser a porta de pool",
			PresentKind: true,
		},
		ValidFrom:          agora - dia,
		ValidUntil:         agora + 90*dia,
		ReviewBy:           agora + 60*dia,
		History:            []exception.HistoryEntry{{Event: exception.EventGranted, At: agora - dia, By: "team:arquitetura"}},
		PresentID:          true,
		PresentADR:         true,
		PresentOwner:       true,
		PresentConvergence: true,
		PresentValidFrom:   true,
		PresentValidUntil:  true,
		PresentReviewBy:    true,
		PresentHistory:     true,
	}
}

func codigos(ds []rule.Diagnostic) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		if !slices.Contains(out, string(d.Code)) {
			out = append(out, string(d.Code))
		}
	}
	slices.Sort(out)
	return out
}

func exigeCodigo(t *testing.T, ds []rule.Diagnostic, querido rule.Code) {
	t.Helper()
	for _, d := range ds {
		if d.Code == querido {
			return
		}
	}
	t.Fatalf("esperado %s, veio %v", querido, codigos(ds))
}

func TestPedidoCompletoEAdmitido(t *testing.T) {
	if ds := exception.Admit(admissivel(), agora); len(ds) != 0 {
		t.Fatalf("pedido completo recusado por %v", codigos(ds))
	}
}

func TestX001PorItemFaltanteDeGOV30(t *testing.T) {
	casos := map[string]func(*exception.Exception){
		"sem adr":             func(x *exception.Exception) { x.ADR = "" },
		"adr fora do formato": func(x *exception.Exception) { x.ADR = "ADR-35" },
		"sem owner":           func(x *exception.Exception) { x.Owner = "" },
		"owner sem equipe":    func(x *exception.Exception) { x.Owner = "mateus" },
		"sem justificativa":   func(x *exception.Exception) { x.Justification = "   " },
		"sem convergência": func(x *exception.Exception) {
			x.Convergence = exception.Convergence{}
			x.PresentConvergence = false
		},
		"plano sem prazo":    func(x *exception.Exception) { x.Convergence.Deadline = 0 },
		"plano sem condição": func(x *exception.Exception) { x.Convergence.Condition = "" },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			x := admissivel()
			estraga(&x)
			exigeCodigo(t, exception.Admit(x, agora), rule.CodeX001)
		})
	}
}

// GOV-33: o id é a identidade da dívida no tempo, e precisa ser legível.
func TestX001QuandoIDNaoENominal(t *testing.T) {
	for _, id := range []string{"", "pgx", "X-", "X-PGX", "X-pgx_v5", "1234"} {
		t.Run(id, func(t *testing.T) {
			x := admissivel()
			x.ID = id
			exigeCodigo(t, exception.Admit(x, agora), rule.CodeX001)
		})
	}
}

// O ramo de revisão substitui o prazo por aprovação dual; uma só autoridade
// deixaria a equipe aprovar a própria dívida.
func TestX001QuandoRevisaoNaoTemAprovacaoDual(t *testing.T) {
	casos := map[string][]exception.Approver{
		"sem nenhuma":      nil,
		"só arquitetura":   {exception.ApproverArchitecture},
		"só plataforma":    {exception.ApproverPlatform},
		"outra autoridade": {exception.ApproverArchitecture, "financeiro"},
	}
	for nome, aprovadores := range casos {
		t.Run(nome, func(t *testing.T) {
			x := admissivel()
			x.Convergence = exception.Convergence{
				Kind:                exception.ConvergenceReview,
				ReviewBy:            agora + 30*dia,
				ApprovedBy:          aprovadores,
				ReplanningCondition: "se o driver ganhar porta pura",
				PresentKind:         true,
			}
			exigeCodigo(t, exception.Admit(x, agora), rule.CodeX001)
		})
	}
}

func TestRevisaoComAprovacaoDualEAdmitida(t *testing.T) {
	x := admissivel()
	x.Convergence = exception.Convergence{
		Kind:                exception.ConvergenceReview,
		ReviewBy:            agora + 30*dia,
		ApprovedBy:          []exception.Approver{exception.ApproverArchitecture, exception.ApproverPlatform},
		ReplanningCondition: "se o driver ganhar porta pura",
		PresentKind:         true,
	}
	if ds := exception.Admit(x, agora); len(ds) != 0 {
		t.Fatalf("revisão com aprovação dual recusada por %v", codigos(ds))
	}
}

func TestX002QuandoClasseForaDeE1aE3(t *testing.T) {
	for _, k := range []exception.Kind{"", "external", "EXTERNAL-DEPENDENCY", "tooling"} {
		t.Run(string(k), func(t *testing.T) {
			x := admissivel()
			x.Object.Kind = k
			exigeCodigo(t, exception.Admit(x, agora), rule.CodeX002)
		})
	}
}

// Um vetor por item. Transcritos À MÃO do catálogo, não derivados da tabela de
// produção: derivá-los faria o teste comparar o dado consigo mesmo.
func TestCatalogoFechadoRecusaUmVetorPorItem(t *testing.T) {
	casos := []struct {
		item     string
		identity string
		code     rule.Code
	}{
		{"N1 constraint P0", "P0-1", rule.CodeX004},
		{"N2 aresta", "kernel/domain->kernel/provider-postgres", rule.CodeX003},
		{"N2 célula", "cell:26", rule.CodeX003},
		{"N3 âncora", "ANC-0007", rule.CodeX004},
		{"N4 gatilho", "T3", rule.CodeX004},
		{"N4 gatilho com prefixo", "autorizacao-T5", rule.CodeX004},
		{"N5 classificação", "block", rule.CodeX004},
		{"N5 bounded context", "bounded_context:kernel", rule.CodeX004},
		{"N6 rito Buf", "tools/buf.sh", rule.CodeX004},
		{"N6 plugin pinado", "protoc-gen-go", rule.CodeX004},
		{"N7 evidência", "evidence_digest", rule.CodeX004},
	}
	for _, c := range casos {
		t.Run(c.item, func(t *testing.T) {
			x := admissivel()
			x.Object.Identity = c.identity
			exigeCodigo(t, exception.Admit(x, agora), c.code)
		})
	}
}

// A política de capability de domain e port é RFC §6.2 e constraint P0: uma
// exceção externa não a reescreve, qualquer que seja a capability pedida.
func TestX003QuandoExcecaoTentaFurarPoliticaDeDomainOuPort(t *testing.T) {
	for _, bloco := range []rule.Block{rule.BlockDomain, rule.BlockPort} {
		for _, cap := range []rule.Capability{
			rule.CapIONetwork,
			rule.CapIOClock,
			rule.CapObservability,
			"",
		} {
			t.Run(string(bloco)+"/"+string(cap), func(t *testing.T) {
				x := admissivel()
				s := exception.Subject{Block: bloco, Capability: cap}
				exigeCodigo(t, exception.AdmitIn(x, exception.RegistryManifest, s, agora), rule.CodeX003)
			})
		}
	}
}

// A exceção externa continua servindo ao que existe para servir: autorizar um
// import nominal cuja capability o bloco já admite.
func TestPedidoPureEmDomainEAdmitido(t *testing.T) {
	for _, bloco := range []rule.Block{rule.BlockDomain, rule.BlockPort} {
		t.Run(string(bloco), func(t *testing.T) {
			x := admissivel()
			s := exception.Subject{Block: bloco, Capability: rule.CapPure}
			if ds := exception.AdmitIn(x, exception.RegistryManifest, s, agora); len(ds) != 0 {
				t.Fatalf("pedido pure em %s recusado por %v", bloco, codigos(ds))
			}
		})
	}
}

// Fora de domain e port a política de bloco não é P0: application admite
// observability, e provider e app são permissivos.
func TestX003NaoAlcancaBlocosForaDeDomainEPort(t *testing.T) {
	for _, bloco := range []rule.Block{rule.BlockApplication, rule.BlockProvider, rule.BlockApp, rule.BlockContract} {
		t.Run(string(bloco), func(t *testing.T) {
			x := admissivel()
			s := exception.Subject{Block: bloco, Capability: rule.CapIONetwork}
			if ds := exception.AdmitIn(x, exception.RegistryManifest, s, agora); len(ds) != 0 {
				t.Fatalf("pedido em %s recusado por %v", bloco, codigos(ds))
			}
		})
	}
}

// Sem sujeito resolvido, N1 decide só o que a identidade revela: afirmar
// violação de política sem saber o bloco seria reprovar por suposição.
func TestSemSujeitoResolvidoAMetadeDeBlocoNaoDecide(t *testing.T) {
	x := admissivel()
	if ds := exception.Admit(x, agora); len(ds) != 0 {
		t.Fatalf("pedido sem sujeito resolvido recusado por %v", codigos(ds))
	}
}

// A metade de bloco vale só para E1: combinação do BOM e instrumento de
// governança não falam de import, e o par (bloco, capability) não os alcança.
func TestX003DeBlocoNaoAlcancaE2NemE3(t *testing.T) {
	for _, k := range []exception.Kind{exception.KindBOMCombination, exception.KindGovernanceInstrument} {
		t.Run(string(k), func(t *testing.T) {
			x := admissivel()
			x.Object.Kind = k
			s := exception.Subject{Block: rule.BlockDomain, Capability: rule.CapIONetwork}
			for _, d := range exception.AdmitIn(x, exception.RegistryBOM, s, agora) {
				if d.Code == rule.CodeX003 {
					t.Fatalf("X003 de bloco emitido para %s: %s", k, d.Detail)
				}
			}
		})
	}
}

// O catálogo decide por REGRA sobre o objeto: um pedido legítimo cujo nome
// apenas lembra um item reservado continua admissível.
func TestCatalogoNaoRecusaIdentidadeApenasParecida(t *testing.T) {
	for _, id := range []string{
		"github.com/jackc/pgx/v5",
		"go.opentelemetry.io/otel",
		"T7",
		"blocklist/parser",
	} {
		t.Run(id, func(t *testing.T) {
			x := admissivel()
			x.Object.Identity = id
			if ds := exception.Admit(x, agora); len(ds) != 0 {
				t.Fatalf("identidade legítima %q recusada por %v", id, codigos(ds))
			}
		})
	}
}

func TestX005QuandoCicloDeVidaEInconsistente(t *testing.T) {
	casos := map[string]func(*exception.Exception){
		"valid_from depois de valid_until": func(x *exception.Exception) { x.ValidFrom = x.ValidUntil + dia },
		"review_by depois de valid_until":  func(x *exception.Exception) { x.ReviewBy = x.ValidUntil + dia },
		"history vazio":                    func(x *exception.Exception) { x.History = nil },
		"primeiro evento não é granted": func(x *exception.Exception) {
			x.History = []exception.HistoryEntry{{Event: exception.EventRenewed, At: agora, By: "team:plataforma", Reason: "ainda sem porta"}}
		},
		"renewed sem reason": func(x *exception.Exception) {
			x.History = append(x.History, exception.HistoryEntry{Event: exception.EventRenewed, At: agora, By: "team:plataforma"})
		},
		"evento fora do conjunto": func(x *exception.Exception) {
			x.History = append(x.History, exception.HistoryEntry{Event: "prorrogado", At: agora, By: "team:plataforma"})
		},
		"evento sem autor": func(x *exception.Exception) {
			x.History = append(x.History, exception.HistoryEntry{Event: exception.EventConverged, At: agora})
		},
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			x := admissivel()
			estraga(&x)
			exigeCodigo(t, exception.Admit(x, agora), rule.CodeX005)
		})
	}
}

func TestX006QuandoExcecaoVenceSemRenovacao(t *testing.T) {
	x := admissivel()
	exigeCodigo(t, exception.Admit(x, x.ValidUntil+dia), rule.CodeX006)
}

// GOV-34: renovação automática não existe. Um renewed sem valid_until novo não
// reabre a exceção vencida.
func TestRenovacaoSemVigenciaNovaNaoReabre(t *testing.T) {
	x := admissivel()
	depois := x.ValidUntil + dia
	x.History = append(x.History, exception.HistoryEntry{
		Event: exception.EventRenewed, At: depois, By: "team:plataforma", Reason: "porta pura ainda não existe",
	})
	ds := exception.Admit(x, depois+dia)
	exigeCodigo(t, ds, rule.CodeX005)
	exigeCodigo(t, ds, rule.CodeX006)
}

func TestRenovacaoComVigenciaNovaEAdmitida(t *testing.T) {
	x := admissivel()
	depois := x.ValidUntil + dia
	x.ValidUntil = depois + 90*dia
	x.History = append(x.History, exception.HistoryEntry{
		Event: exception.EventRenewed, At: depois, By: "team:plataforma", Reason: "porta pura ainda não existe",
	})
	if ds := exception.Admit(x, depois+dia); len(ds) != 0 {
		t.Fatalf("renovação com valid_until novo recusada por %v", codigos(ds))
	}
}

func TestExcecaoEncerradaNaoVence(t *testing.T) {
	for _, evento := range []exception.Event{exception.EventConverged, exception.EventRevoked} {
		t.Run(string(evento), func(t *testing.T) {
			x := admissivel()
			x.History = append(x.History, exception.HistoryEntry{Event: evento, At: agora, By: "team:plataforma"})
			if !exception.Closed(x) {
				t.Fatalf("%s não encerrou a exceção", evento)
			}
			for _, d := range exception.Admit(x, x.ValidUntil+dia) {
				if d.Code == rule.CodeX006 {
					t.Fatalf("exceção %s recebeu X006: %s", evento, d.Detail)
				}
			}
		})
	}
}

func TestX001SemParNominalVigenciaOuRevisao(t *testing.T) {
	casos := map[string]func(*exception.Exception){
		"sem object.unit":     func(x *exception.Exception) { x.Object.Unit = "" },
		"sem object.identity": func(x *exception.Exception) { x.Object.Identity = " " },
		"sem valid_until":     func(x *exception.Exception) { x.ValidUntil, x.PresentValidUntil = 0, false },
		"sem review_by":       func(x *exception.Exception) { x.ReviewBy, x.PresentReviewBy = 0, false },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			x := admissivel()
			estraga(&x)
			exigeCodigo(t, exception.Admit(x, agora), rule.CodeX001)
		})
	}
}

func TestX005QuandoDataNaoParseia(t *testing.T) {
	x := admissivel()
	x.ValidUntil, x.InvalidDates = 0, []string{"valid_until"}
	ds := exception.Admit(x, agora)
	exigeCodigo(t, ds, rule.CodeX005)
	for _, d := range ds {
		if d.Code == rule.CodeX001 {
			t.Fatalf("data inválida lida como ausente: %s", d.Detail)
		}
	}
}

func TestRenovacaoAnteriorAoVencimentoNaoReabre(t *testing.T) {
	x := admissivel()
	x.History = append(x.History, exception.HistoryEntry{
		Event: exception.EventRenewed, At: x.ValidUntil - dia, By: "team:plataforma", Reason: "revisão antecipada",
	})
	exigeCodigo(t, exception.Admit(x, x.ValidUntil+dia), rule.CodeX006)
}

func TestX007QuandoAExcecaoEstaNoRegistroErrado(t *testing.T) {
	t.Run("E2 no manifesto", func(t *testing.T) {
		x := admissivel()
		x.Object.Kind = exception.KindBOMCombination
		exigeCodigo(t, exception.AdmitIn(x, exception.RegistryManifest, exception.Subject{}, agora), rule.CodeX007)
	})
	t.Run("E3 no manifesto", func(t *testing.T) {
		x := admissivel()
		x.Object.Kind = exception.KindGovernanceInstrument
		exigeCodigo(t, exception.AdmitIn(x, exception.RegistryManifest, exception.Subject{}, agora), rule.CodeX007)
	})
	t.Run("E1 no BOM", func(t *testing.T) {
		x := admissivel()
		exigeCodigo(t, exception.AdmitIn(x, exception.RegistryBOM, exception.Subject{}, agora), rule.CodeX007)
	})
	t.Run("E2 no BOM é admitida", func(t *testing.T) {
		x := admissivel()
		x.Object.Kind = exception.KindBOMCombination
		if ds := exception.AdmitIn(x, exception.RegistryBOM, exception.Subject{}, agora); len(ds) != 0 {
			t.Fatalf("E2 no BOM recusada por %v", codigos(ds))
		}
	})
}

// Um pedido ruim em vários eixos precisa mostrar TODOS: corrigir um item por
// rodada é o que faz uma exceção malformada sobreviver a várias revisões.
func TestTodosOsMotivosSaoReportadosDeUmaVez(t *testing.T) {
	x := admissivel()
	x.ADR = ""
	x.Object.Kind = "tooling"
	x.Object.Identity = "P0-1"
	x.History = nil

	got := codigos(exception.Admit(x, agora))
	for _, querido := range []string{"DMPF-X001", "DMPF-X002", "DMPF-X004", "DMPF-X005"} {
		if !slices.Contains(got, querido) {
			t.Errorf("%s ausente de %v", querido, got)
		}
	}
}

func TestTodoCodigoDeGovernancaTemSecaoRastreavel(t *testing.T) {
	normativos := []string{
		"DMPF-X001", "DMPF-X002", "DMPF-X003",
		"DMPF-X004", "DMPF-X005", "DMPF-X006", "DMPF-X007",
	}
	specs := rule.GovernanceCodeSpecs()
	got := make([]string, 0, len(specs))
	for _, s := range specs {
		got = append(got, string(s.Code))
		if s.Section == "" {
			t.Errorf("código %s sem seção normativa rastreável", s.Code)
		}
	}
	if !slices.Equal(got, normativos) {
		t.Fatalf("emitidos\n  %v\nGOV-30..36 fixa\n  %v", got, normativos)
	}
}
