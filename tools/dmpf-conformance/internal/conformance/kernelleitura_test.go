package conformance_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/baseline"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/conformance"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/manifest"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/port"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

type kernelFake struct {
	k   port.Kernel
	err error
}

func (f kernelFake) Kernel() (port.Kernel, error) { return f.k, f.err }

var _ port.KernelSource = kernelFake{}

func kernelPorVersao() port.Kernel {
	doc := documentoDoModulo("kernel",
		unidadeManifesto{id: "kernel-domain", block: "domain", bc: "kernel", include: "kernel/domain"})
	doc.Path = "kernel@v1.0.0/dmpf-units.json"
	return port.Kernel{
		Documents: []manifest.Document{doc},
		Packages: []rule.Package{
			{CanonicalKey: "kernel/domain", Module: "kernel"},
			{CanonicalKey: "kernel/semunidade", Module: "kernel"},
		},
	}
}

func baselineSoDoConsumidor() baseline.Document {
	d := baseline.Document{Schema: baseline.SchemaID, Entries: []baseline.Entry{
		{Unit: "x-domain", Module: "x", Block: "domain", BoundedContext: "x", Membership: []string{"x/domain"}},
	}}
	d.Digest = baseline.DigestOf(d)
	return d
}

func consumidorDoKernel(edges ...port.Edge) conformance.Input {
	return conformance.Input{
		Modules: []rule.Module{{Path: "x", HasManifest: true, HasProduction: true}},
		Manifests: manifestoModulos{docs: []manifest.Document{
			documentoDoModulo("x", unidadeManifesto{id: "x-domain", block: "domain", bc: "x", include: "x/domain"}),
		}},
		Graph:    grafoSK{pkgs: []rule.Package{{CanonicalKey: "x/domain", Module: "x"}}, edges: edges},
		Baseline: storeSK{doc: baselineSoDoConsumidor(), existe: true},
		Kernel:   kernelFake{k: kernelPorVersao()},
	}
}

var arestaParaOKernel = port.Edge{From: "x/domain", To: "kernel/domain", SourceFile: "x/domain/d.go"}

func TestKernelPorVersaoEhSharedKernelDeLeitura(t *testing.T) {
	rel, err := conformance.Check(consumidorDoKernel(arestaParaOKernel))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if rel.Reprovado() {
		t.Fatalf("consumidor do kernel por versão reprovou: %v %v", rel.Diagnostics, rel.NaoVerificado)
	}
}

func TestSemKernelSourceOKernelSegueExterno(t *testing.T) {
	in := consumidorDoKernel(arestaParaOKernel)
	in.Kernel = nil

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Contains(codigos(rel.Diagnostics), rule.CodeE001) {
		t.Fatalf("sem KernelSource o kernel deixou de ser externo: %v", rel.Diagnostics)
	}
}

func TestSharedKernelDeLeituraDispensaDesignacaoSemBaseline(t *testing.T) {
	in := consumidorDoKernel(arestaParaOKernel)
	in.Baseline = nil

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rel.Diagnostics) != 0 {
		t.Fatalf("D002 sem designação manual: %v", rel.Diagnostics)
	}
}

func TestArestaParaPackageDoKernelSemUnidadeEhU001(t *testing.T) {
	rel, err := conformance.Check(consumidorDoKernel(
		port.Edge{From: "x/domain", To: "kernel/semunidade", SourceFile: "x/domain/d.go"}))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rel.Diagnostics) != 1 || rel.Diagnostics[0].Code != rule.CodeU001 ||
		rel.Diagnostics[0].CanonicalKey != "kernel/semunidade" {
		t.Fatalf("diagnósticos = %v, esperado um U001 sobre kernel/semunidade", rel.Diagnostics)
	}
}

func TestPackageDoKernelSemUnidadeENaoImportadoNaoReprova(t *testing.T) {
	rel, err := conformance.Check(consumidorDoKernel())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if rel.Reprovado() {
		t.Fatalf("cobertura do kernel verificada no consumidor: %v", rel.Diagnostics)
	}
}

func TestKernelIlegivelEhFalhaNaoVerificada(t *testing.T) {
	in := consumidorDoKernel(arestaParaOKernel)
	in.Kernel = kernelFake{err: errors.New("build list não carregada pelo toolchain: kernel@v1.0.0: module lookup disabled by GOPROXY=off")}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !rel.Falha || len(rel.NaoVerificado) == 0 || !strings.Contains(rel.NaoVerificado[0], "kernel@v1.0.0") {
		t.Fatalf("falha = %v, não verificado = %v", rel.Falha, rel.NaoVerificado)
	}
}

func TestManifestoDoKernelInvalidoEhFalhaNaoVerificada(t *testing.T) {
	k := kernelPorVersao()
	k.Documents[0].Units[0].Block = "inexistente"
	in := consumidorDoKernel(arestaParaOKernel)
	in.Kernel = kernelFake{k: k}

	rel, err := conformance.Check(in)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !rel.Falha || len(rel.NaoVerificado) == 0 || !strings.Contains(rel.NaoVerificado[0], "kernel@v1.0.0/dmpf-units.json") {
		t.Fatalf("falha = %v, não verificado = %v", rel.Falha, rel.NaoVerificado)
	}
}
