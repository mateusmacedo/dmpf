package rule_test

import (
	"testing"

	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-conformance/internal/rule"
)

func TestEntradaDegeneradaNaoEntraEmPanicoNemAprova(t *testing.T) {
	casos := []struct {
		nome  string
		units []rule.Unit
		pkgs  []rule.Package
		mods  []rule.Module
	}{
		{"tudo nil", nil, nil, nil},
		{"tudo vazio", []rule.Unit{}, []rule.Package{}, []rule.Module{}},
		{"unidade sem include", []rule.Unit{{ID: "a", Block: rule.BlockDomain, Module: "m"}}, []rule.Package{{CanonicalKey: "m/p", Module: "m"}}, nil},
		{"unidade com include nil", []rule.Unit{{ID: "a", Block: rule.BlockDomain, Module: "m", Include: nil}}, nil, nil},
		{"package com canonical_key vazia", nil, []rule.Package{{CanonicalKey: "", Module: "m"}}, nil},
		{"unidade com Module vazio", []rule.Unit{{ID: "a", Block: rule.BlockDomain, Include: []string{"m/p"}}}, []rule.Package{{CanonicalKey: "m/p", Module: "m"}}, nil},
		{"bloco inválido", []rule.Unit{{ID: "a", Block: rule.Block("core"), Module: "m", Include: []string{"m/p"}}}, []rule.Package{{CanonicalKey: "m/p", Module: "m"}}, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			u, ds := rule.BuildUniverse(c.units, c.pkgs, c.mods)
			if u == nil {
				t.Fatal("universo nil")
			}
			_ = u.Membership()
			_, _ = u.Lookup("qualquer")
			_, _ = u.Endpoint("qualquer")
			t.Logf("diagnósticos: %v", ds)
		})
	}
}

func TestBlocoDesconhecidoNuncaPermiteAresta(t *testing.T) {
	desconhecidos := []rule.Block{"", "core", "DOMAIN", "domain ", "infra"}
	for _, b := range desconhecidos {
		for _, conhecido := range rule.Blocks() {
			if rule.AllowedByMatrix(b, conhecido) {
				t.Errorf("origem desconhecida %q -> %s permitida", b, conhecido)
			}
			if rule.AllowedByMatrix(conhecido, b) {
				t.Errorf("%s -> destino desconhecido %q permitida", conhecido, b)
			}
		}
		if d := rule.Decide(rule.Endpoint{Block: b, BoundedContext: "a"}, rule.Endpoint{Block: b, BoundedContext: "a"}); d.Allowed() {
			t.Errorf("aresta entre blocos desconhecidos %q permitida", b)
		}
	}
}

// Rotular o código de forma conveniente é o caminho mais barato de burla, e a
// norma o declara violação mesmo quando nenhuma ferramenta o detecta.
func TestRotulagemOportunistaReprova(t *testing.T) {
	for _, v := range []string{"Domain", "DOMAIN", " domain", "domain ", "dom ain", "contract\t"} {
		u := manifest.Unit{
			ID: "a", Block: v, BoundedContext: "a", Include: []string{"m/p"},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}
		ds := manifest.Validate(manifest.Document{Path: "m/dmpf-units.json", Module: "m", Schema: manifest.SchemaID, Units: []manifest.Unit{u}})
		if len(ds) == 0 {
			t.Errorf("block %q aceito sem diagnóstico", v)
		}
	}
}

func TestSuperficiePublicaEmDomainEmiteExatamenteM002(t *testing.T) {
	u := manifest.Unit{
		ID: "a", Block: "domain", BoundedContext: "a", Include: []string{"m/p"},
		PublicIntegrationSurface: true,
		PresentID:                true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
	}
	ds := manifest.Validate(manifest.Document{Path: "m/u.json", Module: "m", Schema: manifest.SchemaID, Units: []manifest.Unit{u}})
	if len(ds) != 1 || ds[0].Code != rule.CodeM002 {
		t.Errorf("esperado exatamente M002, got %v", ds)
	}
}

// O manifesto é escrito por quem abre o PR: um `id` com quebra de linha forja
// uma linha inteira no log e o gate passa a mentir para quem o lê.
//
// Os três primeiros vetores atravessam os caminhos SEM `%q`; o do manifesto
// entra como defesa em profundidade.
func TestManifestoNaoForjaLinhaDeDiagnostico(t *testing.T) {
	const forja = "a\nDMPF-D001: forjado -> tudo ok [RFC §7.3]"

	t.Run("bounded_context na aresta (D002)", func(t *testing.T) {
		exigeUmaLinha(t, rule.DiagnoseEdge(
			rule.Endpoint{CanonicalKey: "m/a", Block: rule.BlockDomain, BoundedContext: forja},
			rule.Endpoint{CanonicalKey: "m/b", Block: rule.BlockDomain, BoundedContext: "outro"},
			"m/a/x.go",
		))
	})

	t.Run("id da unidade na sobreposição (U002)", func(t *testing.T) {
		units := []rule.Unit{
			{ID: forja, Block: rule.BlockDomain, BoundedContext: "a", Include: []string{"m/p"}, Module: "m"},
			{ID: "outra", Block: rule.BlockApp, BoundedContext: "a", Include: []string{"m/p"}, Module: "m"},
		}
		_, ds := rule.BuildUniverse(units, []rule.Package{{CanonicalKey: "m/p", Module: "m"}}, nil)
		exigeUmaLinha(t, ds)
	})

	t.Run("canonical_key não coberta (U001)", func(t *testing.T) {
		_, ds := rule.BuildUniverse(nil, []rule.Package{{CanonicalKey: forja, Module: "m"}}, nil)
		exigeUmaLinha(t, ds)
	})

	t.Run("id no manifesto (defesa em profundidade)", func(t *testing.T) {
		u := manifest.Unit{
			ID: forja, Block: "core", Include: []string{"m/p"},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}
		exigeUmaLinha(t, manifest.Validate(manifest.Document{
			Path: "m/u.json", Module: "m", Schema: manifest.SchemaID, Units: []manifest.Unit{u},
		}))
	})
}

func exigeUmaLinha(t *testing.T, ds []rule.Diagnostic) {
	t.Helper()
	if len(ds) == 0 {
		t.Fatal("entrada inválida não gerou diagnóstico: o vetor perdeu a força")
	}
	for _, d := range ds {
		if strings.ContainsAny(d.String(), "\n\r") {
			t.Errorf("diagnóstico com quebra de linha embutida, permite forjar linha no log:\n%s", d.String())
		}
	}
}
