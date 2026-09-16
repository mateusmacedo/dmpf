package conformance_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/port"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

// storeFalso devolve o baseline de antes e o de agora sem tocar o disco.
type storeFalso struct {
	agora   baseline.Document
	antes   baseline.Document
	tinha   bool
	commits []baseline.Commit
	erroEm  error
}

func (s storeFalso) Baseline() (baseline.Document, bool, error) { return s.agora, true, nil }
func (s storeFalso) BaselineEm(string) (baseline.Document, bool, error) {
	return s.antes, s.tinha, s.erroEm
}
func (s storeFalso) CommitsQueTocaram(string) ([]baseline.Commit, error) { return s.commits, nil }

var _ port.BaselineStore = storeFalso{}

// grafoDeUmaUnidade é o menor universo possível: um package classificado, sem
// aresta nenhuma. Isola a conferência de classificação do resto da verificação.
type grafoDeUmaUnidade struct{}

func (grafoDeUmaUnidade) Packages() ([]rule.Package, error) {
	return []rule.Package{{CanonicalKey: "m/p", Module: "m"}}, nil
}
func (grafoDeUmaUnidade) Edges() ([]port.Edge, error) { return nil, nil }

type manifestoDe struct{ block, bc string }

func (m manifestoDe) Documents() ([]manifest.Document, error) {
	return []manifest.Document{{
		Path: "m/dmpf-units.json", Module: "m", Schema: manifest.SchemaID,
		Units: []manifest.Unit{{
			ID: "u", Block: m.block, BoundedContext: m.bc, Include: []string{"m/p"},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}},
	}}, nil
}

func entradaDe(block, bc string) baseline.Document {
	e := []baseline.Entry{{Unit: "u", Module: "m", Block: block, BoundedContext: bc, Membership: []string{"m/p"}}}
	return baseline.Document{Schema: baseline.SchemaID, Digest: baseline.Digest(e), Entries: e}
}

func verificar(t *testing.T, m manifestoDe, s storeFalso, base string) conformance.Report {
	t.Helper()
	rel, err := conformance.Check(conformance.Input{
		Modules:   []rule.Module{{Path: "m", HasManifest: true, HasProduction: true}},
		Manifests: m,
		Graph:     grafoDeUmaUnidade{},
		Baseline:  s,
		Base:      base,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return rel
}

func temCodigo(rel conformance.Report, c rule.Code) bool {
	return slices.ContainsFunc(rel.Diagnostics, func(d rule.Diagnostic) bool { return d.Code == c })
}

// TestReclassificacaoCoordenadaEPega é o caso que a exigência de aval existe
// para cobrir, e o mais fácil de deixar passar.
//
// Confrontar o baseline com o manifesto NO MESMO PONTO só acha quem esqueceu de
// atualizar um dos dois. Quem altera os dois de forma coerente deixa a
// comparação verde — e libera imports antes proibidos sem que ninguém veja.
// A comparação tem de ser entre o de antes e o de agora.
func TestReclassificacaoCoordenadaEPega(t *testing.T) {
	// Manifesto e baseline concordam entre si: nenhuma divergência a reportar.
	store := storeFalso{
		agora: entradaDe("app", "bc"),
		antes: entradaDe("domain", "bc"), tinha: true,
		commits: []baseline.Commit{{
			SHA: "aaa", Arquivos: []string{"tools/dmpf-baseline/units-baseline.json", "m/dmpf-units.json", "m/p/p.go"},
		}},
	}
	rel := verificar(t, manifestoDe{"app", "bc"}, store, "origin/develop")

	if temCodigo(rel, rule.CodeT001) {
		t.Error("acusou divergência: manifesto e baseline concordam entre si")
	}
	if !temCodigo(rel, rule.CodeT002) {
		t.Fatalf("reclassificação coordenada passou sem exigir aval: %v", rel.Diagnostics)
	}
}

// TestReclassificacaoCoordenadaEmCommitProprioPassa é o positivo do par: a
// mesma mudança, isolada do código, é o caminho que a regra prescreve.
func TestReclassificacaoCoordenadaEmCommitProprioPassa(t *testing.T) {
	store := storeFalso{
		agora: entradaDe("app", "bc"),
		antes: entradaDe("domain", "bc"), tinha: true,
		commits: []baseline.Commit{
			{SHA: "aaa", Arquivos: []string{"tools/dmpf-baseline/units-baseline.json", "m/dmpf-units.json"}},
			{SHA: "bbb", Arquivos: []string{"m/p/p.go"}},
		},
	}
	rel := verificar(t, manifestoDe{"app", "bc"}, store, "origin/develop")
	if len(rel.Diagnostics) != 0 || len(rel.NaoVerificado) != 0 {
		t.Fatalf("mudança isolada reprovou: %v %v", rel.Diagnostics, rel.NaoVerificado)
	}
}

// TestSemClassificacaoAnteriorTudoEcriacao: sem baseline no ponto de partida,
// cada unidade que existe agora nasceu no intervalo — e nascer também exige aval.
func TestSemClassificacaoAnteriorTudoEcriacao(t *testing.T) {
	store := storeFalso{
		agora: entradaDe("domain", "bc"),
		tinha: false,
		commits: []baseline.Commit{{
			SHA: "aaa", Arquivos: []string{"tools/dmpf-baseline/units-baseline.json", "m/p/p.go"},
		}},
	}
	rel := verificar(t, manifestoDe{"domain", "bc"}, store, "origin/develop")
	if !temCodigo(rel, rule.CodeT002) {
		t.Fatalf("criação de unidade passou sem exigir aval: %v", rel.Diagnostics)
	}
}

// TestClassificacaoIntactaNaoCobraNada: sem mudança entre antes e agora, não há
// ato a autorizar, e um commit que mistura código com o manifesto é irrelevante.
func TestClassificacaoIntactaNaoCobraNada(t *testing.T) {
	store := storeFalso{
		agora: entradaDe("domain", "bc"),
		antes: entradaDe("domain", "bc"), tinha: true,
		commits: []baseline.Commit{{
			SHA: "aaa", Arquivos: []string{"m/dmpf-units.json", "m/p/p.go"},
		}},
	}
	rel := verificar(t, manifestoDe{"domain", "bc"}, store, "origin/develop")
	if len(rel.Diagnostics) != 0 {
		t.Fatalf("cobrou aval sem mudança de classificação: %v", rel.Diagnostics)
	}
}

// TestBaselineAnteriorIlegivelNaoViraConforme: um clone raso não pode fazer a
// mudança passar por inexistente.
func TestBaselineAnteriorIlegivelNaoViraConforme(t *testing.T) {
	store := storeFalso{
		agora:  entradaDe("app", "bc"),
		erroEm: errors.New("ref inalcançável"),
	}
	rel := verificar(t, manifestoDe{"app", "bc"}, store, "origin/develop")
	if len(rel.NaoVerificado) == 0 {
		t.Fatalf("erro ao ler a classificação anterior virou silêncio: %v", rel)
	}
	if !rel.Reprovado() {
		t.Error("não verificado deveria reprovar")
	}
}

// TestSemBaseNaoAvalia: sem o ponto de partida, não há intervalo a comparar.
func TestSemBaseNaoAvalia(t *testing.T) {
	store := storeFalso{agora: entradaDe("app", "bc")}
	rel := verificar(t, manifestoDe{"app", "bc"}, store, "")
	if len(rel.NaoVerificado) == 0 || !rel.Reprovado() {
		t.Fatalf("ausência de base não reprovou: %v", rel)
	}
}
