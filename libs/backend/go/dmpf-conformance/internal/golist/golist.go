// Package golist é bloco `provider`: extrai o grafo de packages pelo toolchain.
//
// A aresta canônica em Go é package → package (RFC §3.3), e a resolução é a do
// compilador, nunca o texto do import (RFC §3.5). Por isso `go list` e não
// `go/parser`: parsear daria o que está escrito no arquivo, que é exatamente o
// que a RFC proíbe como base da decisão.
package golist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/fsstore"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/port"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

// listPackage é o subconjunto do `go list -json` que o verificador consome.
type listPackage struct {
	ImportPath     string            `json:"ImportPath"`
	Standard       bool              `json:"Standard"`
	Dir            string            `json:"Dir"`
	GoFiles        []string          `json:"GoFiles"`
	CgoFiles       []string          `json:"CgoFiles"`
	IgnoredGoFiles []string          `json:"IgnoredGoFiles"`
	Imports        []string          `json:"Imports"`
	ImportMap      map[string]string `json:"ImportMap"`
	Deps           []string          `json:"Deps"`
	Incomplete     bool              `json:"Incomplete"`
	Error          *listError        `json:"Error"`
	DepsErrors     []listError       `json:"DepsErrors"`
	Module         *listModule       `json:"Module"`
}

type listError struct {
	Err string `json:"Err"`
}

type listModule struct {
	Path string `json:"Path"`
}

// Source extrai o grafo por perfil de produção e devolve a união das arestas.
// Arquivo sob build tag que nenhum perfil ativa fica de fora — é a definição
// operacional de "build tag não usada em produção" (RFC §10.3).
type Source struct {
	root     string
	modules  []rule.Module
	profiles []fsstore.BuildProfile

	carregado bool
	pacotes   map[string]listPackage
	arestas   []port.Edge
	closure   map[string][]string
	imports   map[string]map[string]bool
}

func New(root string, modules []rule.Module, profiles []fsstore.BuildProfile) *Source {
	return &Source{root: root, modules: modules, profiles: profiles}
}

var _ port.GraphSource = (*Source)(nil)

// Packages devolve os packages de produção dos módulos do inventário.
func (s *Source) Packages() ([]rule.Package, error) {
	if err := s.carregar(); err != nil {
		return nil, err
	}
	var out []rule.Package
	for _, p := range s.pacotes {
		if p.Standard || p.Module == nil || !s.moduloDoInventario(p.Module.Path) {
			continue
		}
		// `go list` separa GoFiles de CgoFiles: um package composto só de
		// arquivos que importam "C" tem GoFiles vazio e continua sendo produção
		// num perfil com CGO_ENABLED=1. Olhar só GoFiles o faria sumir do
		// universo — e sumir é a única forma de escapar do U001.
		if len(p.arquivosDeProducao()) == 0 {
			continue
		}
		out = append(out, rule.Package{CanonicalKey: p.ImportPath, Module: p.Module.Path})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CanonicalKey < out[b].CanonicalKey })
	return out, nil
}

// Edges devolve as arestas diretas, já resolvidas pelo toolchain.
func (s *Source) Edges() ([]port.Edge, error) {
	if err := s.carregar(); err != nil {
		return nil, err
	}
	return s.arestas, nil
}

// Closure devolve o fechamento transitivo de um package (o campo Deps do
// `go list`). Serve APENAS à avaliação de pureza (DMPF-E002) — nunca à decisão
// de aresta, que é sempre sobre a aresta direta.
func (s *Source) Closure(canonicalKey string) ([]string, bool) {
	deps, ok := s.closure[canonicalKey]
	return deps, ok
}

// IsStandard reporta se o package é da biblioteca padrão. Builtins recebem
// capability como qualquer outra dependência (RFC §6.3).
func (s *Source) IsStandard(canonicalKey string) bool {
	p, ok := s.pacotes[canonicalKey]
	return ok && p.Standard
}

// gowork devolve o valor de GOWORK para o módulo, conforme a origem dele.
func (s *Source) gowork(m rule.Module) string {
	if m.WorkspaceMember && s.root != "" {
		return filepath.Join(s.root, "go.work")
	}
	return "off"
}

func (s *Source) moduloDoInventario(path string) bool {
	for _, m := range s.modules {
		if m.Path == path {
			return true
		}
	}
	return false
}

func (s *Source) carregar() error {
	if s.carregado {
		return nil
	}
	s.pacotes = map[string]listPackage{}
	s.closure = map[string][]string{}
	arestas := map[[3]string]port.Edge{}

	for _, perfil := range s.profiles {
		for _, m := range s.modules {
			pkgs, err := s.listar(perfil, m)
			if err != nil {
				return err
			}
			for _, p := range pkgs {
				s.absorver(p)
			}
		}
	}

	// As arestas são montadas depois de absorver TODOS os perfis, para que a
	// união seja sobre o conjunto completo de packages.
	for _, p := range s.pacotes {
		if p.Standard || p.Module == nil || !s.moduloDoInventario(p.Module.Path) {
			continue
		}
		for _, imp := range p.Imports {
			alvo := imp
			if mapeado, ok := p.ImportMap[imp]; ok && mapeado != "" {
				// ImportMap é a reescrita do toolchain (vendor, replace). O
				// destino real é o mapeado — alias e caminhos distintos para o
				// mesmo package produzem a MESMA aresta (RFC §3.5).
				alvo = mapeado
			}
			e := port.Edge{
				From:       p.ImportPath,
				To:         alvo,
				SourceFile: s.arquivoDoImport(p, imp),
				Unresolved: s.naoResolvido(alvo),
			}
			if e.Unresolved {
				e.Detail = s.motivoDoErro(alvo)
			}
			arestas[[3]string{e.From, e.To, e.SourceFile}] = e
		}
	}

	s.arestas = make([]port.Edge, 0, len(arestas))
	for _, e := range arestas {
		s.arestas = append(s.arestas, e)
	}
	sort.Slice(s.arestas, func(a, b int) bool {
		if s.arestas[a].From != s.arestas[b].From {
			return s.arestas[a].From < s.arestas[b].From
		}
		return s.arestas[a].To < s.arestas[b].To
	})

	s.carregado = true
	return nil
}

// naoResolvido reporta a condição que emite DMPF-E003.
//
// `Incomplete` OU `Error`, como a spec exige — sem a conjunção com "nenhum
// arquivo", que deixava passar o package que tem fonte E tem erro. O `-e` é
// transporte estruturado de erro, não aceitação de universo parcial: o import
// nunca é tratado como ausente, e a política fail-closed é do consumidor.
func (s *Source) naoResolvido(canonicalKey string) bool {
	p, ok := s.pacotes[canonicalKey]
	if !ok {
		return true
	}
	return p.Error != nil || p.Incomplete
}

// arquivosDeProducao são os arquivos que compõem o package no perfil, com as
// exclusões fechadas de RFC §10.3 já aplicadas pelo próprio toolchain
// (`_test.go`, `testdata/`, `vendor/` e tags não ativadas ficam de fora).
func (p listPackage) arquivosDeProducao() []string {
	out := make([]string, 0, len(p.GoFiles)+len(p.CgoFiles))
	out = append(out, p.GoFiles...)
	out = append(out, p.CgoFiles...)
	return out
}

// motivoDoErro devolve o texto do erro do toolchain, para a mensagem.
func (s *Source) motivoDoErro(canonicalKey string) string {
	p, ok := s.pacotes[canonicalKey]
	switch {
	case !ok:
		return "package ausente da saída do go list"
	case p.Error != nil && p.Error.Err != "":
		return p.Error.Err
	case len(p.DepsErrors) > 0 && p.DepsErrors[0].Err != "":
		return p.DepsErrors[0].Err
	default:
		return "package marcado como incompleto pelo toolchain"
	}
}

// absorver acumula o package na união dos perfis. Um package visto em mais de
// um perfil tem os GoFiles e Imports unidos — o grafo é a união das arestas.
func (s *Source) absorver(p listPackage) {
	ja, existe := s.pacotes[p.ImportPath]
	if !existe {
		s.pacotes[p.ImportPath] = p
		if len(p.Deps) > 0 {
			s.closure[p.ImportPath] = p.Deps
		}
		return
	}
	ja.GoFiles = unir(ja.GoFiles, p.GoFiles)
	ja.CgoFiles = unir(ja.CgoFiles, p.CgoFiles)
	ja.Imports = unir(ja.Imports, p.Imports)
	ja.Deps = unir(ja.Deps, p.Deps)
	// Clonar, não aliasar: `ja.ImportMap = p.ImportMap` faria a união do
	// perfil seguinte mutar o map que pertence ao listPackage de outro perfil.
	if ja.ImportMap == nil {
		ja.ImportMap = maps.Clone(p.ImportMap)
	} else {
		for k, v := range p.ImportMap {
			ja.ImportMap[k] = v
		}
	}
	// Erro em qualquer perfil reprova: fail-closed não é média entre perfis.
	if p.Error != nil {
		ja.Error = p.Error
	}
	ja.Incomplete = ja.Incomplete || p.Incomplete
	s.pacotes[p.ImportPath] = ja
	s.closure[p.ImportPath] = ja.Deps
}

func (s *Source) listar(perfil fsstore.BuildProfile, m rule.Module) ([]listPackage, error) {
	args := []string{"list", "-e", "-deps", "-json"}
	if len(perfil.Tags) > 0 {
		args = append(args, "-tags", strings.Join(perfil.Tags, ","))
	}
	// O `--` separa flags de padrões. Sem ele, um `go.mod` com `module -version`
	// faz o `go list` interpretar o padrão como flag e responder "flag provided
	// but not defined" — erro real, mas ilegível. Com o separador, o próprio Go
	// responde "malformed module path: leading dash", que diz o que corrigir.
	args = append(args, "--", m.Path+"/...")

	cmd := exec.Command("go", args...)
	cmd.Dir = m.Dir
	cmd.Env = append(cmd.Environ(),
		"GOOS="+perfil.GOOS,
		"GOARCH="+perfil.GOARCH,
		"CGO_ENABLED="+booleanoDeAmbiente(perfil.CGOEnabled),
		// GOWORK é resolvido pela ORIGEM do módulo, nunca herdado do ambiente.
		//
		// Membro do go.work resolve NO workspace: é assim que o build o compila,
		// e um `require` sem `replace` local cairia na versão publicada — o
		// verificador analisaria um grafo que não é o que se constrói. Módulo
		// fora do go.work resolve isolado, pelo próprio go.mod. Nos dois casos o
		// valor é explícito, então o veredicto não depende de onde o binário foi
		// invocado.
		"GOWORK="+s.gowork(m),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("go list em %s (perfil %s): %w: %s",
			m.Path, perfil.ID, err, strings.TrimSpace(stderr.String()))
	}
	return decodificar(&stdout)
}

// decodificar lê o fluxo de objetos JSON concatenados que o `go list -json`
// emite — não é um array.
func decodificar(r io.Reader) ([]listPackage, error) {
	dec := json.NewDecoder(r)
	var out []listPackage
	for {
		var p listPackage
		if err := dec.Decode(&p); err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, fmt.Errorf("decodificar saída do go list: %w", err)
		}
		out = append(out, p)
	}
}

func booleanoDeAmbiente(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func unir(a, b []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		if !visto[s] {
			visto[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
