package bom

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
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

const (
	moduloDomain      = "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	moduloConformance = "github.com/mateusmacedo/dmpf/tools/dmpf-conformance"
)

// Cópia da fixture em disco temporário com dois módulos no go.work — um em
// libs/, outro em tools/ — e a evidência da 0.1.0 reaproveitada na release.
func workspaceComModulos(t *testing.T, release string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(workspace)); err != nil {
		t.Fatal(err)
	}
	if release != "0.1.0" {
		evidencia := filepath.Join(dir, "bom", "evidence")
		if err := os.CopyFS(filepath.Join(evidencia, release), os.DirFS(filepath.Join(evidencia, "0.1.0"))); err != nil {
			t.Fatal(err)
		}
	}
	escrever := func(rel, conteudo string) {
		t.Helper()
		caminho := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escrever("go.work", "go 1.26.6\n\nuse (\n\t./libs/backend/go/domain\n\t./tools/dmpf-conformance\n)\n")
	escrever("libs/backend/go/domain/go.mod", "module "+moduloDomain+"\n\ngo 1.26.6\n")
	escrever("tools/dmpf-conformance/go.mod", "module "+moduloConformance+"\n\ngo 1.26.6\n")
	return dir
}

// O BOM da fixture reescrito para outra release, o mesmo gesto de criarRelease
// no cmd: tag, nome do arquivo e evidence_uri passam a apontar para ela.
func documentoDaRelease(t *testing.T, release string) Document {
	t.Helper()
	raw, err := os.ReadFile(workspace + "/" + arquivoValido)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode([]byte(strings.ReplaceAll(string(raw), "0.1.0", release)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func entradaKernel(identity, version string, state State) Entry {
	return Entry{Subject: SubjectKernel, Identity: identity, Version: version, State: state, PresentCompatibleWith: true, PresentCVE: true}
}

func comKernel(d *Document, entradas ...Entry) {
	d.Sections[2].Entries = append(d.Sections[2].Entries, entradas...)
}

// alcanceFixo responde pelo mapa (ausente quando a tag não consta) e registra
// as tags perguntadas.
type alcanceFixo struct {
	por     map[string]TagReach
	pedidas []string
}

func (a *alcanceFixo) Reach(tag string) (TagReach, error) {
	a.pedidas = append(a.pedidas, tag)
	return a.por[tag], nil
}

func validarComAlcance(t *testing.T, doc Document, release, raiz string, alcance Ancestry) []rule.Diagnostic {
	t.Helper()
	ds, err := Validate(doc, Input{File: "bom/dmpf/" + release + ".json", Now: instante(t, agoraFixo), Root: os.DirFS(raiz), Ancestry: alcance})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return ds
}

func TestB012TagDeModuloAusenteOuForaDoAlvoReprova(t *testing.T) {
	casos := []struct {
		nome    string
		alcance TagReach
		codigos []rule.Code
	}{
		{nome: "tag ausente", alcance: TagMissing, codigos: []rule.Code{rule.CodeB012}},
		{nome: "tag fora do alvo", alcance: TagNotAncestor, codigos: []rule.Code{rule.CodeB012}},
		{nome: "tag ancestral", alcance: TagAncestor},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raiz := workspaceComModulos(t, "0.2.0")
			doc := documentoDaRelease(t, "0.2.0")
			comKernel(&doc, entradaKernel(moduloDomain, "0.2.0", StateCandidata))
			alcance := &alcanceFixo{por: map[string]TagReach{"libs/backend/go/domain/v0.2.0": c.alcance}}
			exigeCodigos(t, validarComAlcance(t, doc, "0.2.0", raiz, alcance), c.codigos...)
		})
	}
}

func TestB012ResolveATagPeloDiretorioDoModuloNoGoWork(t *testing.T) {
	raiz := workspaceComModulos(t, "0.2.0")
	doc := documentoDaRelease(t, "0.2.0")
	comKernel(&doc,
		entradaKernel(moduloDomain, "0.2.0", StateCandidata),
		entradaKernel(moduloConformance, "0.2.1", StateProposta),
	)
	alcance := &alcanceFixo{por: map[string]TagReach{
		"libs/backend/go/domain/v0.2.0": TagAncestor,
		"tools/dmpf-conformance/v0.2.1": TagAncestor,
	}}
	if ds := validarComAlcance(t, doc, "0.2.0", raiz, alcance); len(ds) > 0 {
		t.Fatalf("tags ancestrais reprovaram:\n%s", listar(ds))
	}
	slices.Sort(alcance.pedidas)
	if want := []string{"libs/backend/go/domain/v0.2.0", "tools/dmpf-conformance/v0.2.1"}; !slices.Equal(alcance.pedidas, want) {
		t.Fatalf("tags perguntadas %v, esperado %v", alcance.pedidas, want)
	}
}

func TestB012ModuloForaDoGoWorkReprovaSemPerguntarTag(t *testing.T) {
	raiz := workspaceComModulos(t, "0.2.0")
	doc := documentoDaRelease(t, "0.2.0")
	comKernel(&doc, entradaKernel("github.com/mateusmacedo/dmpf/libs/backend/go/inexistente", "0.2.0", StateCandidata))
	alcance := &alcanceFixo{}
	exigeCodigos(t, validarComAlcance(t, doc, "0.2.0", raiz, alcance), rule.CodeB012)
	if len(alcance.pedidas) != 0 {
		t.Fatalf("sem diretório não há tag a perguntar; perguntou %v", alcance.pedidas)
	}
}

func TestB012NaoAlcancaRelease010NemRejeitadaNemSemAncestry(t *testing.T) {
	t.Run("release 0.1.0 isenta", func(t *testing.T) {
		raiz := workspaceComModulos(t, "0.1.0")
		doc := documentoValido(t)
		comKernel(&doc, entradaKernel(moduloDomain, "0.0.0", StateCandidata))
		alcance := &alcanceFixo{}
		if ds := validarComAlcance(t, doc, "0.1.0", raiz, alcance); len(ds) > 0 {
			t.Fatalf("0.1.0 reprovou:\n%s", listar(ds))
		}
		if len(alcance.pedidas) != 0 {
			t.Fatalf("0.1.0 não consulta tag; perguntou %v", alcance.pedidas)
		}
	})
	t.Run("rejeitada ignorada", func(t *testing.T) {
		raiz := workspaceComModulos(t, "0.2.0")
		doc := documentoDaRelease(t, "0.2.0")
		comKernel(&doc, entradaKernel(moduloDomain, "0.2.0", StateRejeitada))
		if ds := validarComAlcance(t, doc, "0.2.0", raiz, &alcanceFixo{}); len(ds) > 0 {
			t.Fatalf("rejeitada reprovou:\n%s", listar(ds))
		}
	})
	t.Run("sem Ancestry a regra fica desligada", func(t *testing.T) {
		raiz := workspaceComModulos(t, "0.2.0")
		doc := documentoDaRelease(t, "0.2.0")
		comKernel(&doc, entradaKernel(moduloDomain, "0.2.0", StateCandidata))
		if ds := validarComAlcance(t, doc, "0.2.0", raiz, nil); len(ds) > 0 {
			t.Fatalf("sem Ancestry reprovou:\n%s", listar(ds))
		}
	})
}

func TestModulePathIgnoraComentarioNaLinhaModule(t *testing.T) {
	casos := []struct{ nome, gomod, want string }{
		{"simples", "module " + moduloDomain + "\n\ngo 1.26.6\n", moduloDomain},
		{"comentário de fim de linha", "module " + moduloDomain + " // kernel\n", moduloDomain},
		{"linha comentada acima", "// module antigo\nmodule " + moduloDomain + "\n", moduloDomain},
		{"sem module", "go 1.26.6\n", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := modulePath(c.gomod); got != c.want {
				t.Fatalf("modulePath = %q, esperado %q", got, c.want)
			}
		})
	}
}
