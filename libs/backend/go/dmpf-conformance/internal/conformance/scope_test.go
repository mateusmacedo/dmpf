package conformance_test

import (
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// TestExcecaoNaoVazaEntreModulos: `id` de unidade só é único DENTRO de um
// manifesto (RFC §10.1), então dois módulos podem declarar `id: "domain"`
// legitimamente. Se a exceção nominal for casada só pelo `id`, a exceção que o
// mod-a declarou para si autoriza o mod-b — e a exceção deixa de ser nominal,
// virando a política paralela que RFC §6.4 proíbe.
func TestExcecaoNaoVazaEntreModulos(t *testing.T) {
	rel := rodar(t, "vazamento", "exemplo.test/vaz-a", "exemplo.test/vaz-b")

	var aReprovou, bReprovou bool
	for _, d := range rel.Diagnostics {
		if d.Code != rule.CodeE001 || d.Target != "net/http" {
			continue
		}
		switch d.CanonicalKey {
		case "exemplo.test/vaz-a/domain":
			aReprovou = true
		case "exemplo.test/vaz-b/domain":
			bReprovou = true
		}
	}

	if aReprovou {
		t.Error("mod-a reprovou apesar da exceção nominal que ele declarou")
	}
	if !bReprovou {
		t.Fatalf("mod-b passou sem declarar exceção: a exceção do mod-a vazou pelo id homônimo. Diagnósticos: %v", rel.Diagnostics)
	}
}

// TestPackageNaoClassificadoNaoViraDependenciaExterna: um package descoberto
// como produção mas fora de todo `include` já reprova em U001. Ao ser DESTINO
// de uma aresta, ele não pode ser confundido com dependência externa — isso
// emitiria um E001 sobre código do próprio universo e trocaria a causa real
// (falta de declaração) por um sintoma inventado (capability ausente).
func TestPackageNaoClassificadoNaoViraDependenciaExterna(t *testing.T) {
	rel := rodar(t, "naoclassificado", "exemplo.test/nc-a")

	var temU001, temE001 bool
	for _, d := range rel.Diagnostics {
		switch d.Code {
		case rule.CodeU001:
			if d.CanonicalKey == "exemplo.test/nc-a/orfao" {
				temU001 = true
			}
		case rule.CodeE001:
			if d.Target == "exemplo.test/nc-a/orfao" {
				temE001 = true
			}
		}
	}
	if !temU001 {
		t.Errorf("package órfão não reprovou em U001: %v", rel.Diagnostics)
	}
	if temE001 {
		t.Errorf("package do universo tratado como dependência externa: %v", rel.Diagnostics)
	}
}

// TestCoberturaNaoEncerraAntesDasArestas: a spec fixa UM ponto de parada — o
// passo 2, manifesto e U004. U001 acumulando com os diagnósticos de aresta é o
// que faz um mesmo CI mostrar cobertura e dependência de uma vez, em vez de
// exigir uma rodada por classe.
func TestCoberturaNaoEncerraAntesDasArestas(t *testing.T) {
	rel := rodar(t, "naoclassificado", "exemplo.test/nc-a")
	if rel.PhaseHalted != "" {
		t.Errorf("U001 encerrou a verificação em %q; só o passo 2 encerra", rel.PhaseHalted)
	}
}

// TestImportNaoResolvidoEmiteE003: o `-e` do `go list` é transporte
// estruturado de erro, não aceitação de universo parcial. Um import que não
// resolve reprova com E003 e nunca é tratado como ausente (RFC §10.3).
func TestImportNaoResolvidoEmiteE003(t *testing.T) {
	rel := rodar(t, "naoresolvido", "exemplo.test/nr-a")

	var achou bool
	for _, d := range rel.Diagnostics {
		if d.Code == rule.CodeE003 && d.CanonicalKey == "exemplo.test/nr-a/domain" {
			achou = true
			if d.Detail == "" {
				t.Error("E003 sem o motivo devolvido pelo toolchain")
			}
		}
	}
	if !achou {
		t.Fatalf("import inexistente não emitiu E003: %v", rel.Diagnostics)
	}
}
