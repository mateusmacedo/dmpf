package bom

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/exception"
	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

type Input struct {
	// Relativo à raiz do workspace: o nome do arquivo é a release (BOM-02).
	File string
	Now  exception.Instant
	Root fs.FS
	// Nil desliga a comparação de transições do B003.
	Base *Document
}

func Validate(doc Document, in Input) ([]rule.Diagnostic, error) {
	if in.Root == nil {
		return nil, errors.New("bom: Input.Root é obrigatório")
	}
	v := &validator{doc: doc, in: in}
	v.checkIdentity()
	v.checkSections()
	v.checkDuplicateEntries()
	for _, l := range doc.located() {
		v.checkEntry(l)
	}
	for _, check := range []func() error{v.checkDigests, v.checkCompatibility, v.checkRegistries} {
		if err := check(); err != nil {
			return nil, err
		}
	}
	v.checkTransitions()
	v.checkExceptions()
	v.checkMetrics()
	SortReport(v.out)
	return v.out, nil
}

func ValidRelease(release string) bool {
	return semverRe.MatchString(release)
}

type validator struct {
	doc Document
	in  Input
	out []rule.Diagnostic
}

func (v *validator) add(code rule.Code, where, target, detail string) {
	key := v.in.File
	if where != "" {
		if key != "" {
			key += "#"
		}
		key += where
	}
	v.out = append(v.out, rule.Diagnostic{Code: code, CanonicalKey: key, Target: target, Detail: detail})
}

type field struct{ name, value string }

func missingFields(fields ...field) []string {
	var out []string
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			out = append(out, f.name)
		}
	}
	return out
}

var (
	semverRe       = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	exactVersionRe = regexp.MustCompile(`^(?:v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?|[0-9a-f]{7,40})$`)
	wildcardRe     = regexp.MustCompile(`(^|\.)[xX*](\.|$)`)
)

func isRange(version string) bool {
	return strings.ContainsAny(version, "^~<>=*|, ") || wildcardRe.MatchString(version)
}

func (v *validator) checkIdentity() {
	d := v.doc
	if d.Schema != SchemaID {
		v.add(rule.CodeB002, "schema", "", fmt.Sprintf("schema %q fora do conjunto fechado (esperado %q)", d.Schema, SchemaID))
	}
	if d.Product != Product {
		v.add(rule.CodeB002, "product", "", fmt.Sprintf("product %q, esperado %q", d.Product, Product))
	}
	if !d.PresentSemconv {
		v.add(rule.CodeB002, SlotSemconv, "", "slot nomeado de BOM-10 ausente")
	}
	if !d.PresentExceptions {
		v.add(rule.CodeB002, "exceptions", "", "exceptions ausente: declare [] quando não houver exceção")
	}

	if !semverRe.MatchString(d.Release) {
		v.add(rule.CodeB011, "release", "", fmt.Sprintf("release %q não é semver", d.Release))
	}
	if v.in.File != "" {
		if name := strings.TrimSuffix(path.Base(v.in.File), ".json"); name != d.Release {
			v.add(rule.CodeB011, "release", "", fmt.Sprintf("release %q difere do nome do arquivo %q", d.Release, name))
		}
	}
	if want := "dmpf@" + d.Release; d.Tag != want {
		v.add(rule.CodeB011, "tag", "", fmt.Sprintf("tag %q difere de %q", d.Tag, want))
	}
}

func (v *validator) checkSections() {
	for _, s := range v.doc.Sections {
		switch {
		case !s.Present:
			v.add(rule.CodeB001, s.Name, "", "seção ausente: item sem instância é declarado vazio, com reason")
		case len(s.Entries) == 0 && strings.TrimSpace(s.Reason) == "":
			v.add(rule.CodeB001, s.Name, "", "seção vazia sem reason")
		}
	}
}

func (v *validator) checkDuplicateEntries() {
	first := map[string]string{}
	for _, l := range v.doc.located() {
		if l.entry.Identity == "" {
			continue
		}
		if onde, repetida := first[l.key()]; repetida {
			v.add(rule.CodeB002, l.path, l.entry.Identity,
				fmt.Sprintf("entrada repetida: subject, identity e version iguais aos de %s", onde))
			continue
		}
		first[l.key()] = l.path
	}
}

func (v *validator) checkEntry(l located) {
	e := l.entry
	add := func(code rule.Code, detail string) { v.add(code, l.path, e.Identity, detail) }

	for _, name := range missingFields(
		field{"subject", string(e.Subject)}, field{"identity", e.Identity},
		field{"version", e.Version}, field{"state", string(e.State)},
	) {
		add(rule.CodeB002, fmt.Sprintf("campo obrigatório %q ausente", name))
	}
	if e.Subject != "" && !isSubject(e.Subject) {
		add(rule.CodeB002, fmt.Sprintf("subject %q fora do conjunto fechado", e.Subject))
	}
	if e.State != "" && !isState(e.State) {
		add(rule.CodeB002, fmt.Sprintf("state %q fora dos estados de BOM-07", e.State))
	}
	if e.Criticality != "" && !isCriticality(e.Criticality) {
		add(rule.CodeB002, fmt.Sprintf("criticality %q fora de critica|padrao", e.Criticality))
	}
	if e.Version != "" && !exactVersionRe.MatchString(e.Version) {
		add(rule.CodeB002, fmt.Sprintf("version %q não é versão exata (semver ou SHA de commit): faixa pertence à matriz de compatibilidade", e.Version))
	}
	if l.section == SlotSemconv {
		if e.Subject != SubjectSemconv {
			add(rule.CodeB002, fmt.Sprintf("o slot de BOM-10 exige subject %q", SubjectSemconv))
		}
		if strings.TrimSpace(e.Owner) == "" {
			add(rule.CodeB002, "o slot de BOM-10 exige owner")
		}
	}

	if !e.PresentCompatibleWith {
		add(rule.CodeB002, "compatible_with ausente: declare [] quando nada foi exercitado em conjunto")
	}
	for i, c := range e.CompatibleWith {
		switch {
		case c.Identity == "" || c.Version == "":
			add(rule.CodeB002, fmt.Sprintf("compatible_with[%d] sem identity ou version", i))
		case !exactVersionRe.MatchString(c.Version):
			add(rule.CodeB002, fmt.Sprintf("compatible_with[%d]: version %q não é versão exata", i, c.Version))
		}
	}

	for _, f := range []field{{"certified_at", e.CertifiedAt}, {"valid_until", e.ValidUntil}, {"deprecated_at", e.DeprecatedAt}} {
		if _, ok := parseDate(f.value); f.value != "" && !ok {
			add(rule.CodeB002, fmt.Sprintf("%s %q não é RFC3339 nem AAAA-MM-DD", f.name, f.value))
		}
	}

	switch e.State {
	case StateCertificada:
		v.checkCertification(l)
	case StateDepreciada:
		if missing := missingFields(field{"deprecated_at", e.DeprecatedAt}, field{"successor", e.Successor}); len(missing) > 0 {
			add(rule.CodeB003, "depreciada sem "+strings.Join(missing, " e "))
		}
	}

	v.checkCVEs(l)
}

func (v *validator) checkCertification(l located) {
	e := l.entry
	if missing := missingFields(
		field{"evidence_uri", e.EvidenceURI}, field{"evidence_digest", e.EvidenceDigest},
		field{"approved_by", e.ApprovedBy}, field{"certified_at", e.CertifiedAt},
		field{"valid_until", e.ValidUntil},
	); len(missing) > 0 {
		v.add(rule.CodeB004, l.path, e.Identity, "certificada sem "+strings.Join(missing, ", "))
	}
	if p := e.Promoted; p == nil || missingFields(field{"by", p.By}, field{"reviewed_by", p.ReviewedBy}, field{"pr", p.PR}) != nil {
		v.add(rule.CodeB002, l.path, e.Identity, "certificada sem promoted {by, reviewed_by, pr}: o ato de BOM-05")
	}
	// Erro, nunca aviso: BOM-07 proíbe rebaixar por decurso de prazo e BOM-08
	// nega a certificação vencida — resta reprovar até o commit que rebaixa.
	if until, ok := parseUntil(e.ValidUntil); ok && v.in.Now > exception.Instant(until.UnixNano()) {
		v.add(rule.CodeB008, l.path, e.Identity,
			fmt.Sprintf("certificação vencida em %s: reprova até o commit que declara candidata", e.ValidUntil))
	}
}

func (v *validator) checkCVEs(l located) {
	e := l.entry
	if !e.PresentCVE {
		v.add(rule.CodeB009, l.path, e.Identity, "cve ausente: declare [] quando não houver CVE conhecida")
	}
	for i, c := range e.CVE {
		where := fmt.Sprintf("%s.cve[%d]", l.path, i)
		if strings.TrimSpace(c.ID) == "" {
			v.add(rule.CodeB002, where, e.Identity, "CVE sem id")
		}
		if !isCVEState(c.State) {
			v.add(rule.CodeB002, where, c.ID, fmt.Sprintf("state %q fora de corrigida|mitigada|aberta", c.State))
		}
		if c.State == CVEAberta && strings.TrimSpace(c.Owner) == "" {
			v.add(rule.CodeB009, where, c.ID, "CVE aberta sem owner (BOM-09)")
		}
	}
}

func (v *validator) checkExceptions() {
	ids := map[string]int{}
	for _, x := range v.doc.Exceptions {
		ids[x.ID]++
	}
	for i, x := range v.doc.Exceptions {
		where := fmt.Sprintf("exceptions[%d]", i)
		if x.ID != "" && ids[x.ID] > 1 {
			v.add(rule.CodeX001, where, x.ID, fmt.Sprintf("id %q repetido %d vezes no BOM: GOV-33 exige identidade única", x.ID, ids[x.ID]))
		}
		for _, d := range exception.AdmitIn(x, exception.RegistryBOM, exception.Subject{}, v.in.Now) {
			v.add(d.Code, where, d.Target, d.Detail)
		}
	}
}
