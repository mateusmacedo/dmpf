package rule_test

import (
	"slices"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/manifest"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

func codigos(ds []rule.Diagnostic) []rule.Code {
	out := make([]rule.Code, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
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

func doc(units ...manifest.Unit) manifest.Document {
	return manifest.Document{
		Path:   "exemplo/mod-a/dmpf-units.json",
		Module: modA,
		Schema: manifest.SchemaID,
		Units:  units,
	}
}

func unidadeManifesto(id, block, bc string, include ...string) manifest.Unit {
	return manifest.Unit{
		ID: id, Block: block, BoundedContext: bc, Include: include,
		PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
	}
}

func TestVetorM001(t *testing.T) {
	t.Run("positivo: unidade com todos os obrigatórios", func(t *testing.T) {
		exigeLimpo(t, manifest.Validate(doc(unidadeManifesto("a/domain", "domain", "a", pkgDom))))
	})

	t.Run("negativo: bounded_context ausente", func(t *testing.T) {
		u := unidadeManifesto("a/domain", "domain", "", pkgDom)
		u.PresentBoundedContext = false
		exigeCodigos(t, manifest.Validate(doc(u)), rule.CodeM001)
	})

	t.Run("negativo: herança não supre campo omitido", func(t *testing.T) {
		// Não há herança: a omissão é DMPF-M001, não valor derivado do pai.
		u := unidadeManifesto("a/filha", "", "a", pkgDom)
		u.PresentBlock = false
		exigeCodigos(t, manifest.Validate(doc(u)), rule.CodeM001)
	})
}

func TestVetorM002(t *testing.T) {
	t.Run("positivo: block entre os seis valores", func(t *testing.T) {
		exigeLimpo(t, manifest.Validate(doc(unidadeManifesto("a/contract", "contract", "a", pkgDom))))
	})

	t.Run("negativo: block fora do conjunto fechado", func(t *testing.T) {
		exigeCodigos(t, manifest.Validate(doc(unidadeManifesto("a/core", "core", "a", pkgDom))), rule.CodeM002)
	})

	t.Run("negativo: public_integration_surface true em domain", func(t *testing.T) {

		u := unidadeManifesto("a/domain", "domain", "a", pkgDom)
		u.PublicIntegrationSurface = true
		u.PresentPublicIntegrationSurface = true
		exigeCodigos(t, manifest.Validate(doc(u)), rule.CodeM002)
	})

	t.Run("positivo: public_integration_surface true em contract", func(t *testing.T) {
		u := unidadeManifesto("a/contract", "contract", "a", pkgDom)
		u.PublicIntegrationSurface = true
		u.PresentPublicIntegrationSurface = true
		exigeLimpo(t, manifest.Validate(doc(u)))
	})

	t.Run("negativo: schema fora do conjunto fechado", func(t *testing.T) {
		d := doc(unidadeManifesto("a/domain", "domain", "a", pkgDom))
		d.Schema = "dmpf/units@2"
		exigeCodigos(t, manifest.Validate(d), rule.CodeM002)
	})
}

func TestVetorM003(t *testing.T) {
	t.Run("positivo: ids únicos", func(t *testing.T) {
		exigeLimpo(t, manifest.Validate(doc(
			unidadeManifesto("a/domain", "domain", "a", pkgDom),
			unidadeManifesto("a/port", "port", "a", pkgPort),
		)))
	})

	t.Run("negativo: id duplicado no mesmo manifesto", func(t *testing.T) {
		exigeCodigos(t, manifest.Validate(doc(
			unidadeManifesto("a/domain", "domain", "a", pkgDom),
			unidadeManifesto("a/domain", "port", "a", pkgPort),
		)), rule.CodeM003)
	})
}

func TestDefaultDeSurfaceEFalse(t *testing.T) {
	u := unidadeManifesto("a/app", "app", "a", pkgDom)
	if u.PublicIntegrationSurface || u.PresentPublicIntegrationSurface {
		t.Fatal("public_integration_surface ausente deve ter default false")
	}
	exigeLimpo(t, manifest.Validate(doc(u)))
}

// ---------------------------------------------------------------- classe D ---

// Literais transcritos à mão. Usar as constantes de produção como expectativa
// deixaria renomear DMPF-U001 para DMPF-U999 passar verde.
var codigosNormativos = []string{
	"DMPF-U001", "DMPF-U002", "DMPF-U003", "DMPF-U004",
	"DMPF-M001", "DMPF-M002", "DMPF-M003", "DMPF-M004",
	"DMPF-T001", "DMPF-T002",
	"DMPF-D001", "DMPF-D002",
	"DMPF-E001", "DMPF-E002", "DMPF-E003", "DMPF-E004",
}

func TestConjuntoFechadoDeDezesseisCodigos(t *testing.T) {
	specs := rule.CodeSpecs()

	got := make([]string, 0, len(specs))
	for _, s := range specs {
		got = append(got, string(s.Code))
		if s.Section == "" {
			t.Errorf("código %s sem seção normativa rastreável", s.Code)
		}
	}
	if !slices.Equal(got, codigosNormativos) {
		t.Fatalf("literais emitidos\n  %v\nRFC §10.3 fixa\n  %v", got, codigosNormativos)
	}

	vistos := map[string]bool{}
	for _, c := range got {
		if vistos[c] {
			t.Errorf("código %s duplicado", c)
		}
		vistos[c] = true
	}
}

func TestLookupCodeCobreOsDezesseis(t *testing.T) {
	for _, c := range codigosNormativos {
		if _, ok := rule.LookupCode(rule.Code(c)); !ok {
			t.Errorf("literal normativo %s ausente da tabela de §10.3", c)
		}
	}
	if _, ok := rule.LookupCode(rule.Code("DMPF-U999")); ok {
		t.Error("código inexistente resolvido: a tabela não é fechada")
	}
}

func TestVetorIncludeInvalido(t *testing.T) {
	casos := []struct {
		nome    string
		include []string
		limpo   bool
	}{
		{"positivo: import path canônico", []string{pkgDom}, true},
		{"positivo: barra final tolerada", []string{pkgDom + "/"}, true},
		{"negativo: elemento vazio junto de um válido", []string{pkgDom, ""}, false},
		{"negativo: só barra", []string{"/"}, false},
		{"negativo: barra inicial", []string{"/" + pkgDom}, false},
		{"negativo: barra dupla", []string{"exemplo//mod-a"}, false},
		{"negativo: espaço nas bordas", []string{" " + pkgDom}, false},
		{"negativo: segmento relativo", []string{"exemplo/../mod-a"}, false},
		{"negativo: glob é binding de TypeScript", []string{"exemplo/mod-a/**"}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			u := unidadeManifesto("a/domain", "domain", "a", c.include...)
			ds := manifest.Validate(doc(u))
			if c.limpo {
				exigeLimpo(t, ds)
				return
			}
			exigeCodigos(t, ds, rule.CodeM002)
		})
	}
}

// ------------------------------------------- identidade e imutabilidade ---

// O `id` só é único dentro de um manifesto: chavear só por ele fundiria
// unidades homônimas de módulos distintos e cegaria a movimentação entre elas.
func TestMembershipQualificadoPorModulo(t *testing.T) {
	const modB, pkgB = "exemplo/mod-b", "exemplo/mod-b/domain"
	units := []rule.Unit{
		{ID: "domain", Block: rule.BlockDomain, BoundedContext: "a", Include: []string{pkgDom}, Module: modA},
		{ID: "domain", Block: rule.BlockDomain, BoundedContext: "b", Include: []string{pkgB}, Module: modB},
	}
	pkgs := []rule.Package{{CanonicalKey: pkgDom, Module: modA}, {CanonicalKey: pkgB, Module: modB}}
	mods := []rule.Module{
		{Path: modA, HasManifest: true, HasProduction: true},
		{Path: modB, HasManifest: true, HasProduction: true},
	}

	universe, ds := rule.BuildUniverse(units, pkgs, mods)
	exigeLimpo(t, ds)

	membership := universe.Membership()
	if len(membership) != 2 {
		t.Fatalf("membership com %d unidades, esperadas 2 (ids homônimos em módulos distintos): %v", len(membership), membership)
	}
	for _, k := range []rule.UnitKey{{Module: modA, ID: "domain"}, {Module: modB, ID: "domain"}} {
		if len(membership[k]) != 1 {
			t.Errorf("membership de %s = %v, esperado exatamente 1 package", k, membership[k])
		}
	}
}

// O veredicto não pode mudar porque o chamador mexeu no slice que passou.
func TestUniverseNaoAliasaEntradaDoChamador(t *testing.T) {
	units := []rule.Unit{unidade("a/domain", rule.BlockDomain, "a", pkgDom)}
	pkgs := []rule.Package{{CanonicalKey: pkgDom, Module: modA}}

	universe, ds := rule.BuildUniverse(units, pkgs, moduloOK())
	exigeLimpo(t, ds)

	units[0].Block = rule.BlockProvider
	units[0].BoundedContext = "outro"
	units[0].Include[0] = "sequestrado"

	got, ok := universe.Endpoint(pkgDom)
	if !ok {
		t.Fatal("package sumiu do universo após mutação da entrada")
	}
	if got.Block != rule.BlockDomain || got.BoundedContext != "a" {
		t.Errorf("classificação mudou após a construção: %+v", got)
	}

	unit, _ := universe.Lookup(pkgDom)
	if unit.Include[0] != pkgDom {
		t.Errorf("Include aliasado: %q", unit.Include[0])
	}
	unit.Include[0] = "mutado pelo consumidor"
	if again, _ := universe.Lookup(pkgDom); again.Include[0] != pkgDom {
		t.Errorf("Lookup devolve alias mutável: %q", again.Include[0])
	}
}

// A designação vive na Unit, e só alcança C2 se o Universe a repassar: sem
// esta cópia a exceção existiria no baseline e nunca decidiria aresta alguma.
func TestSharedKernelChegaAoEndpoint(t *testing.T) {
	designada := unidade("a/domain", rule.BlockDomain, "a", pkgDom)
	designada.SharedKernel = true
	privada := unidade("a/port", rule.BlockPort, "a", pkgPort)

	universe, ds := rule.BuildUniverse(
		[]rule.Unit{designada, privada},
		[]rule.Package{{CanonicalKey: pkgDom, Module: modA}, {CanonicalKey: pkgPort, Module: modA}},
		moduloOK(),
	)
	exigeLimpo(t, ds)

	casos := []struct {
		pkg  string
		quer bool
	}{
		{pkgDom, true},
		{pkgPort, false},
	}
	for _, c := range casos {
		got, ok := universe.Endpoint(c.pkg)
		if !ok {
			t.Fatalf("%s ausente do universo", c.pkg)
		}
		if got.SharedKernel != c.quer {
			t.Errorf("Endpoint(%s).SharedKernel=%v, esperado %v", c.pkg, got.SharedKernel, c.quer)
		}
	}
}
