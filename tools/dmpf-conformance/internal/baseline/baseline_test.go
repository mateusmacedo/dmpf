package baseline_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

func entrada(unit, block, bc string, membros ...string) baseline.Entry {
	return baseline.Entry{Unit: unit, Module: "m", Block: block, BoundedContext: bc, Membership: membros}
}

func doc(entries ...baseline.Entry) baseline.Document {
	return baseline.Document{Schema: baseline.SchemaID, Digest: baseline.Digest(entries), Entries: entries}
}

func comSharedKernelUnits(d baseline.Document, chaves ...string) baseline.Document {
	d.SharedKernelUnits = chaves
	d.HasSharedKernelUnits = true
	d.Digest = baseline.DigestOf(d)
	return d
}

func codigos(ds []rule.Diagnostic) []rule.Code {
	out := make([]rule.Code, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
}

func exigeT001(t *testing.T, ds []rule.Diagnostic, quantos int) {
	t.Helper()
	if len(ds) != quantos {
		t.Fatalf("esperados %d diagnóstico(s), got %d: %v", quantos, len(ds), ds)
	}
	for _, c := range codigos(ds) {
		if c != rule.CodeT001 {
			t.Errorf("código %s, esperado DMPF-T001", c)
		}
	}
}

// ------------------------------------------------- onde o baseline vive ---

// Dentro do módulo que descreve, o mesmo commit mexeria nos dois lados e a
// comparação nunca acusaria nada.
func TestBaselineViveForaDoOwnershipModule(t *testing.T) {
	if baseline.Path != "tools/dmpf-baseline/units-baseline.json" {
		t.Fatalf("caminho do baseline mudou: %s", baseline.Path)
	}
	for _, prefixo := range []string{"libs/", "apps/"} {
		if len(baseline.Path) >= len(prefixo) && baseline.Path[:len(prefixo)] == prefixo {
			t.Errorf("baseline sob %s: pode cair dentro de um ownership_module", prefixo)
		}
	}
}

// -------------------------------------------- divergência com o manifesto ---

func TestVetorT001(t *testing.T) {
	versionado := doc(entrada("u", "domain", "bc", "m/a", "m/b"))

	t.Run("positivo: manifesto e baseline coincidem", func(t *testing.T) {
		exigeT001(t, baseline.Compare(versionado, doc(entrada("u", "domain", "bc", "m/a", "m/b"))), 0)
	})

	t.Run("negativo: block alterado só no manifesto", func(t *testing.T) {
		exigeT001(t, baseline.Compare(versionado, doc(entrada("u", "app", "bc", "m/a", "m/b"))), 1)
	})

	t.Run("negativo: bounded_context alterado só no manifesto", func(t *testing.T) {
		exigeT001(t, baseline.Compare(versionado, doc(entrada("u", "domain", "outro", "m/a", "m/b"))), 1)
	})

	t.Run("negativo: remapeamento de membership sem tocar campo", func(t *testing.T) {
		// Mover package entre unidades muda a classificação sem editar campo:
		// um baseline só de chaves não veria isso.
		exigeT001(t, baseline.Compare(versionado, doc(entrada("u", "domain", "bc", "m/a", "m/c"))), 1)
	})

	t.Run("negativo: unidade nova no manifesto", func(t *testing.T) {
		novo := doc(entrada("u", "domain", "bc", "m/a", "m/b"), entrada("nova", "app", "bc", "m/c"))
		exigeT001(t, baseline.Compare(versionado, novo), 1)
	})

	t.Run("negativo: unidade removida do manifesto", func(t *testing.T) {
		exigeT001(t, baseline.Compare(versionado, doc()), 1)
	})

	t.Run("negativo: digest não fecha com as próprias entradas", func(t *testing.T) {
		corrompido := versionado
		corrompido.Digest = "sha256:0000"
		exigeT001(t, baseline.Compare(corrompido, doc(entrada("u", "domain", "bc", "m/a", "m/b"))), 1)
	})

	t.Run("negativo: schema fora do conjunto fechado", func(t *testing.T) {
		outro := versionado
		outro.Schema = "dmpf/units-baseline@2"
		exigeT001(t, baseline.Compare(outro, doc(entrada("u", "domain", "bc", "m/a", "m/b"))), 1)
	})

	t.Run("negativo: shared_kernel_units declarado e digest não recalculado", func(t *testing.T) {
		comSharedKernel := versionado
		comSharedKernel.SharedKernelUnits = []string{"u"}
		comSharedKernel.HasSharedKernelUnits = true
		// Digest continua o legado: não fecha com DigestOf porque a lista entrou no cálculo.
		exigeT001(t, baseline.Compare(comSharedKernel, doc(entrada("u", "domain", "bc", "m/a", "m/b"))), 1)
	})

	t.Run("positivo: shared_kernel_units declarado e digest recalculado com DigestOf", func(t *testing.T) {
		comSharedKernel := versionado
		comSharedKernel.SharedKernelUnits = []string{"u"}
		comSharedKernel.HasSharedKernelUnits = true
		comSharedKernel.Digest = baseline.DigestOf(comSharedKernel)
		exigeT001(t, baseline.Compare(comSharedKernel, doc(entrada("u", "domain", "bc", "m/a", "m/b"))), 0)
	})

	t.Run("positivo: baseline sem a chave segue verificando pelo legado", func(t *testing.T) {
		exigeT001(t, baseline.Compare(versionado, doc(entrada("u", "domain", "bc", "m/a", "m/b"))), 0)
	})
}

// Se o digest não reagir a alguma dimensão, alterá-la e manter o digest
// deixaria o arquivo coerente com uma classificação diferente.
func TestDigestSensivelAsQuatroDimensoes(t *testing.T) {
	base := []baseline.Entry{entrada("u", "domain", "bc", "m/a", "m/b")}
	d := baseline.Digest(base)

	if baseline.Digest(base) != d {
		t.Fatal("digest não é determinístico")
	}
	casos := map[string][]baseline.Entry{
		"block":               {entrada("u", "app", "bc", "m/a", "m/b")},
		"bounded_context":     {entrada("u", "domain", "outro", "m/a", "m/b")},
		"membership":          {entrada("u", "domain", "bc", "m/a")},
		"ordem do membership": {entrada("u", "domain", "bc", "m/b", "m/a")},
		"unidade":             {entrada("outra", "domain", "bc", "m/a", "m/b")},
	}
	for nome, e := range casos {
		if baseline.Digest(e) == d {
			t.Errorf("digest insensível a mudança de %s", nome)
		}
	}
}

// Sem estabilidade, regravar produziria diff a cada execução e a revisão
// perderia o sinal.
func TestFromUniverseOrdenaEFechaODigest(t *testing.T) {
	units := []rule.Unit{
		{ID: "z", Block: rule.BlockApp, BoundedContext: "bc", Module: "m"},
		{ID: "a", Block: rule.BlockDomain, BoundedContext: "bc", Module: "m"},
	}
	membership := map[rule.UnitKey][]string{
		{Module: "m", ID: "z"}: {"m/z2", "m/z1"},
		{Module: "m", ID: "a"}: {"m/a1"},
	}

	d := baseline.FromUniverse(units, membership)
	if len(d.Entries) != 2 || d.Entries[0].Unit != "a" {
		t.Fatalf("entradas fora de ordem: %+v", d.Entries)
	}
	if !slices.Equal(d.Entries[1].Membership, []string{"m/z1", "m/z2"}) {
		t.Errorf("membership não ordenado: %v", d.Entries[1].Membership)
	}
	if d.Digest != baseline.Digest(d.Entries) {
		t.Error("digest não fecha com as próprias entradas")
	}
	for i := 0; i < 3; i++ {
		if baseline.FromUniverse(units, membership).Digest != d.Digest {
			t.Fatal("derivação não é determinística")
		}
	}
}

func TestDesignarChaveResolvidaMarcaAUnidade(t *testing.T) {
	d := doc(entrada("shared", "domain", "bc", "m/a"))
	d.SharedKernelUnits = []string{"shared"}
	d.HasSharedKernelUnits = true

	units := []rule.Unit{{ID: "shared", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	out, diags := baseline.Designar(d, units)

	exigeT001(t, diags, 0)
	if !out[0].SharedKernel {
		t.Fatal("unidade resolvida não foi marcada como shared kernel")
	}
	if units[0].SharedKernel {
		t.Error("Designar mutou a entrada do chamador")
	}
}

func TestDesignarChaveInexistenteNoBaselineEmiteM004(t *testing.T) {
	d := doc(entrada("u", "domain", "bc", "m/a"))
	d.SharedKernelUnits = []string{"fantasma"}
	d.HasSharedKernelUnits = true

	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	_, diags := baseline.Designar(d, units)

	if len(diags) != 1 || diags[0].Code != rule.CodeM004 {
		t.Fatalf("esperado exatamente 1 DMPF-M004, got %v", diags)
	}
}

func TestDesignarChaveAmbiguaEmDoisModulosEmiteM004(t *testing.T) {
	d := baseline.Document{
		Schema: baseline.SchemaID,
		Entries: []baseline.Entry{
			{Unit: "shared", Module: "m1", Block: "domain", BoundedContext: "bc1"},
			{Unit: "shared", Module: "m2", Block: "domain", BoundedContext: "bc2"},
		},
		SharedKernelUnits:    []string{"shared"},
		HasSharedKernelUnits: true,
	}
	units := []rule.Unit{
		{ID: "shared", Module: "m1", Block: rule.BlockDomain, BoundedContext: "bc1"},
		{ID: "shared", Module: "m2", Block: rule.BlockDomain, BoundedContext: "bc2"},
	}
	_, diags := baseline.Designar(d, units)

	if len(diags) != 1 || diags[0].Code != rule.CodeM004 {
		t.Fatalf("esperado exatamente 1 DMPF-M004 (ambígua), got %v", diags)
	}
	if units[0].SharedKernel || units[1].SharedKernel {
		t.Error("chave ambígua não pode marcar nenhuma unidade")
	}
}

// Sem M004 aqui a designação falharia em silêncio: a chave resolve para uma
// entry do baseline que já não existe em `units`, e a aresta seria decidida
// sobre um shared kernel fora do universo.
func TestDesignarChaveResolvidaEAusenteDoUniversoEmiteM004(t *testing.T) {
	d := doc(entrada("orfa", "domain", "bc", "m/a"))
	d.SharedKernelUnits = []string{"orfa"}
	d.HasSharedKernelUnits = true

	units := []rule.Unit{{ID: "outra", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	out, diags := baseline.Designar(d, units)

	if len(diags) != 1 || diags[0].Code != rule.CodeM004 {
		t.Fatalf("esperado exatamente 1 DMPF-M004 (ausente do universo), got %v", diags)
	}
	if out[0].SharedKernel {
		t.Error("unidade sem relação com a chave não pode ser marcada")
	}
}

func TestDesignarOrdemDeSaidaEstavel(t *testing.T) {
	d := doc(entrada("u", "domain", "bc", "m/a"))
	d.SharedKernelUnits = []string{"z-fantasma", "a-fantasma"}
	d.HasSharedKernelUnits = true

	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	_, diags := baseline.Designar(d, units)

	if len(diags) != 2 || diags[0].CanonicalKey != "a-fantasma" || diags[1].CanonicalKey != "z-fantasma" {
		t.Fatalf("ordem de saída não é estável por chave: %v", diags)
	}
}

func TestRegravarPreservaSharedKernelUnits(t *testing.T) {
	atual := comSharedKernelUnits(doc(entrada("u", "domain", "bc", "m/a")), "u")
	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	membership := map[rule.UnitKey][]string{{Module: "m", ID: "u"}: {"m/a"}}

	regravado := baseline.Regravar(atual, true, units, membership)

	if !regravado.HasSharedKernelUnits {
		t.Fatal("Has virou false ao regravar")
	}
	if !slices.Equal(regravado.SharedKernelUnits, []string{"u"}) {
		t.Errorf("lista de shared kernel apagada: %v", regravado.SharedKernelUnits)
	}
	if regravado.Digest != baseline.DigestOf(regravado) {
		t.Error("digest não fecha após regravar")
	}
}

func TestRegravarSemAtualDeclaraListaVazia(t *testing.T) {
	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	membership := map[rule.UnitKey][]string{{Module: "m", ID: "u"}: {"m/a"}}

	regravado := baseline.Regravar(baseline.Document{}, false, units, membership)

	if !regravado.HasSharedKernelUnits {
		t.Fatal("Has deveria ser true mesmo sem atual")
	}
	if len(regravado.SharedKernelUnits) != 0 {
		t.Errorf("lista deveria ser vazia, got %v", regravado.SharedKernelUnits)
	}
}

func TestRegravarIgnoraAtualQuandoNaoExistia(t *testing.T) {
	atual := comSharedKernelUnits(doc(entrada("u", "domain", "bc", "m/a")), "u")
	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	membership := map[rule.UnitKey][]string{{Module: "m", ID: "u"}: {"m/a"}}

	regravado := baseline.Regravar(atual, false, units, membership)

	if len(regravado.SharedKernelUnits) != 0 {
		t.Errorf("existia=false deveria ignorar a lista do atual: %v", regravado.SharedKernelUnits)
	}
}
