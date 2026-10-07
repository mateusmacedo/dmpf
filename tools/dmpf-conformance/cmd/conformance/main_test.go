package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/manifest"
)

func TestInstanteDeAceitaSoRFC3339(t *testing.T) {
	got, err := instanteDe("2026-09-12T00:00:00Z")
	if err != nil {
		t.Fatalf("instanteDe: %v", err)
	}
	want := exception.Instant(time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC).UnixNano())
	if got != want {
		t.Errorf("instante %d, esperado %d", got, want)
	}
	if _, err := instanteDe("2026-09-12"); err == nil {
		t.Error("--now aceitou data sem hora")
	}
}

func TestSemProfilesNaoDependeDoLayoutDaRaiz(t *testing.T) {
	_, perfis, err := preparar(opcoes{raiz: t.TempDir()})
	if err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if perfis != "" {
		t.Errorf("perfis = %q, esperado vazio (perfis embutidos)", perfis)
	}
}

func documentoDeContrato(exceptions ...manifest.Exception) manifest.Document {
	return manifest.Document{
		Path: "m/dmpf-units.json", Module: "m", Schema: manifest.SchemaID,
		Units: []manifest.Unit{{
			ID: "u", Block: "contract", BoundedContext: "bc", Include: []string{"m/p"},
			PresentID: true, PresentBlock: true, PresentBoundedContext: true, PresentInclude: true,
		}},
		Exceptions: exceptions,
	}
}

func TestRegravacaoRecusaExcecaoNaoAdmitida(t *testing.T) {
	soLegado := manifest.Exception{
		Unit: "u", Dependency: "reflect", Reason: "legado", Owner: "team:plataforma", ReviewBy: "2027-01-01",
		Object:        manifest.ExceptionObject{Unit: "u", Identity: "reflect", PresentUnit: true, PresentIdentity: true},
		Justification: "legado", PresentJustification: true,
	}

	err := admitido(documentoDeContrato(soLegado), nil, 0)
	if err == nil || !strings.Contains(err.Error(), "exceção não admitida") {
		t.Fatalf("regravação aceitou exceção só com campos legados: %v", err)
	}
}

func TestRegravacaoAceitaManifestoSemExcecao(t *testing.T) {
	if err := admitido(documentoDeContrato(), nil, 0); err != nil {
		t.Fatalf("regravação recusou manifesto válido: %v", err)
	}
}

// O kernel fica FORA do repositório, como no consumidor real: dentro dele, o
// inventário o trataria como módulo local.
func repoConsumidor(t *testing.T, cenario string) string {
	t.Helper()
	base := t.TempDir()
	fixture := filepath.Join("..", "..", "internal", "golist", "testdata", cenario)
	for _, nome := range []string{"consumer", "kernel"} {
		origem := filepath.Join(fixture, nome)
		if _, err := os.Stat(origem); err != nil {
			continue
		}
		if err := os.CopyFS(filepath.Join(base, nome), os.DirFS(origem)); err != nil {
			t.Fatal(err)
		}
	}
	repo := filepath.Join(base, "consumer")
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestConsumidorDoKernelPorVersaoConformeSemGravarOKernel(t *testing.T) {
	repo := repoConsumidor(t, "kernelext")
	var saida, erros strings.Builder

	if code := run(opcoes{raiz: repo, regravar: true}, &saida, &erros); code != exitConforme {
		t.Fatalf("--write-baseline saiu %d: %s", code, erros.String())
	}
	raw, err := os.ReadFile(filepath.Join(repo, baseline.Path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "exemplo.test/kernel") {
		t.Errorf("baseline gravou o kernel recebido por versão:\n%s", raw)
	}

	saida.Reset()
	if code := run(opcoes{raiz: repo}, &saida, &erros); code != exitConforme {
		t.Fatalf("conformance saiu %d:\n%s%s", code, saida.String(), erros.String())
	}
}

func TestKernelForaDoCacheSemRedeSaiComFalha(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOTOOLCHAIN", "local")
	repo := repoConsumidor(t, "kernelausente")
	var saida, erros strings.Builder

	code := run(opcoes{raiz: repo}, &saida, &erros)
	if code != exitFalha || !strings.Contains(saida.String(), "NAO VERIFICADO") {
		t.Fatalf("exit %d, esperado %d com NAO VERIFICADO:\n%s%s", code, exitFalha, saida.String(), erros.String())
	}
}
