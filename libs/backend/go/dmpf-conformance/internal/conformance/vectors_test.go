// Vetores da regra alimentados por um port.GraphSource em memória. Vivem na
// camada de services, não no domínio: um teste que precisa de duplo de porta
// é teste de services (FND-09 PIR-17), e o domínio `rule` continua testado
// por valor em internal/rule.
package conformance_test

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/fitness"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/port"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

type memGraph struct {
	packages []rule.Package
	edges    []port.Edge
}

func (g memGraph) Packages() ([]rule.Package, error) { return g.packages, nil }

func (g memGraph) Edges() ([]port.Edge, error) { return g.edges, nil }

var _ port.GraphSource = memGraph{}

// Harness dos vetores: a mesma decisão que o package público `fitness`
// exporta para a suíte (FIT-02), para que os vetores provem o pipeline
// publicado e não uma cópia dele.
func decideGrafo(t *testing.T, units []rule.Unit, mods []rule.Module, g port.GraphSource) []rule.Diagnostic {
	t.Helper()
	return fitness.Diagnose(units, mods, g)
}

// Conjunto EXATO: "contém" deixaria o vetor verde com diagnóstico espúrio
// junto, e um a mais é tão errado quanto um a menos.
func exigeCodigos(t *testing.T, ds []rule.Diagnostic, want ...rule.Code) {
	t.Helper()
	got := codigos(ds)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("códigos emitidos %v, esperado exatamente %v", got, want)
	}
}

// Põe a mensagem sob contrato, não só o código.
func exigeDiagnostico(t *testing.T, ds []rule.Diagnostic, want rule.Diagnostic) {
	t.Helper()
	for _, d := range ds {
		if d.Code == want.Code && d.CanonicalKey == want.CanonicalKey && d.Target == want.Target {
			if want.SourceFile != "" && d.SourceFile != want.SourceFile {
				t.Errorf("SourceFile %q, esperado %q", d.SourceFile, want.SourceFile)
			}
			return
		}
	}
	t.Fatalf("diagnóstico %s %s -> %s ausente em %v", want.Code, want.CanonicalKey, want.Target, ds)
}

func exigeLimpo(t *testing.T, ds []rule.Diagnostic) {
	t.Helper()
	if len(ds) != 0 {
		t.Fatalf("vetor positivo reprovou: %v", ds)
	}
}

const (
	modA    = "exemplo/mod-a"
	pkgDom  = "exemplo/mod-a/domain"
	pkgPort = "exemplo/mod-a/port"
	pkgProv = "exemplo/mod-a/provider"
)

func unidade(id string, block rule.Block, bc string, include ...string) rule.Unit {
	return rule.Unit{ID: id, Block: block, BoundedContext: bc, Include: include, Module: modA}
}

func moduloOK() []rule.Module {
	return []rule.Module{{Path: modA, HasManifest: true, HasProduction: true}}
}

// ---------------------------------------------------------------- classe U ---

func TestVetorU001(t *testing.T) {
	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}
	g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}}}

	t.Run("positivo: todo package coberto", func(t *testing.T) {
		exigeLimpo(t, decideGrafo(t, units, moduloOK(), g))
	})

	t.Run("negativo: package fora de todo include", func(t *testing.T) {
		g := memGraph{packages: append(slices.Clone(g.packages),
			rule.Package{CanonicalKey: "exemplo/mod-a/orfao", Module: modA})}
		exigeCodigos(t, decideGrafo(t, units, moduloOK(), g), rule.CodeU001)
	})
}

func TestVetorU002(t *testing.T) {
	g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}}}

	t.Run("positivo: includes disjuntos", func(t *testing.T) {
		units := []rule.Unit{
			unidade("a/domain", rule.BlockDomain, "a", pkgDom),
			unidade("a/port", rule.BlockPort, "a", pkgPort),
		}
		exigeLimpo(t, decideGrafo(t, units, moduloOK(), g))
	})

	t.Run("negativo: duas unidades declarando o mesmo import path", func(t *testing.T) {
		units := []rule.Unit{
			unidade("a/domain", rule.BlockDomain, "a", pkgDom),
			unidade("a/tambem", rule.BlockApp, "a", pkgDom),
		}
		exigeCodigos(t, decideGrafo(t, units, moduloOK(), g), rule.CodeU002)
	})
}

func TestVetorU003(t *testing.T) {
	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}

	t.Run("positivo: canonical_keys únicas", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}}}
		exigeLimpo(t, decideGrafo(t, units, moduloOK(), g))
	})

	t.Run("negativo: canonical_key duplicada no universo", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{
			{CanonicalKey: pkgDom, Module: modA},
			{CanonicalKey: pkgDom, Module: "exemplo/mod-b"},
		}}
		exigeCodigos(t, decideGrafo(t, units, moduloOK(), g), rule.CodeU003)
	})
}

func TestVetorU004(t *testing.T) {
	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}
	g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}}}

	t.Run("positivo: módulo de produção com manifesto", func(t *testing.T) {
		exigeLimpo(t, decideGrafo(t, units, moduloOK(), g))
	})

	t.Run("negativo: módulo de produção sem manifesto", func(t *testing.T) {
		mods := append(moduloOK(), rule.Module{
			Path: "exemplo/mod-sem-manifesto", HasProduction: true, HasManifest: false,
		})
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeU004)
	})

	t.Run("negativo: módulo omitido do go.work não escapa", func(t *testing.T) {
		// O inventário é independente do go.work justamente para isto: o módulo
		// chega ao universo pela fonte que o viu (go.mod rastreado ou projeto Nx).
		mods := []rule.Module{{Path: "exemplo/mod-fora-do-workspace", HasProduction: true}}
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeU004)
	})
}

// ---------------------------------------------------------------- classe M ---

func TestVetorD001(t *testing.T) {
	units := []rule.Unit{
		unidade("a/domain", rule.BlockDomain, "a", pkgDom),
		unidade("a/port", rule.BlockPort, "a", pkgPort),
	}
	mods := moduloOK()
	pkgs := []rule.Package{{CanonicalKey: pkgDom, Module: modA}, {CanonicalKey: pkgPort, Module: modA}}

	t.Run("positivo: port -> domain (célula 19)", func(t *testing.T) {
		g := memGraph{packages: pkgs, edges: []port.Edge{
			{From: pkgPort, To: pkgDom, SourceFile: "port/p.go"},
		}}
		exigeLimpo(t, decideGrafo(t, units, mods, g))
	})

	t.Run("negativo: domain -> port (célula 4, ADR-014)", func(t *testing.T) {
		g := memGraph{packages: pkgs, edges: []port.Edge{
			{From: pkgDom, To: pkgPort, SourceFile: "domain/d.go"},
		}}
		ds := decideGrafo(t, units, mods, g)
		exigeCodigos(t, ds, rule.CodeD001)
		exigeDiagnostico(t, ds, rule.Diagnostic{
			Code: rule.CodeD001, CanonicalKey: pkgDom, Target: pkgPort, SourceFile: "domain/d.go",
		})
	})

	t.Run("negativo: domain -> provider (célula 5, P0-1)", func(t *testing.T) {
		units := append(slices.Clone(units), unidade("a/provider", rule.BlockProvider, "a", pkgProv))
		g := memGraph{
			packages: append(slices.Clone(pkgs), rule.Package{CanonicalKey: pkgProv, Module: modA}),
			edges:    []port.Edge{{From: pkgDom, To: pkgProv, SourceFile: "domain/d.go"}},
		}
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeD001)
	})
}

func TestVetorD002(t *testing.T) {
	const pkgB = "exemplo/mod-a/outro"
	mods := moduloOK()
	pkgs := []rule.Package{{CanonicalKey: pkgDom, Module: modA}, {CanonicalKey: pkgB, Module: modA}}
	g := memGraph{packages: pkgs, edges: []port.Edge{
		{From: pkgDom, To: pkgB, SourceFile: "domain/d.go"},
	}}

	t.Run("positivo: domain -> domain no mesmo context (célula 1)", func(t *testing.T) {
		units := []rule.Unit{
			unidade("a/domain", rule.BlockDomain, "a", pkgDom),
			unidade("a/outro", rule.BlockDomain, "a", pkgB),
		}
		exigeLimpo(t, decideGrafo(t, units, mods, g))
	})

	t.Run("negativo: domain -> domain entre contexts distintos", func(t *testing.T) {
		units := []rule.Unit{
			unidade("a/domain", rule.BlockDomain, "a", pkgDom),
			unidade("b/outro", rule.BlockDomain, "b", pkgB),
		}
		// Exatamente D002: domain -> domain é permitido na matriz, então só o
		// contexto reprova.
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeD002)
	})

	t.Run("positivo: contexts distintos com superfície pública no destino", func(t *testing.T) {
		units := []rule.Unit{
			unidade("a/domain", rule.BlockDomain, "a", pkgDom),
			{ID: "b/contract", Block: rule.BlockContract, BoundedContext: "b", Include: []string{pkgB}, Module: modA},
		}
		// Célula 6: superfície pública não substitui a matriz.
		// Exatamente D001: C2 passa por superfície pública e C1 reprova sozinha.
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeD001)
	})
}

// ---------------------------------------------------------------- classe E ---

func TestVetorE003(t *testing.T) {
	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}
	pkgs := []rule.Package{{CanonicalKey: pkgDom, Module: modA}}

	t.Run("positivo: todos os imports resolvem", func(t *testing.T) {
		g := memGraph{packages: pkgs}
		exigeLimpo(t, decideGrafo(t, units, moduloOK(), g))
	})

	t.Run("negativo: import não resolvido reprova, nunca é tratado como ausente", func(t *testing.T) {
		g := memGraph{packages: pkgs, edges: []port.Edge{
			{From: pkgDom, To: "modulo/inexistente", SourceFile: "domain/d.go", Unresolved: true},
		}}
		exigeCodigos(t, decideGrafo(t, units, moduloOK(), g), rule.CodeE003)
	})
}

// Não-emissão da classe reservada: a gramática exige `ImportPath = string_lit`,
// e o código segue no conjunto dos dezesseis como ausência declarada.
func TestE004NaoEmitidoNoBindingGo(t *testing.T) {
	spec, ok := rule.LookupCode(rule.CodeE004)
	if !ok {
		t.Fatal("DMPF-E004 ausente do conjunto fechado dos dezesseis")
	}
	if spec.Applicable {
		t.Error("DMPF-E004 marcado como aplicável no binding Go")
	}

	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}
	g := memGraph{
		packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}},
		edges:    []port.Edge{{From: pkgDom, To: pkgDom, SourceFile: "domain/d.go"}},
	}
	exigeCodigos(t, decideGrafo(t, units, moduloOK(), g))
}

// ------------------------------------------------------ conjunto e ordem ---

func TestSaidaEDeterministica(t *testing.T) {
	units := []rule.Unit{
		unidade("a/domain", rule.BlockDomain, "a", pkgDom),
		unidade("a/port", rule.BlockPort, "a", pkgPort),
		unidade("a/outro-port", rule.BlockPort, "a", "exemplo/mod-a/port2"),
	}
	mods := moduloOK()
	// Mesma CanonicalKey e mesmo código, diferindo só em Target e SourceFile:
	// o par que uma ordenação incompleta deixaria oscilar.
	entrada := memGraph{
		packages: []rule.Package{
			{CanonicalKey: pkgPort, Module: modA},
			{CanonicalKey: "exemplo/mod-a/port2", Module: modA},
			{CanonicalKey: pkgDom, Module: modA},
			{CanonicalKey: "exemplo/mod-a/orfao", Module: modA},
		},
		edges: []port.Edge{
			{From: pkgDom, To: pkgPort, SourceFile: "domain/b.go"},
			{From: pkgDom, To: "exemplo/mod-a/port2", SourceFile: "domain/a.go"},
		},
	}

	esperado := decideGrafo(t, units, mods, entrada)
	if len(esperado) != 3 {
		t.Fatalf("cenário perdeu força: esperados 3 diagnósticos (1 U001 + 2 D001), got %v", esperado)
	}

	// A saída não pode depender da ordem em que packages e arestas chegaram.
	for i := range 8 {
		embaralhado := memGraph{
			packages: slices.Clone(entrada.packages),
			edges:    slices.Clone(entrada.edges),
		}
		rnd := rand.New(rand.NewPCG(uint64(i), 0x5DEECE66D))
		rnd.Shuffle(len(embaralhado.packages), func(a, b int) {
			embaralhado.packages[a], embaralhado.packages[b] = embaralhado.packages[b], embaralhado.packages[a]
		})
		rnd.Shuffle(len(embaralhado.edges), func(a, b int) {
			embaralhado.edges[a], embaralhado.edges[b] = embaralhado.edges[b], embaralhado.edges[a]
		})

		got := decideGrafo(t, units, mods, embaralhado)
		if !slices.Equal(got, esperado) {
			t.Fatalf("ordem instável na iteração %d:\n  got      %v\n  esperado %v", i, got, esperado)
		}
	}

	if !slices.IsSortedFunc(esperado, func(a, b rule.Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.CanonicalKey, b.CanonicalKey),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Target, b.Target),
			cmp.Compare(a.SourceFile, b.SourceFile),
			cmp.Compare(a.Detail, b.Detail),
		)
	}) {
		t.Errorf("ordenação não é total sobre os campos de saída: %v", esperado)
	}
}

// ------------------------------------------------- casamento de `include` ---

// `include` designa o package, não a subárvore: prefixo faria `x/domain/infra`
// herdar a classificação só por estar embaixo dele.
func TestIncludeCasaImportPathExato(t *testing.T) {
	const sub = pkgDom + "/infra"
	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}
	mods := moduloOK()

	t.Run("positivo: package declarado é coberto", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}}}
		exigeLimpo(t, decideGrafo(t, units, mods, g))
	})

	t.Run("negativo: subpackage não declarado reprova com U001", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{
			{CanonicalKey: pkgDom, Module: modA},
			{CanonicalKey: sub, Module: modA},
		}}
		ds := decideGrafo(t, units, mods, g)
		exigeCodigos(t, ds, rule.CodeU001)
		exigeDiagnostico(t, ds, rule.Diagnostic{Code: rule.CodeU001, CanonicalKey: sub})
	})

	t.Run("positivo: subpackage declarado explicitamente é coberto", func(t *testing.T) {
		units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom, sub)}
		g := memGraph{packages: []rule.Package{
			{CanonicalKey: pkgDom, Module: modA},
			{CanonicalKey: sub, Module: modA},
		}}
		exigeLimpo(t, decideGrafo(t, units, mods, g))
	})

	t.Run("positivo: barra final no include não muda o package designado", func(t *testing.T) {
		units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom+"/")}
		g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom, Module: modA}}}
		exigeLimpo(t, decideGrafo(t, units, mods, g))
	})

	t.Run("negativo: prefixo comum não é cobertura", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{{CanonicalKey: pkgDom + "-legado", Module: modA}}}
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeU001)
	})
}

// Sem isso, unidade de módulo pai captura package de módulo aninhado e o
// classifica pelo módulo errado, escondendo o U004 do aninhado.
func TestUnidadeSoPossuiPackageDoProprioModulo(t *testing.T) {
	const aninhado = "exemplo/mod-a/nested"
	units := []rule.Unit{unidade("a/tudo", rule.BlockApp, "a", aninhado)}
	mods := []rule.Module{
		{Path: modA, HasManifest: true, HasProduction: true},
		{Path: aninhado, HasManifest: true, HasProduction: true},
	}

	t.Run("positivo: package do próprio módulo", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{{CanonicalKey: aninhado, Module: modA}}}
		exigeLimpo(t, decideGrafo(t, units, mods, g))
	})

	t.Run("negativo: mesmo import path, outro ownership_module", func(t *testing.T) {
		g := memGraph{packages: []rule.Package{{CanonicalKey: aninhado, Module: aninhado}}}
		exigeCodigos(t, decideGrafo(t, units, mods, g), rule.CodeU001)
	})
}
