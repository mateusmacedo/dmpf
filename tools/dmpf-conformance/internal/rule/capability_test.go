package rule_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

// Transcrição À MÃO da norma. Como o oráculo da matriz, NÃO deriva de
// capabilityPolicy: derivá-la faria o teste comparar o dado consigo mesmo.
var politicaNormativa = map[rule.Block][]rule.Capability{
	rule.BlockDomain:      {rule.CapPure},
	rule.BlockPort:        {rule.CapPure},
	rule.BlockApplication: {rule.CapPure, rule.CapObservability},
	rule.BlockContract:    {rule.CapPure, rule.CapWireCodec},
}

var blocosPermissivos = []rule.Block{rule.BlockProvider, rule.BlockApp}

func TestPoliticaDeCapabilityConfereComRFC62(t *testing.T) {
	for _, b := range rule.Blocks() {
		for _, c := range rule.Capabilities() {
			esperado := slices.Contains(blocosPermissivos, b) ||
				slices.Contains(politicaNormativa[b], c)
			if got := rule.CapabilityAllowed(b, c); got != esperado {
				t.Errorf("RFC §6.2 (%s, %s): devolveu %v, normativa diz %v", b, c, got, esperado)
			}
		}
	}
}

// Princípio 11, e a exceção que mais se pede — por isso tem vetor próprio.
func TestObservabilidadeVedadaNoDominioENaPorta(t *testing.T) {
	for _, b := range []rule.Block{rule.BlockDomain, rule.BlockPort} {
		if rule.CapabilityAllowed(b, rule.CapObservability) {
			t.Errorf("observability permitida em %s (RFC §6.2, princípio 11)", b)
		}
	}
	if !rule.CapabilityAllowed(rule.BlockApplication, rule.CapObservability) {
		t.Error("observability negada em application: a vedação é de domain e port, não geral")
	}
}

func TestBlocoOuCapabilityForaDoConjuntoFechadoReprova(t *testing.T) {
	for _, b := range []rule.Block{"", "core", "DOMAIN"} {
		if rule.CapabilityAllowed(b, rule.CapPure) {
			t.Errorf("bloco desconhecido %q aceitou pure", b)
		}
	}
	for _, c := range []rule.Capability{"", "io", "PURE", "io.socket"} {
		if rule.CapabilityAllowed(rule.BlockProvider, c) {
			t.Errorf("capability desconhecida %q aceita em bloco permissivo", c)
		}
	}
}

func TestConjuntoFechadoDeCapabilities(t *testing.T) {
	// Transcritos à mão da norma, não derivados das constantes de produção.
	normativas := []string{
		"io.storage", "io.messaging", "io.network", "io.filesystem",
		"io.clock", "io.random", "runtime.framework", "wire.codec",
		"observability", "pure",
	}
	got := make([]string, 0, 10)
	for _, c := range rule.Capabilities() {
		got = append(got, string(c))
	}
	if !slices.Equal(got, normativas) {
		t.Fatalf("capabilities emitidas\n  %v\nRFC §6.1 fixa\n  %v", got, normativas)
	}
}

// ------------------------------------------------------------- stdlib ---

func TestCapabilityDaStdlib(t *testing.T) {
	casos := []struct {
		pkg  string
		want rule.Capability
	}{
		{"net/http", rule.CapIONetwork},   // citado nominalmente em RFC §6.3
		{"crypto", rule.CapPure},          // idem
		{"crypto/sha256", rule.CapPure},   // herda por prefixo
		{"crypto/rand", rule.CapIORandom}, // entrada própria vence o prefixo
		{"os", rule.CapIOFilesystem},
		{"encoding/json", rule.CapWireCodec},
		{"log/slog", rule.CapObservability},
		{"time", rule.CapIOClock},
		{"errors", rule.CapPure},
	}
	for _, c := range casos {
		got, ok := rule.StdlibCapability(c.pkg)
		if !ok {
			t.Errorf("%s sem capability na tabela", c.pkg)
			continue
		}
		if got != c.want {
			t.Errorf("%s = %s, esperado %s", c.pkg, got, c.want)
		}
	}

	if _, ok := rule.StdlibCapability("pacote/que/nao/existe"); ok {
		t.Error("package desconhecido resolvido: a tabela não é fail-closed")
	}
}

// --------------------------------------------------------- E001 / E002 ---

func endpoint(b rule.Block) rule.Endpoint {
	return rule.Endpoint{CanonicalKey: "m/u", Block: b, BoundedContext: "bc"}
}

func stdlib(paths ...string) func(string) bool {
	return func(p string) bool { return slices.Contains(paths, p) }
}

func TestVetorE001(t *testing.T) {
	semAllowlist := rule.ExternalPolicy{}

	t.Run("positivo: domain importando builtin pure", func(t *testing.T) {
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u", "errors", "u/a.go",
			semAllowlist, nil, stdlib("errors"))
		exigeCodigos(t, ds)
	})

	t.Run("negativo: domain importando net/http", func(t *testing.T) {
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u", "net/http", "u/a.go",
			semAllowlist, nil, stdlib("net/http"))
		exigeCodigos(t, ds, rule.CodeE001)
	})

	t.Run("negativo: application importando SDK de broker", func(t *testing.T) {
		policy := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
			{Package: "github.com/exemplo/broker", Versions: "v1", Entrypoints: []string{"."}, Capability: rule.CapIOMessaging},
		}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockApplication), "u",
			"github.com/exemplo/broker", "u/a.go", policy, nil, stdlib())
		exigeCodigos(t, ds, rule.CodeE001)
	})

	t.Run("positivo: provider importando o mesmo SDK", func(t *testing.T) {
		policy := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
			{Package: "github.com/exemplo/broker", Versions: "v1", Entrypoints: []string{"."}, Capability: rule.CapIOMessaging},
		}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockProvider), "u",
			"github.com/exemplo/broker", "u/a.go", policy, nil, stdlib())
		exigeCodigos(t, ds)
	})

	t.Run("negativo: dependência sem capability declarada em bloco default deny", func(t *testing.T) {
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
			"github.com/desconhecido/x", "u/a.go", semAllowlist, nil, stdlib())
		exigeCodigos(t, ds, rule.CodeE001)
	})

	t.Run("positivo: mesma dependência em bloco permissivo", func(t *testing.T) {
		// Aceita qualquer capability, então não saber qual é não muda nada.
		ds := rule.EvaluateExternal(endpoint(rule.BlockApp), "u",
			"github.com/desconhecido/x", "u/a.go", semAllowlist, nil, stdlib())
		exigeCodigos(t, ds)
	})

	t.Run("negativo: entrypoint fora da allowlist", func(t *testing.T) {

		policy := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
			{Package: "github.com/exemplo/lib", Versions: "v1",
				Entrypoints: []string{"github.com/exemplo/lib/puro"}, Capability: rule.CapPure},
		}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
			"github.com/exemplo/lib/impuro", "u/a.go", policy, nil, stdlib())
		exigeCodigos(t, ds, rule.CodeE001)
	})

	t.Run("positivo: entrypoint declarado", func(t *testing.T) {
		policy := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
			{Package: "github.com/exemplo/lib", Versions: "v1",
				Entrypoints: []string{"github.com/exemplo/lib/puro"}, Capability: rule.CapPure},
		}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
			"github.com/exemplo/lib/puro", "u/a.go", policy, nil, stdlib())
		exigeCodigos(t, ds)
	})
}

func TestVetorE002(t *testing.T) {
	policy := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
		{Package: "github.com/exemplo/puro", Versions: "v1", Entrypoints: []string{"."}, Capability: rule.CapPure},
	}}

	t.Run("positivo: fechamento inteiramente pure", func(t *testing.T) {
		fechamento := func(p string) ([]string, bool) {
			return []string{"errors", "strings"}, p == "github.com/exemplo/puro"
		}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
			"github.com/exemplo/puro", "u/a.go", policy, fechamento, stdlib("errors", "strings"))
		exigeCodigos(t, ds)
	})

	t.Run("negativo: fechamento alcança io.network", func(t *testing.T) {
		fechamento := func(p string) ([]string, bool) {
			return []string{"errors", "net/http"}, p == "github.com/exemplo/puro"
		}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
			"github.com/exemplo/puro", "u/a.go", policy, fechamento, stdlib("errors", "net/http"))
		exigeCodigos(t, ds, rule.CodeE002)
	})

	t.Run("negativo: fechamento com dependência não classificada", func(t *testing.T) {

		fechamento := func(p string) ([]string, bool) {
			return []string{"github.com/opaco/x"}, p == "github.com/exemplo/puro"
		}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
			"github.com/exemplo/puro", "u/a.go", policy, fechamento, stdlib())
		exigeCodigos(t, ds, rule.CodeE002)
	})

	t.Run("builtin não sofre pureza transitiva", func(t *testing.T) {
		// Builtin recebe capability atribuída: descer o fechamento chegaria
		// em internal/abi e tornaria todo package puro impuro.
		fechamento := func(string) ([]string, bool) { return []string{"internal/abi"}, true }
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u", "errors", "u/a.go",
			rule.ExternalPolicy{}, fechamento, stdlib("errors"))
		exigeCodigos(t, ds)
	})
}

// ------------------------------------------------------ exceção nominal ---

func TestExcecaoNominal(t *testing.T) {
	nominal := rule.ExceptionEntry{
		Unit: "u", Dependency: "net/http", Reason: "adapter legado em migração",
		Owner: "plataforma", ReviewBy: "2026-12-31",
	}

	t.Run("positivo: exceção nominal autoriza o par declarado", func(t *testing.T) {
		policy := rule.ExternalPolicy{Exceptions: []rule.ExceptionEntry{nominal}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u", "net/http", "u/a.go",
			policy, nil, stdlib("net/http"))
		exigeCodigos(t, ds)
	})

	t.Run("negativo: exceção não cobre outra unidade", func(t *testing.T) {
		policy := rule.ExternalPolicy{Exceptions: []rule.ExceptionEntry{nominal}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "outra", "net/http", "u/a.go",
			policy, nil, stdlib("net/http"))
		exigeCodigos(t, ds, rule.CodeE001)
	})

	t.Run("negativo: exceção não cobre outra dependência", func(t *testing.T) {
		policy := rule.ExternalPolicy{Exceptions: []rule.ExceptionEntry{nominal}}
		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u", "os", "u/a.go",
			policy, nil, stdlib("os"))
		exigeCodigos(t, ds, rule.CodeE001)
	})

	t.Run("negativo: exceção incompleta não autoriza nem passa calada", func(t *testing.T) {
		incompleta := nominal
		incompleta.Owner = ""
		policy := rule.ExternalPolicy{Exceptions: []rule.ExceptionEntry{incompleta}}

		ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u", "net/http", "u/a.go",
			policy, nil, stdlib("net/http"))
		exigeCodigos(t, ds, rule.CodeE001)

		exigeCodigos(t, policy.Validate("m/dmpf-units.json"), rule.CodeM001)
	})
}

// ------------------------------------------------- allowlist completa ---

// Um default implícito para a raiz devolveria a autorização que a regra dos
// entrypoints retira do pacote com subpath impuro.
func TestEntrypointsVazioNaoAutorizaARaiz(t *testing.T) {
	semEntrypoints := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
		{Package: "github.com/exemplo/lib", Versions: "v1", Capability: rule.CapPure},
	}}
	ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
		"github.com/exemplo/lib", "u/a.go", semEntrypoints, nil, stdlib())
	exigeCodigos(t, ds, rule.CodeE001)

	comRaiz := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{
		{Package: "github.com/exemplo/lib", Versions: "v1",
			Entrypoints: []string{rule.EntrypointRoot}, Capability: rule.CapPure},
	}}
	exigeCodigos(t, rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
		"github.com/exemplo/lib", "u/a.go", comRaiz, nil, stdlib()))
}

// Sem os quatro elementos, autorizar seria autorizar por omissão.
func TestEntradaIncompletaNaoAutoriza(t *testing.T) {
	completa := rule.AllowlistEntry{
		Package: "github.com/exemplo/lib", Versions: "v1",
		Entrypoints: []string{rule.EntrypointRoot}, Capability: rule.CapPure,
	}

	casos := []struct {
		nome     string
		mutar    func(rule.AllowlistEntry) rule.AllowlistEntry
		completa bool
	}{
		{"completa", func(e rule.AllowlistEntry) rule.AllowlistEntry { return e }, true},
		{"sem versions", func(e rule.AllowlistEntry) rule.AllowlistEntry { e.Versions = ""; return e }, false},
		{"sem entrypoints", func(e rule.AllowlistEntry) rule.AllowlistEntry { e.Entrypoints = nil; return e }, false},
		{"sem package", func(e rule.AllowlistEntry) rule.AllowlistEntry { e.Package = ""; return e }, false},
		{"capability inválida", func(e rule.AllowlistEntry) rule.AllowlistEntry { e.Capability = "io"; return e }, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := c.mutar(completa)
			if e.Complete() != c.completa {
				t.Fatalf("Complete() = %v, esperado %v", e.Complete(), c.completa)
			}

			policy := rule.ExternalPolicy{Allowlist: []rule.AllowlistEntry{e}}
			ds := rule.EvaluateExternal(endpoint(rule.BlockDomain), "u",
				"github.com/exemplo/lib", "u/a.go", policy, nil, stdlib())

			if c.completa {
				exigeCodigos(t, ds)
				exigeCodigos(t, policy.Validate("m/dmpf-units.json"))
				return
			}

			exigeCodigos(t, ds, rule.CodeE001)

			exigeCodigos(t, policy.Validate("m/dmpf-units.json"), rule.CodeM001)
		})
	}
}
