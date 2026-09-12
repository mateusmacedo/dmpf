package rule

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Conjunto fechado de dezesseis. Classe não implementada é ausência declarada
// de verificação, nunca "conforme".
type Code string

const (
	CodeU001 Code = "DMPF-U001"
	CodeU002 Code = "DMPF-U002"
	CodeU003 Code = "DMPF-U003"
	CodeU004 Code = "DMPF-U004"
	CodeM001 Code = "DMPF-M001"
	CodeM002 Code = "DMPF-M002"
	CodeM003 Code = "DMPF-M003"
	CodeM004 Code = "DMPF-M004"
	CodeT001 Code = "DMPF-T001"
	CodeT002 Code = "DMPF-T002"
	CodeD001 Code = "DMPF-D001"
	CodeD002 Code = "DMPF-D002"
	CodeE001 Code = "DMPF-E001"
	CodeE002 Code = "DMPF-E002"
	CodeE003 Code = "DMPF-E003"
	CodeE004 Code = "DMPF-E004"
)

// GOV-30..GOV-36 decidem a admissão de um pedido de exceção, não a aresta:
// ficam fora do conjunto fechado de §10.3, que é sobre a regra de dependência.
const (
	CodeX001 Code = "DMPF-X001"
	CodeX002 Code = "DMPF-X002"
	CodeX003 Code = "DMPF-X003"
	CodeX004 Code = "DMPF-X004"
	CodeX005 Code = "DMPF-X005"
	CodeX006 Code = "DMPF-X006"
	CodeX007 Code = "DMPF-X007"
)

// BOM-01..BOM-09 e GOV-36 decidem a certificação de uma combinação, não a
// aresta: ficam fora do conjunto fechado de §10.3, como os X*.
const (
	CodeB001 Code = "DMPF-B001"
	CodeB002 Code = "DMPF-B002"
	CodeB003 Code = "DMPF-B003"
	CodeB004 Code = "DMPF-B004"
	CodeB005 Code = "DMPF-B005"
	CodeB006 Code = "DMPF-B006"
	CodeB007 Code = "DMPF-B007"
	CodeB008 Code = "DMPF-B008"
	CodeB009 Code = "DMPF-B009"
	CodeB010 Code = "DMPF-B010"
	CodeB011 Code = "DMPF-B011"
)

type CodeSpec struct {
	Code       Code
	Summary    string
	Section    string
	Applicable bool
}

// A ordem é a da tabela normativa, e é a que a documentação usa.
var codeSpecs = []CodeSpec{
	{CodeU001, "Arquivo de produção não coberto por nenhuma unidade", "RFC §3.6", true},
	{CodeU002, "Arquivo coberto por mais de uma unidade", "RFC §3.6", true},
	{CodeU003, "canonical_key duplicada no universo", "RFC §3.6", true},
	{CodeU004, "Manifesto ausente em módulo com código de produção", "RFC §3.6", true},
	{CodeM001, "Campo obrigatório ausente no manifesto", "RFC §10.1", true},
	{CodeM002, "Valor fora do conjunto fechado", "RFC §10.1, §7.2", true},
	{CodeM003, "Unidade duplicada no manifesto", "RFC §10.1", true},
	{CodeM004, "Unidade de shared kernel não resolvida", "ADR-042; RFC §7.2 estendida", true},
	{CodeT001, "Divergência entre manifesto e baseline", "RFC §10.2 T3", true},
	{CodeT002, "Mudança normativa sem evidência de autorização", "RFC §10.2 T5", true},
	{CodeD001, "Aresta proibida entre blocos", "RFC §7.1 C1, §7.3", true},
	{CodeD002, "Aresta proibida entre bounded contexts", "RFC §7.1 C2, §5.5", true},
	{CodeE001, "Capability externa não permitida para o bloco", "RFC §6.2", true},
	{CodeE002, "Dependência declarada pure com fechamento impuro", "RFC §6.3", true},
	{CodeE003, "Import não resolvido", "RFC §10.3", true},
	// Reservado e sem ocorrência possível em Go: a gramática exige
	// `ImportPath = string_lit`, e `plugin.Open` já cai em E001/E002.
	{CodeE004, "Import dinâmico com alvo não determinável", "RFC §10.3", false},
}

// A ordem é a do catálogo fechado N1–N7 de GOV-32.
var governanceSpecs = []CodeSpec{
	{CodeX001, "Pedido de exceção sem os itens obrigatórios", "GOV-30, GOV-33", true},
	{CodeX002, "Objeto da exceção fora das classes E1–E3", "GOV-31", true},
	{CodeX003, "Objeto que nenhuma exceção alcança: aresta, célula ou política de bloco", "GOV-32 N1, N2", true},
	{CodeX004, "Objeto sob identidade reservada do catálogo fechado", "GOV-32 N1, N3–N7", true},
	{CodeX005, "Ciclo de vida da exceção inconsistente", "GOV-34", true},
	{CodeX006, "Exceção vencida sem renovação posterior", "GOV-34", true},
	{CodeX007, "Exceção declarada fora do registro que a comporta", "GOV-35", true},
}

// A ordem é a da tabela de diagnósticos do BOM, e é a que a documentação usa.
var bomSpecs = []CodeSpec{
	{CodeB001, "Seção ausente, ou vazia sem reason", "BOM-01", true},
	{CodeB002, "Campo obrigatório ausente; version com faixa; state, criticality ou subject fora do conjunto", "BOM-03", true},
	{CodeB003, "Transição inválida entre o BOM anterior e o atual, ou depreciada sem deprecated_at/successor", "BOM-07", true},
	{CodeB004, "Certificada sem um dos cinco campos de evidência", "BOM-07", true},
	{CodeB005, "evidence_digest diferente do SHA-256 do arquivo em bom/evidence/<release>/", "BOM-03", true},
	{CodeB006, "compatible_with com identity e version sem execução na evidência referenciada", "BOM-04", true},
	{CodeB007, "Valor resolvido por registry_ref diferente da version declarada; registry_ref ausente em sujeito da tabela", "BOM-06", true},
	{CodeB008, "Certificada com valid_until anterior a now — erro, nunca aviso", "BOM-08", true},
	{CodeB009, "cve ausente; CVE aberta sem owner", "BOM-03, BOM-09", true},
	{CodeB010, "metrics declaradas diferentes das derivadas de exceptions[].history", "GOV-36", true},
	{CodeB011, "Mais de um arquivo em bom/dmpf/ sem --release; release do documento diferente do nome do arquivo; tag diferente de dmpf@<release>", "BOM-02", true},
}

// CodeSpecs devolve só as dezesseis de §10.3: é o conjunto que a RFC fixa, e
// quem o consome espera a tabela normativa da regra de dependência.
func CodeSpecs() []CodeSpec {
	out := make([]CodeSpec, len(codeSpecs))
	copy(out, codeSpecs)
	return out
}

func GovernanceCodeSpecs() []CodeSpec {
	out := make([]CodeSpec, len(governanceSpecs))
	copy(out, governanceSpecs)
	return out
}

func BOMCodeSpecs() []CodeSpec {
	out := make([]CodeSpec, len(bomSpecs))
	copy(out, bomSpecs)
	return out
}

func LookupCode(c Code) (CodeSpec, bool) {
	for _, s := range codeSpecs {
		if s.Code == c {
			return s, true
		}
	}
	for _, s := range governanceSpecs {
		if s.Code == c {
			return s, true
		}
	}
	for _, s := range bomSpecs {
		if s.Code == c {
			return s, true
		}
	}
	return CodeSpec{}, false
}

// CanonicalKey é o import path completo da origem.
type Diagnostic struct {
	Code         Code
	CanonicalKey string
	Target       string
	SourceFile   string
	Detail       string
}

func (d Diagnostic) Section() string {
	if s, ok := LookupCode(d.Code); ok {
		return s.Section
	}
	return ""
}

// Todo campo deriva de entrada não confiável: o manifesto é escrito por quem
// abre o PR. Sem isto, um `id` com quebra de linha forja uma linha inteira no
// log e o gate passa a mentir para quem o lê.
func umaLinha(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || (r < 0x20) || r == 0x7f {
			return '\ufffd'
		}
		return r
	}, v)
}

func (d Diagnostic) String() string {
	var b strings.Builder
	b.WriteString(string(d.Code))
	b.WriteString(": ")
	if d.Target != "" {
		fmt.Fprintf(&b, "%s -> %s", umaLinha(d.CanonicalKey), umaLinha(d.Target))
	} else {
		b.WriteString(umaLinha(d.CanonicalKey))
	}
	if d.SourceFile != "" {
		fmt.Fprintf(&b, " (%s)", umaLinha(d.SourceFile))
	}
	if d.Detail != "" {
		fmt.Fprintf(&b, ": %s", umaLinha(d.Detail))
	}
	if s := d.Section(); s != "" {
		fmt.Fprintf(&b, " [%s]", s)
	}
	return b.String()
}

// Ordem total sobre os campos de saída: parcial deixaria diagnósticos de mesma
// chave e código oscilarem entre execuções.
func SortDiagnostics(ds []Diagnostic) {
	slices.SortStableFunc(ds, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.CanonicalKey, b.CanonicalKey),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Target, b.Target),
			cmp.Compare(a.SourceFile, b.SourceFile),
			cmp.Compare(a.Detail, b.Detail),
		)
	})
}
