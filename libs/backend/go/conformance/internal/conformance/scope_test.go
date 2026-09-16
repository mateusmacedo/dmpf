package conformance_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

// O `id` só é único dentro de um manifesto: casar a exceção só por ele faz a do
// mod-a autorizar o mod-b, e ela deixa de nomear o par que autoriza.
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

// Package fora de todo `include` já reprova em U001: como DESTINO, tratá-lo
// como externo trocaria a causa real por um sintoma inventado.
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

// Um ponto de parada só (passo 2): acumular deixa o mesmo CI mostrar cobertura
// e dependência de uma vez, em vez de uma rodada por classe.
func TestCoberturaNaoEncerraAntesDasArestas(t *testing.T) {
	rel := rodar(t, "naoclassificado", "exemplo.test/nc-a")
	if rel.PhaseHalted != "" {
		t.Errorf("U001 encerrou a verificação em %q; só o passo 2 encerra", rel.PhaseHalted)
	}
}

// O `-e` é transporte estruturado de erro, não aceitação de universo parcial.
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
