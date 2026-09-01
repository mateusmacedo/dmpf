package rule

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Code é um dos quinze códigos estáveis de RFC §10.3. O conjunto é fechado:
// classe não implementada é ausência declarada de verificação, nunca "conforme".
type Code string

const (
	CodeU001 Code = "DMPF-U001"
	CodeU002 Code = "DMPF-U002"
	CodeU003 Code = "DMPF-U003"
	CodeU004 Code = "DMPF-U004"
	CodeM001 Code = "DMPF-M001"
	CodeM002 Code = "DMPF-M002"
	CodeM003 Code = "DMPF-M003"
	CodeT001 Code = "DMPF-T001"
	CodeT002 Code = "DMPF-T002"
	CodeD001 Code = "DMPF-D001"
	CodeD002 Code = "DMPF-D002"
	CodeE001 Code = "DMPF-E001"
	CodeE002 Code = "DMPF-E002"
	CodeE003 Code = "DMPF-E003"
	CodeE004 Code = "DMPF-E004"
)

// CodeSpec descreve um código e o amarra à seção normativa que o institui.
type CodeSpec struct {
	Code       Code
	Summary    string
	Section    string
	Applicable bool
}

// codeSpecs é a transcrição da tabela de RFC §10.3, na ordem em que a RFC a
// apresenta. É a fonte de verdade do conjunto fechado.
var codeSpecs = []CodeSpec{
	{CodeU001, "Arquivo de produção não coberto por nenhuma unidade", "RFC §3.6", true},
	{CodeU002, "Arquivo coberto por mais de uma unidade", "RFC §3.6", true},
	{CodeU003, "canonical_key duplicada no universo", "RFC §3.6", true},
	{CodeU004, "Manifesto ausente em módulo com código de produção", "RFC §3.6", true},
	{CodeM001, "Campo obrigatório ausente no manifesto", "RFC §10.1", true},
	{CodeM002, "Valor fora do conjunto fechado", "RFC §10.1, §7.2", true},
	{CodeM003, "Unidade duplicada no manifesto", "RFC §10.1", true},
	{CodeT001, "Divergência entre manifesto e baseline", "RFC §10.2 T3", true},
	{CodeT002, "Mudança normativa sem evidência de autorização", "RFC §10.2 T5", true},
	{CodeD001, "Aresta proibida entre blocos", "RFC §7.1 C1, §7.3", true},
	{CodeD002, "Aresta proibida entre bounded contexts", "RFC §7.1 C2, §5.5", true},
	{CodeE001, "Capability externa não permitida para o bloco", "RFC §6.2", true},
	{CodeE002, "Dependência declarada pure com fechamento impuro", "RFC §6.3", true},
	{CodeE003, "Import não resolvido", "RFC §10.3", true},
	// DMPF-E004 permanece reservado e estável no conjunto dos quinze, sem
	// ocorrência possível neste binding: a gramática de Go exige
	// `ImportPath = string_lit`, e carregamento dinâmico via API (plugin.Open e
	// afins) já é capturado pela política de capabilities (E001/E002).
	{CodeE004, "Import dinâmico com alvo não determinável", "RFC §10.3", false},
}

// CodeSpecs devolve a tabela de RFC §10.3 na ordem normativa.
func CodeSpecs() []CodeSpec {
	out := make([]CodeSpec, len(codeSpecs))
	copy(out, codeSpecs)
	return out
}

func LookupCode(c Code) (CodeSpec, bool) {
	for _, s := range codeSpecs {
		if s.Code == c {
			return s, true
		}
	}
	return CodeSpec{}, false
}

// Diagnostic é uma reprovação individual. CanonicalKey é o import path completo
// da unidade de origem (ADR-011); Target, quando presente, é o destino da aresta.
type Diagnostic struct {
	Code         Code
	CanonicalKey string
	Target       string
	SourceFile   string
	Detail       string
}

// Section devolve a seção normativa que institui o código do diagnóstico.
func (d Diagnostic) Section() string {
	if s, ok := LookupCode(d.Code); ok {
		return s.Section
	}
	return ""
}

// umaLinha neutraliza caracteres de controle no texto renderizado.
//
// Todo campo do diagnóstico deriva de entrada não confiável: o manifesto é
// escrito por quem abre o PR, e o import path vem do módulo que ele controla.
// Sem isto, um `id` com quebra de linha forja uma linha inteira no log do CI —
// "DMPF-D001: ... conforme" — e o gate passa a mentir para quem o lê. A saída é
// um diagnóstico por linha, e essa invariante é parte do contrato.
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

// SortDiagnostics impõe a ordem determinística exigida como requisito não
// funcional: por canonical_key, depois código, depois destino e detalhe.
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
