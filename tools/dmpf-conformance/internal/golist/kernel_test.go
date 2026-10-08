package golist_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/fsstore"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/golist"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

var perfilLinuxAmd64 = []fsstore.BuildProfile{
	{ID: "linux-amd64", GOOS: "linux", GOARCH: "amd64", CGOEnabled: false},
}

func consumidor(t *testing.T, cenario string) *golist.Source {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", cenario, "consumer"))
	if err != nil {
		t.Fatal(err)
	}
	return consumidorEm(dir)
}

func consumidorEm(dir string) *golist.Source {
	modules := []rule.Module{{Path: "exemplo.test/consumer", Dir: dir, HasManifest: true, HasProduction: true}}
	return golist.New("", modules, perfilLinuxAmd64)
}

func TestKernelPorVersaoTrazManifestoEPacotes(t *testing.T) {
	src := consumidor(t, "kernelext")

	k, err := src.Kernel()
	if err != nil {
		t.Fatalf("Kernel: %v", err)
	}
	if len(k.Documents) != 1 {
		t.Fatalf("documentos = %+v, esperado só o do kernel", k.Documents)
	}
	doc := k.Documents[0]
	if doc.Module != "exemplo.test/kernel" || doc.Path != "exemplo.test/kernel@v1.2.3/dmpf-units.json" {
		t.Errorf("documento de %q em %q", doc.Module, doc.Path)
	}
	if len(doc.Units) != 1 || doc.Units[0].ID != "kernel/domain" {
		t.Errorf("unidades = %+v", doc.Units)
	}
	want := []rule.Package{{CanonicalKey: "exemplo.test/kernel/domain", Module: "exemplo.test/kernel"}}
	if !slices.Equal(k.Packages, want) {
		t.Errorf("pacotes = %+v, esperado %+v", k.Packages, want)
	}

	locais, err := src.Packages()
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	for _, p := range locais {
		if p.Module != "exemplo.test/consumer" {
			t.Errorf("Packages() devolveu %s: o kernel entraria na regravação do baseline", p.CanonicalKey)
		}
	}
}

func TestKernelLidoComVendorNoConsumidor(t *testing.T) {
	t.Setenv("GOWORK", "off")
	base := t.TempDir()
	if err := os.CopyFS(base, os.DirFS(filepath.Join("testdata", "kernelext"))); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "consumer")
	vendor := exec.Command("go", "mod", "vendor")
	vendor.Dir = dir
	if out, err := vendor.CombinedOutput(); err != nil {
		t.Fatalf("go mod vendor: %v\n%s", err, out)
	}

	k, err := consumidorEm(dir).Kernel()
	if err != nil {
		t.Fatalf("Kernel com vendor/: %v", err)
	}
	if len(k.Documents) != 1 || k.Documents[0].Module != "exemplo.test/kernel" {
		t.Fatalf("documentos = %+v, esperado o manifesto do kernel", k.Documents)
	}
}

// O manifesto inválido nasce no teste: versionado, ele reprovaria o formatador.
func TestManifestoInvalidoDoKernelCitaModuloEVersao(t *testing.T) {
	base := t.TempDir()
	if err := os.CopyFS(base, os.DirFS(filepath.Join("testdata", "kernelext"))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "kernel", "dmpf-units.json"), []byte(`{ "schema": "dmpf/units@1", "units": [`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := consumidorEm(filepath.Join(base, "consumer")).Kernel()
	if err == nil || !strings.Contains(err.Error(), "exemplo.test/kernel@v1.2.3/dmpf-units.json") {
		t.Fatalf("erro = %v, esperado citando exemplo.test/kernel@v1.2.3/dmpf-units.json", err)
	}
}

func TestKernelForaDoCacheSemRedeNaoViraKernelVazio(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOTOOLCHAIN", "local")

	_, err := consumidor(t, "kernelausente").Kernel()
	if err == nil || !strings.Contains(err.Error(), "exemplo.test/ausente@v1.0.0") {
		t.Fatalf("erro = %v, esperado citando exemplo.test/ausente@v1.0.0", err)
	}
}
