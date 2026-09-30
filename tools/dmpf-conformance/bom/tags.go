package bom

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/internal/rule"
)

// Ancestry responde se a tag de um módulo Go — do kernel ou do contrato de um
// contexto — alcança o commit alvo da validação. Nil desliga o B012, como Base
// nil desliga o B003.
type Ancestry interface {
	Reach(tag string) (TagReach, error)
}

type TagReach int

const (
	TagMissing TagReach = iota
	TagNotAncestor
	TagAncestor
)

// O BOM 0.1.0 certificou os módulos em 0.0.0, antes de existir tag de módulo.
const firstTaggedRelease = "0.2.0"

var contractDir = regexp.MustCompile(`^apps/backend/[^/]+/contract$`)

func (v *validator) checkModuleTags() error {
	if v.in.Ancestry == nil || !semverRe.MatchString(v.doc.Release) || compareSemver(v.doc.Release, firstTaggedRelease) < 0 {
		return nil
	}
	dirs, err := moduleDirs(v.in.Root)
	if err != nil {
		return err
	}
	for _, l := range v.doc.located() {
		e := l.entry
		if (e.Subject != SubjectKernel && e.Subject != SubjectContract) || e.State == StateRejeitada {
			continue
		}
		dir, ok := dirs[e.Identity]
		if !ok {
			v.add(rule.CodeB012, l.path, e.Identity, "módulo fora do go.work: sem diretório de onde derivar a tag")
			continue
		}
		tag := dir + "/v" + strings.TrimPrefix(e.Version, "v")
		reach, err := v.in.Ancestry.Reach(tag)
		if err != nil {
			return fmt.Errorf("tag %s: %w", tag, err)
		}
		switch reach {
		case TagMissing:
			v.add(rule.CodeB012, l.path, e.Identity, fmt.Sprintf("tag %s ausente: a versão %s não foi publicada", tag, e.Version))
		case TagNotAncestor:
			v.add(rule.CodeB012, l.path, e.Identity, fmt.Sprintf("tag %s não é ancestral do commit alvo", tag))
		}
	}
	v.checkContractEntries(dirs)
	return nil
}

func moduleDirs(fsys fs.FS) (map[string]string, error) {
	uses, err := goWorkUses(fsys)
	if err != nil {
		return nil, err
	}
	dirs := make(map[string]string, len(uses))
	for _, dir := range uses {
		raw, err := fs.ReadFile(fsys, path.Join(dir, "go.mod"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("ler %s/go.mod: %w", dir, err)
		}
		if module := modulePath(string(raw)); module != "" {
			dirs[module] = dir
		}
	}
	return dirs, nil
}

func modulePath(gomod string) string {
	for _, line := range strings.Split(withoutComments(gomod), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}

func goWorkUses(fsys fs.FS) ([]string, error) {
	raw, err := fs.ReadFile(fsys, "go.work")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler go.work: %w", err)
	}
	var uses []string
	inBlock := false
	for _, line := range strings.Split(withoutComments(string(raw)), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0:
		case fields[0] == "use" && len(fields) > 1 && fields[1] == "(":
			inBlock = true
		case fields[0] == "use" && len(fields) > 1:
			uses = append(uses, path.Clean(fields[1]))
		case fields[0] == ")":
			inBlock = false
		case inBlock:
			uses = append(uses, path.Clean(fields[0]))
		}
	}
	return uses, nil
}

func (v *validator) checkContractEntries(dirs map[string]string) {
	declared := map[string]bool{}
	for _, l := range v.doc.located() {
		if l.entry.Subject == SubjectContract {
			declared[l.entry.Identity] = true
		}
	}
	identities := slices.Sorted(maps.Keys(dirs))
	for _, identity := range identities {
		if contractDir.MatchString(dirs[identity]) && !declared[identity] {
			v.add(rule.CodeB012, "", identity, fmt.Sprintf("módulo de contrato %s sem entrada subject: contract", dirs[identity]))
		}
	}
}
