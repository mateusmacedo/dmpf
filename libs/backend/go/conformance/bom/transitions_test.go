package bom

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

func comEstado(e *Entry, s State) {
	e.State = s
	if s == StateDepreciada {
		e.DeprecatedAt, e.Successor = "2026-09-01", "github.com/jackc/pgx/v6"
	}
}

func TestTransicoesDeBOM07(t *testing.T) {
	casos := []struct {
		antes, depois State
		reprova       bool
	}{
		{StateProposta, StateCandidata, false},
		{StateCandidata, StateCertificada, false},
		{StateCertificada, StateDepreciada, false},
		{StateDepreciada, StateNaoSuportada, false},
		{StateCertificada, StateCandidata, false},
		{StateNaoSuportada, StateProposta, false},
		{StateCandidata, StateRejeitada, false},
		{StateProposta, StateCertificada, true},
		{StateDepreciada, StateCertificada, true},
		{StateRejeitada, StateCandidata, true},
	}
	for _, c := range casos {
		t.Run(string(c.antes)+"→"+string(c.depois), func(t *testing.T) {
			base := documentoValido(t)
			comEstado(driverDe(&base), c.antes)
			atual := documentoValido(t)
			comEstado(driverDe(&atual), c.depois)

			ds := validarEm(t, atual, arquivoValido, &base)
			if got := temCodigo(ds, rule.CodeB003); got != c.reprova {
				t.Fatalf("B003=%v, esperado %v:\n%s", got, c.reprova, listar(ds))
			}
		})
	}
}

func TestEntradaNovaComecaEmPropostaOuCandidata(t *testing.T) {
	casos := []struct {
		estado  State
		reprova bool
	}{
		{StateProposta, false},
		{StateCandidata, false},
		{StateCertificada, true},
	}
	for _, c := range casos {
		t.Run(string(c.estado), func(t *testing.T) {
			base := documentoValido(t)
			base.Sections[2].Entries = nil
			atual := documentoValido(t)
			comEstado(driverDe(&atual), c.estado)

			ds := validarEm(t, atual, arquivoValido, &base)
			if got := temCodigo(ds, rule.CodeB003); got != c.reprova {
				t.Fatalf("B003=%v, esperado %v:\n%s", got, c.reprova, listar(ds))
			}
		})
	}
}

func TestSemBaseNaoAvaliaTransicao(t *testing.T) {
	atual := documentoValido(t)
	comEstado(driverDe(&atual), StateCertificada)
	if ds := validarEm(t, atual, arquivoValido, nil); temCodigo(ds, rule.CodeB003) {
		t.Fatalf("B003 sem --base:\n%s", listar(ds))
	}
}

func TestRemocaoDeEntradaSoParaRejeitada(t *testing.T) {
	casos := []struct {
		estado  State
		reprova bool
	}{
		{StateCandidata, true},
		{StateRejeitada, false},
	}
	for _, c := range casos {
		t.Run(string(c.estado), func(t *testing.T) {
			base := documentoValido(t)
			comEstado(driverDe(&base), c.estado)
			atual := documentoValido(t)
			atual.Sections[2].Entries, atual.Sections[2].Reason = nil, "driver retirado"

			ds := validarEm(t, atual, arquivoValido, &base)
			if got := temCodigo(ds, rule.CodeB003); got != c.reprova {
				t.Fatalf("B003=%v, esperado %v:\n%s", got, c.reprova, listar(ds))
			}
		})
	}
}

func TestEntradaDepreciadaPodeMudarDeSecao(t *testing.T) {
	base := documentoValido(t)
	comEstado(driverDe(&base), StateCertificada)
	atual := documentoValido(t)
	movida := *driverDe(&atual)
	comEstado(&movida, StateDepreciada)
	atual.Sections[2].Entries, atual.Sections[2].Reason = nil, "driver depreciado"
	atual.Sections[4].Entries = []Entry{movida}

	if ds := validarEm(t, atual, arquivoValido, &base); temCodigo(ds, rule.CodeB003) {
		t.Fatalf("mudança de seção lida como remoção ou entrada nova:\n%s", listar(ds))
	}
}
