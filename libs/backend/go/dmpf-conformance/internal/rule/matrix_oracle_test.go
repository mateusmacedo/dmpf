package rule

import "testing"

// Transcrita À MÃO da norma. Derivá-la de matrix.go faria o teste comparar
// o dado consigo mesmo e passar por qualquer valor.
type oracleCell struct {
	n      int
	source Block
	target Block
	allow  bool
}

// A numeração é a da norma, e é por ela que a documentação referencia cada
// decisão.
var oracle = []oracleCell{
	{1, BlockDomain, BlockDomain, true},
	{2, BlockDomain, BlockApplication, false},
	{3, BlockDomain, BlockApp, false},
	{4, BlockDomain, BlockPort, false},
	{5, BlockDomain, BlockProvider, false},
	{6, BlockDomain, BlockContract, false},
	{7, BlockApplication, BlockDomain, true},
	{8, BlockApplication, BlockApplication, true},
	{9, BlockApplication, BlockApp, false},
	{10, BlockApplication, BlockPort, true},
	{11, BlockApplication, BlockProvider, false},
	{12, BlockApplication, BlockContract, false},
	{13, BlockApp, BlockDomain, true},
	{14, BlockApp, BlockApplication, true},
	{15, BlockApp, BlockApp, true},
	{16, BlockApp, BlockPort, true},
	{17, BlockApp, BlockProvider, true},
	{18, BlockApp, BlockContract, true},
	{19, BlockPort, BlockDomain, true},
	{20, BlockPort, BlockApplication, false},
	{21, BlockPort, BlockApp, false},
	{22, BlockPort, BlockPort, true},
	{23, BlockPort, BlockProvider, false},
	{24, BlockPort, BlockContract, false},
	{25, BlockProvider, BlockDomain, true},
	{26, BlockProvider, BlockApplication, false},
	{27, BlockProvider, BlockApp, false},
	{28, BlockProvider, BlockPort, true},
	{29, BlockProvider, BlockProvider, true},
	{30, BlockProvider, BlockContract, true},
	{31, BlockContract, BlockDomain, false},
	{32, BlockContract, BlockApplication, false},
	{33, BlockContract, BlockApp, false},
	{34, BlockContract, BlockPort, false},
	{35, BlockContract, BlockProvider, false},
	{36, BlockContract, BlockContract, true},
}

func TestOracleCobreAsTrintaESeisCelulas(t *testing.T) {
	if len(oracle) != 36 {
		t.Fatalf("oráculo com %d células, esperado 36", len(oracle))
	}
	seen := map[[2]Block]bool{}
	for _, c := range oracle {
		k := [2]Block{c.source, c.target}
		if seen[k] {
			t.Errorf("célula %d duplicada: %s -> %s", c.n, c.source, c.target)
		}
		seen[k] = true
	}
	for _, s := range Blocks() {
		for _, tg := range Blocks() {
			if !seen[[2]Block{s, tg}] {
				t.Errorf("par %s -> %s ausente do oráculo", s, tg)
			}
		}
	}
}

func TestMatrizConfereComRFC74(t *testing.T) {
	for _, c := range oracle {
		got := AllowedByMatrix(c.source, c.target)
		if got != c.allow {
			t.Errorf("RFC §7.4 célula %d (%s -> %s): matriz devolveu %v, normativa diz %v",
				c.n, c.source, c.target, got, c.allow)
		}
	}
}

// C1 é só a matriz: não olha bounded context, superfície pública nem shared kernel.
func TestC1NaoDependeDeContexto(t *testing.T) {
	for _, c := range oracle {
		for _, bc := range []struct{ src, tgt string }{{"a", "a"}, {"a", "b"}} {
			for _, surface := range []bool{false, true} {
				for _, srcKernel := range []bool{false, true} {
					for _, tgtKernel := range []bool{false, true} {
						d := Decide(
							Endpoint{Block: c.source, BoundedContext: bc.src, SharedKernel: srcKernel},
							Endpoint{Block: c.target, BoundedContext: bc.tgt, PublicIntegrationSurface: surface, SharedKernel: tgtKernel},
						)
						if d.C1 != c.allow {
							t.Errorf("célula %d (%s -> %s) com bc=%v/%v surface=%v srcKernel=%v tgtKernel=%v: C1=%v, esperado %v",
								c.n, c.source, c.target, bc.src, bc.tgt, surface, srcKernel, tgtKernel, d.C1, c.allow)
						}
					}
				}
			}
		}
	}
}

func TestC2TabelaVerdade(t *testing.T) {
	casos := []struct {
		nome      string
		srcBC     string
		tgtBC     string
		tgtBlock  Block
		surface   bool
		srcKernel bool
		tgtKernel bool
		querC2    bool
	}{
		{"mesmo contexto, sem superfície", "a", "a", BlockDomain, false, false, false, true},
		{"mesmo contexto, com superfície", "a", "a", BlockApplication, true, false, false, true},
		{"contextos distintos, sem superfície", "a", "b", BlockDomain, false, false, false, false},
		{"contextos distintos, com superfície", "a", "b", BlockApplication, true, false, false, true},
		{"contextos distintos, destino contract", "a", "b", BlockContract, false, false, false, true},
		{"mesmo contexto, destino contract", "a", "a", BlockContract, false, false, false, true},
		{"contextos distintos, destino no shared kernel", "a", "b", BlockDomain, false, false, true, true},
		{"contextos distintos, origem no shared kernel não propaga", "a", "b", BlockDomain, false, true, false, false},
		{"contextos distintos, nenhum designado shared kernel", "a", "b", BlockApplication, false, false, false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := Decide(
				Endpoint{Block: BlockApp, BoundedContext: c.srcBC, SharedKernel: c.srcKernel},
				Endpoint{Block: c.tgtBlock, BoundedContext: c.tgtBC, PublicIntegrationSurface: c.surface, SharedKernel: c.tgtKernel},
			)
			if d.C2 != c.querC2 {
				t.Errorf("C2=%v, esperado %v", d.C2, c.querC2)
			}
		})
	}

	t.Run("destino no shared kernel com C1 reprovando (célula 4)", func(t *testing.T) {
		src := Endpoint{CanonicalKey: "x/domain", Block: BlockDomain, BoundedContext: "a"}
		tgt := Endpoint{CanonicalKey: "y/port", Block: BlockPort, BoundedContext: "b", SharedKernel: true}

		d := Decide(src, tgt)
		if !d.C2 {
			t.Fatalf("C2=%v, esperado true (destino no shared kernel)", d.C2)
		}
		if d.Allowed() {
			t.Fatalf("Allowed()=true, esperado false (célula 4 reprova C1)")
		}

		ds := DiagnoseEdge(src, tgt, "x/domain/a.go")
		if len(ds) != 1 || ds[0].Code != CodeD001 {
			t.Fatalf("esperado só D001, got %v", ds)
		}
	})
}

func TestConjuncaoEIndependente(t *testing.T) {
	src := Endpoint{CanonicalKey: "x/domain", Block: BlockDomain, BoundedContext: "a"}
	tgt := Endpoint{CanonicalKey: "y/provider", Block: BlockProvider, BoundedContext: "b"}

	d := Decide(src, tgt)
	if d.C1 || d.C2 || d.Allowed() {
		t.Fatalf("esperado C1 e C2 falsas; got %+v", d)
	}

	ds := DiagnoseEdge(src, tgt, "x/domain/a.go")
	if len(ds) != 2 {
		t.Fatalf("esperados 2 diagnósticos, got %d: %v", len(ds), ds)
	}
	if ds[0].Code != CodeD001 || ds[1].Code != CodeD002 {
		t.Errorf("esperados D001 e D002, got %s e %s", ds[0].Code, ds[1].Code)
	}
}

// domain -> port é proibida sem condicional: nenhum contexto, superfície
// pública ou flag a relaxa.
func TestDomainPortProibidaSemExcecao(t *testing.T) {
	for _, bc := range []string{"mesmo", "outro"} {
		for _, surface := range []bool{false, true} {
			d := Decide(
				Endpoint{Block: BlockDomain, BoundedContext: "mesmo"},
				Endpoint{Block: BlockPort, BoundedContext: bc, PublicIntegrationSurface: surface},
			)
			if d.Allowed() {
				t.Errorf("domain -> port permitida com bc=%s surface=%v (RFC §7.4 célula 4; ADR-014)", bc, surface)
			}
		}
	}
}
