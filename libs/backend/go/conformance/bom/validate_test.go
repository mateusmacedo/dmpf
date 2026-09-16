package bom

import (
	"cmp"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

const (
	workspace     = "testdata/workspace"
	arquivoValido = "bom/dmpf/0.1.0.json"
	agoraFixo     = "2026-09-12T00:00:00Z"
)

func instante(t *testing.T, s string) exception.Instant {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return exception.Instant(tm.UnixNano())
}

func documentoValido(t *testing.T) Document {
	t.Helper()
	raw, err := os.ReadFile(workspace + "/" + arquivoValido)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func validarEm(t *testing.T, doc Document, arquivo string, base *Document) []rule.Diagnostic {
	t.Helper()
	ds, err := Validate(doc, Input{File: arquivo, Now: instante(t, agoraFixo), Root: os.DirFS(workspace), Base: base})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return ds
}

func listar(ds []rule.Diagnostic) string {
	linhas := make([]string, 0, len(ds))
	for _, d := range ds {
		linhas = append(linhas, d.String())
	}
	return strings.Join(linhas, "\n")
}

func temCodigo(ds []rule.Diagnostic, c rule.Code) bool {
	return slices.ContainsFunc(ds, func(d rule.Diagnostic) bool { return d.Code == c })
}

func exigeCodigos(t *testing.T, ds []rule.Diagnostic, want ...rule.Code) {
	t.Helper()
	got := make([]rule.Code, 0, len(ds))
	for _, d := range ds {
		got = append(got, d.Code)
	}
	slices.Sort(got)
	got = slices.Compact(got)
	want = slices.Clone(want)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("códigos %v, esperado exatamente %v:\n%s", got, want, listar(ds))
	}
}

func runtimeDe(d *Document) *Entry { return &d.Sections[0].Entries[0] }

func driverDe(d *Document) *Entry { return &d.Sections[2].Entries[0] }

func excecaoE1NoBOM() exception.Exception {
	return exception.Exception{
		ID: "X-bom-e1", ADR: "ADR-041", Owner: "team:plataforma", Justification: "E1 declarada no registro errado",
		Object: exception.Object{
			Kind: exception.KindExternalDependency, Unit: "u", Identity: "example.com/sdk",
			PresentKind: true, PresentUnit: true, PresentIdentity: true,
		},
		Convergence: exception.Convergence{Kind: exception.ConvergencePlan, Deadline: 1, Condition: "sdk certificado", PresentKind: true},
		History:     []exception.HistoryEntry{{Event: exception.EventGranted, At: 1, By: "team:plataforma"}},
		ValidUntil:  exception.Instant(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()),
		ReviewBy:    exception.Instant(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()),

		PresentID: true, PresentADR: true, PresentOwner: true, PresentJustification: true,
		PresentConvergence: true, PresentHistory: true, PresentValidUntil: true, PresentReviewBy: true,
	}
}

func TestBOMValidoPassa(t *testing.T) {
	if ds := validarEm(t, documentoValido(t), arquivoValido, nil); len(ds) > 0 {
		t.Fatalf("BOM válido reprovou:\n%s", listar(ds))
	}
}

func TestUmVetorNegativoPorCodigo(t *testing.T) {
	casos := []struct {
		nome    string
		arquivo string
		mudar   func(d *Document)
		codigos []rule.Code
	}{
		{nome: "B001 seção ausente", mudar: func(d *Document) { d.Sections[4].Present = false }, codigos: []rule.Code{rule.CodeB001}},
		{nome: "B001 seção vazia sem reason", mudar: func(d *Document) { d.Sections[4].Reason = "" }, codigos: []rule.Code{rule.CodeB001}},
		{nome: "B002 version com faixa", mudar: func(d *Document) { driverDe(d).Version = ">=5.10 <6" }, codigos: []rule.Code{rule.CodeB002}},
		{nome: "B002 state fora do conjunto", mudar: func(d *Document) { driverDe(d).State = "aprovada" }, codigos: []rule.Code{rule.CodeB002}},
		{nome: "B003 depreciada sem successor", mudar: func(d *Document) {
			driverDe(d).State, driverDe(d).DeprecatedAt = StateDepreciada, "2026-09-01"
		}, codigos: []rule.Code{rule.CodeB003}},
		{nome: "B004 certificada sem evidence_digest, que desancora a evidência", mudar: func(d *Document) { runtimeDe(d).EvidenceDigest = "" }, codigos: []rule.Code{rule.CodeB004, rule.CodeB006}},
		{nome: "B004 certificada sem certified_at", mudar: func(d *Document) { runtimeDe(d).CertifiedAt = "" }, codigos: []rule.Code{rule.CodeB004}},
		{nome: "B005 digest divergente", mudar: func(d *Document) {
			runtimeDe(d).EvidenceDigest = "sha256:" + strings.Repeat("0", 64)
		}, codigos: []rule.Code{rule.CodeB005}},
		{nome: "B006 compatible_with sem execução", mudar: func(d *Document) {
			runtimeDe(d).CompatibleWith[0].Version = "v5.9.0"
		}, codigos: []rule.Code{rule.CodeB006}},
		{nome: "B007 version diverge do registro", mudar: func(d *Document) { d.Semconv.Version = "v1.44.0" }, codigos: []rule.Code{rule.CodeB007}},
		{nome: "B007 runtime sem registry_ref", mudar: func(d *Document) { runtimeDe(d).RegistryRef = nil }, codigos: []rule.Code{rule.CodeB007}},
		{nome: "B008 certificação vencida", mudar: func(d *Document) { runtimeDe(d).ValidUntil = "2026-09-11" }, codigos: []rule.Code{rule.CodeB008}},
		{nome: "B009 cve ausente", mudar: func(d *Document) { driverDe(d).PresentCVE = false }, codigos: []rule.Code{rule.CodeB009}},
		{nome: "B009 CVE aberta sem owner", mudar: func(d *Document) {
			driverDe(d).CVE = []CVE{{ID: "CVE-2026-0001", State: CVEAberta}}
		}, codigos: []rule.Code{rule.CodeB009}},
		{nome: "B010 metrics divergente", mudar: func(d *Document) { d.Metrics.Vigentes = 1 }, codigos: []rule.Code{rule.CodeB010}},
		{nome: "B011 tag de outra release", mudar: func(d *Document) { d.Tag = "dmpf@0.2.0" }, codigos: []rule.Code{rule.CodeB011}},
		{nome: "B011 release diferente do arquivo", arquivo: "bom/dmpf/0.2.0.json", mudar: func(*Document) {}, codigos: []rule.Code{rule.CodeB011}},
		{nome: "X007 exceção E1 no BOM", mudar: func(d *Document) {
			d.Exceptions = []exception.Exception{excecaoE1NoBOM()}
			d.Metrics = Metrics{Vigentes: 1, Renovacoes: map[string]int{"X-bom-e1": 0}}
		}, codigos: []rule.Code{rule.CodeX007}},
		{nome: "B002 version não exata", mudar: func(d *Document) { driverDe(d).Version = "latest" }, codigos: []rule.Code{rule.CodeB002}},
		{nome: "B002 entrada repetida", mudar: func(d *Document) {
			d.Sections[2].Entries = append(d.Sections[2].Entries, *driverDe(d))
		}, codigos: []rule.Code{rule.CodeB002}},
		{nome: "B006 própria entrada fora da evidência", mudar: func(d *Document) {
			e := driverDe(d)
			e.Version = "v5.9.0"
			e.CompatibleWith = []Compatibility{{Identity: "go", Version: "1.26.6", Evidence: "provider"}}
		}, codigos: []rule.Code{rule.CodeB006}},
		{nome: "B006 evidência sem digest que a ancore", mudar: func(d *Document) {
			runtimeDe(d).CompatibleWith[0].Evidence = "forjada"
		}, codigos: []rule.Code{rule.CodeB006}},
		{nome: "X001 id de exceção repetido", mudar: func(d *Document) {
			e2 := excecaoE1NoBOM()
			e2.Object.Kind = exception.KindBOMCombination
			d.Exceptions = []exception.Exception{e2, e2}
			d.Metrics = Metrics{Vigentes: 2, Renovacoes: map[string]int{"X-bom-e1": 0}}
		}, codigos: []rule.Code{rule.CodeX001}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			doc := documentoValido(t)
			c.mudar(&doc)
			arquivo := cmp.Or(c.arquivo, arquivoValido)
			exigeCodigos(t, validarEm(t, doc, arquivo, nil), c.codigos...)
		})
	}
}

func TestCertificacaoVencidaRebaixadaParaCandidataPassa(t *testing.T) {
	doc := documentoValido(t)
	runtimeDe(&doc).ValidUntil = "2026-09-11"
	exigeCodigos(t, validarEm(t, doc, arquivoValido, nil), rule.CodeB008)

	runtimeDe(&doc).State = StateCandidata
	if ds := validarEm(t, doc, arquivoValido, nil); len(ds) > 0 {
		t.Fatalf("o rebaixamento declarado ainda reprova:\n%s", listar(ds))
	}
}

func TestRelatorioDeterministicoOrdenadoPorCodigo(t *testing.T) {
	doc := documentoValido(t)
	doc.Tag = "dmpf@9.9.9"
	doc.Sections[1].Present = false
	driverDe(&doc).Version = "^5"

	primeiro := validarEm(t, doc, arquivoValido, nil)
	segundo := validarEm(t, doc, arquivoValido, nil)
	if len(primeiro) < 3 {
		t.Fatalf("vetor perdeu a força:\n%s", listar(primeiro))
	}
	if !slices.Equal(primeiro, segundo) {
		t.Fatalf("dois relatórios com o mesmo --now divergem:\n%s\n---\n%s", listar(primeiro), listar(segundo))
	}
	if !slices.IsSortedFunc(primeiro, func(a, b rule.Diagnostic) int { return cmp.Compare(a.Code, b.Code) }) {
		t.Fatalf("relatório fora da ordem por código:\n%s", listar(primeiro))
	}
}

func TestValidUntilSemHoraValeODiaInteiro(t *testing.T) {
	doc := documentoValido(t)
	runtimeDe(&doc).ValidUntil = "2026-09-12"
	ds, err := Validate(doc, Input{File: arquivoValido, Now: instante(t, "2026-09-12T12:00:00Z"), Root: os.DirFS(workspace)})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if temCodigo(ds, rule.CodeB008) {
		t.Fatalf("certificação válida até o fim do dia vencida ao meio-dia:\n%s", listar(ds))
	}
}
