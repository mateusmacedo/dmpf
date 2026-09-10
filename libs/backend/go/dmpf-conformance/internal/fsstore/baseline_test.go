package fsstore_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/baseline"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/fsstore"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/internal/rule"
)

func escreverBaselineBruto(t *testing.T, raiz, conteudo string) {
	t.Helper()
	caminho := filepath.Join(raiz, baseline.Path)
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatalf("preparar diretório: %v", err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatalf("escrever baseline de teste: %v", err)
	}
}

func TestBaselineSharedKernelUnitsAusente(t *testing.T) {
	raiz := t.TempDir()
	escreverBaselineBruto(t, raiz, `{"schema":"dmpf/units-baseline@1","digest":"x","entries":[]}`)

	doc, ok, err := fsstore.NewBaselineStore(raiz).Baseline()
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}
	if !ok {
		t.Fatal("baseline deveria existir")
	}
	if doc.HasSharedKernelUnits {
		t.Error("chave ausente deveria manter Has=false")
	}
	if doc.SharedKernelUnits != nil {
		t.Errorf("chave ausente deveria manter lista nil, got %v", doc.SharedKernelUnits)
	}
}

func TestBaselineSharedKernelUnitsVazia(t *testing.T) {
	raiz := t.TempDir()
	escreverBaselineBruto(t, raiz, `{"schema":"dmpf/units-baseline@1","digest":"x","entries":[],"shared_kernel_units":[]}`)

	doc, ok, err := fsstore.NewBaselineStore(raiz).Baseline()
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}
	if !ok {
		t.Fatal("baseline deveria existir")
	}
	if !doc.HasSharedKernelUnits {
		t.Error("lista vazia presente deveria marcar Has=true")
	}
	if len(doc.SharedKernelUnits) != 0 {
		t.Errorf("lista deveria ser vazia, got %v", doc.SharedKernelUnits)
	}
}

func TestBaselineSharedKernelUnitsComItens(t *testing.T) {
	raiz := t.TempDir()
	escreverBaselineBruto(t, raiz,
		`{"schema":"dmpf/units-baseline@1","digest":"x","entries":[],"shared_kernel_units":["orders","reservations"]}`)

	doc, ok, err := fsstore.NewBaselineStore(raiz).Baseline()
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}
	if !ok {
		t.Fatal("baseline deveria existir")
	}
	if !doc.HasSharedKernelUnits {
		t.Error("lista com itens deveria marcar Has=true")
	}
	if !slices.Equal(doc.SharedKernelUnits, []string{"orders", "reservations"}) {
		t.Errorf("lista incorreta: %v", doc.SharedKernelUnits)
	}
}

func TestBaselineSharedKernelUnitsNullReprova(t *testing.T) {
	raiz := t.TempDir()
	escreverBaselineBruto(t, raiz, `{"schema":"dmpf/units-baseline@1","digest":"x","entries":[],"shared_kernel_units":null}`)

	_, _, err := fsstore.NewBaselineStore(raiz).Baseline()
	if err == nil {
		t.Fatal("shared_kernel_units:null deveria reprovar a decodificação")
	}
}

func TestEscreverNormalizaListaVaziaParaColchetes(t *testing.T) {
	raiz := t.TempDir()
	store := fsstore.NewBaselineStore(raiz)

	doc := baseline.Document{Schema: baseline.SchemaID, Entries: []baseline.Entry{}}
	doc.Digest = baseline.Digest(doc.Entries)
	if err := store.Escrever(doc); err != nil {
		t.Fatalf("Escrever: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(raiz, baseline.Path))
	if err != nil {
		t.Fatalf("ler arquivo escrito: %v", err)
	}
	if !strings.Contains(string(raw), `"shared_kernel_units": []`) {
		t.Errorf("Escrever não normalizou a lista para colchetes: %s", raw)
	}

	relido, ok, err := store.Baseline()
	if err != nil || !ok {
		t.Fatalf("Baseline após Escrever: ok=%v err=%v", ok, err)
	}
	if !relido.HasSharedKernelUnits {
		t.Error("round-trip perdeu a presença da chave")
	}
}

func TestEscreverPreservaListaComItens(t *testing.T) {
	raiz := t.TempDir()
	store := fsstore.NewBaselineStore(raiz)

	doc := baseline.Document{
		Schema:               baseline.SchemaID,
		Entries:              []baseline.Entry{},
		SharedKernelUnits:    []string{"orders"},
		HasSharedKernelUnits: true,
	}
	doc.Digest = baseline.Digest(doc.Entries)
	if err := store.Escrever(doc); err != nil {
		t.Fatalf("Escrever: %v", err)
	}

	relido, ok, err := store.Baseline()
	if err != nil || !ok {
		t.Fatalf("Baseline após Escrever: ok=%v err=%v", ok, err)
	}
	if !slices.Equal(relido.SharedKernelUnits, []string{"orders"}) {
		t.Errorf("round-trip alterou a lista: %v", relido.SharedKernelUnits)
	}
}

func rodarGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestBaselineEmUsaOMesmoDecodificadorDaBaseline: BaselineEm lê via `git show`
// em vez de os.ReadFile, então tem seu próprio ponto de Unmarshal — sem este
// teste, o wire de shared_kernel_units poderia divergir entre os dois.
func TestBaselineEmUsaOMesmoDecodificadorDaBaseline(t *testing.T) {
	raiz := t.TempDir()
	rodarGit(t, raiz, "init", "-q")
	rodarGit(t, raiz, "config", "user.email", "test@test.com")
	rodarGit(t, raiz, "config", "user.name", "test")

	escreverBaselineBruto(t, raiz,
		`{"schema":"dmpf/units-baseline@1","digest":"x","entries":[],"shared_kernel_units":["u"]}`)
	rodarGit(t, raiz, "add", ".")
	rodarGit(t, raiz, "commit", "-q", "-m", "baseline")

	doc, ok, err := fsstore.NewBaselineStore(raiz).BaselineEm("HEAD")
	if err != nil {
		t.Fatalf("BaselineEm: %v", err)
	}
	if !ok {
		t.Fatal("baseline não encontrado no ref")
	}
	if !doc.HasSharedKernelUnits || !slices.Equal(doc.SharedKernelUnits, []string{"u"}) {
		t.Errorf("BaselineEm não decodificou shared_kernel_units: %+v", doc)
	}
}

// Sem isso, main.go voltaria a reabrir o achado da Fase 1: Escrever normaliza
// a lista para `[]` e a releitura fecha Has=true contra um digest legado.
func TestRegravarFechaORoundTripSemDivergencia(t *testing.T) {
	raiz := t.TempDir()
	store := fsstore.NewBaselineStore(raiz)

	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	membership := map[rule.UnitKey][]string{{Module: "m", ID: "u"}: {"m/a"}}

	if err := store.Escrever(baseline.Regravar(baseline.Document{}, false, units, membership)); err != nil {
		t.Fatalf("Escrever: %v", err)
	}

	relido, ok, err := store.Baseline()
	if err != nil || !ok {
		t.Fatalf("Baseline após Escrever: ok=%v err=%v", ok, err)
	}
	if ds := baseline.Compare(relido, baseline.FromUniverse(units, membership)); len(ds) != 0 {
		t.Fatalf("round-trip via Regravar divergiu: %v", ds)
	}
}

// Controle negativo do teste acima: sem Regravar, o mesmo round-trip diverge
// em exatamente 1 T001 — Escrever normaliza a lista para `[]` e Baseline volta
// com Has=true, que não fecha com o digest legado que FromUniverse gravou.
func TestFromUniverseSemRegravarDivergeNoRoundTripControleNegativo(t *testing.T) {
	raiz := t.TempDir()
	store := fsstore.NewBaselineStore(raiz)

	units := []rule.Unit{{ID: "u", Module: "m", Block: rule.BlockDomain, BoundedContext: "bc"}}
	membership := map[rule.UnitKey][]string{{Module: "m", ID: "u"}: {"m/a"}}

	if err := store.Escrever(baseline.FromUniverse(units, membership)); err != nil {
		t.Fatalf("Escrever: %v", err)
	}

	relido, ok, err := store.Baseline()
	if err != nil || !ok {
		t.Fatalf("Baseline após Escrever: ok=%v err=%v", ok, err)
	}
	ds := baseline.Compare(relido, baseline.FromUniverse(units, membership))
	if len(ds) != 1 || ds[0].Code != rule.CodeT001 {
		t.Fatalf("esperado exatamente 1 DMPF-T001 (controle negativo), got %v", ds)
	}
}
