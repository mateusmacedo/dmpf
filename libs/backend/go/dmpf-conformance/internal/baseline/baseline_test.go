package baseline_test

import (
	"slices"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/baseline"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

func entrada(unit, block, bc string, membros ...string) baseline.Entry {
	return baseline.Entry{Unit: unit, Module: "m", Block: block, BoundedContext: bc, Membership: membros}
}

func doc(entries ...baseline.Entry) baseline.Document {
	return baseline.Document{Schema: baseline.SchemaID, Digest: baseline.Digest(entries), Entries: entries}
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

// ------------------------------------------------ atos que exigem aval ---

func TestDetectarAtosRegulados(t *testing.T) {
	antes := doc(entrada("u", "domain", "bc", "m/a"))

	casos := []struct {
		nome string
		novo baseline.Document
		atos []baseline.Ato
	}{
		{"sem mudança", doc(entrada("u", "domain", "bc", "m/a")), nil},
		{"alterar block", doc(entrada("u", "app", "bc", "m/a")), []baseline.Ato{baseline.AtoAlterarBlock}},
		{"alterar bounded_context", doc(entrada("u", "domain", "x", "m/a")), []baseline.Ato{baseline.AtoAlterarBoundedContext}},
		{"remapear membership", doc(entrada("u", "domain", "bc", "m/b")), []baseline.Ato{baseline.AtoRemapearMembership}},
		{"criar unidade", doc(entrada("u", "domain", "bc", "m/a"), entrada("n", "app", "bc", "m/b")), []baseline.Ato{baseline.AtoCriarUnidade}},
		{"remover unidade", doc(), []baseline.Ato{baseline.AtoRemoverUnidade}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := baseline.Detectar(antes, c.novo)
			atos := make([]baseline.Ato, 0, len(got))
			for _, m := range got {
				atos = append(atos, m.Ato)
			}
			if !slices.Equal(atos, c.atos) {
				t.Errorf("atos %v, esperado %v", atos, c.atos)
			}
		})
	}
}

// ------------------------------------------------- ausência de aval ---

func TestVetorT002(t *testing.T) {
	mudanca := []baseline.MudancaNormativa{{
		Ato: baseline.AtoAlterarBlock, Unidade: rule.UnitKey{Module: "m", ID: "u"},
		Anterior: "domain", Novo: "app",
	}}

	t.Run("positivo: mudança normativa em commit próprio", func(t *testing.T) {
		commits := []baseline.Commit{
			{SHA: "aaa", Autor: "a", Arquivos: []string{baseline.Path, "m/dmpf-units.json"}},
			{SHA: "bbb", Autor: "a", Arquivos: []string{"m/p/p.go"}},
		}
		if ds := baseline.VerificarAutorizacao(mudanca, commits); len(ds) != 0 {
			t.Fatalf("commit próprio reprovou: %v", ds)
		}
	})

	t.Run("negativo: normativa misturada com código no mesmo commit", func(t *testing.T) {
		// O ataque que essa regra existe para tornar visível.
		commits := []baseline.Commit{
			{SHA: "aaa", Autor: "a", Arquivos: []string{baseline.Path, "m/dmpf-units.json", "m/p/p.go"}},
		}
		ds := baseline.VerificarAutorizacao(mudanca, commits)
		if len(ds) != 1 || ds[0].Code != rule.CodeT002 {
			t.Fatalf("esperado exatamente DMPF-T002, got %v", ds)
		}
	})

	t.Run("negativo: sem histórico é não verificado, nunca conforme", func(t *testing.T) {
		ds := baseline.VerificarAutorizacao(mudanca, nil)
		if len(ds) != 1 || ds[0].Code != rule.CodeT002 {
			t.Fatalf("ausência de histórico não reprovou: %v", ds)
		}
	})

	t.Run("sem mudança normativa não exige nada", func(t *testing.T) {
		if ds := baseline.VerificarAutorizacao(nil, nil); len(ds) != 0 {
			t.Fatalf("cobrou autorização sem ato regulado: %v", ds)
		}
	})
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
