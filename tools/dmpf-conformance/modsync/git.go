package modsync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func recusarCloneRaso(ctx context.Context, abs string) error {
	saida, err := executar(ctx, abs, "git", "rev-parse", "--is-shallow-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(saida)) == "true" {
		return errors.New("clone raso: tags de módulo podem faltar e a versão do require sairia errada; busque o histórico completo (fetch-depth: 0)")
	}
	return nil
}

func versoesPorTag(ctx context.Context, abs string, modules []Module) (map[string]string, error) {
	saida, err := executar(ctx, abs, "git", "tag", "--merged", "HEAD")
	if err != nil {
		return nil, err
	}
	tags := strings.Fields(string(saida))
	versoes := make(map[string]string, len(modules))
	for _, m := range modules {
		versoes[m.Path] = maiorRelease(tags, m.Dir)
	}
	return versoes, nil
}

// Pré-release só conta sem estável: com as duas, uma rc da próxima versão
// passaria à frente da release que os consumidores de fato usam.
func maiorRelease(tags []string, dir string) string {
	var estavel, pre *versao
	for _, tag := range tags {
		bruta, ok := strings.CutPrefix(tag, dir+"/")
		if !ok {
			continue
		}
		v, ok := parseRelease(bruta)
		if !ok {
			continue
		}
		alvo := &estavel
		if len(v.pre) > 0 {
			alvo = &pre
		}
		if *alvo == nil || v.compare(**alvo) > 0 {
			*alvo = &v
		}
	}
	switch {
	case estavel != nil:
		return estavel.String()
	case pre != nil:
		return pre.String()
	default:
		return versaoInicial
	}
}

type versao struct {
	nucleo [3]int
	pre    []string
}

func (v versao) String() string {
	s := fmt.Sprintf("v%d.%d.%d", v.nucleo[0], v.nucleo[1], v.nucleo[2])
	if len(v.pre) > 0 {
		s += "-" + strings.Join(v.pre, ".")
	}
	return s
}

// Precedência do SemVer 2.0 §11; só é chamada entre versões do mesmo tipo
// (estável com estável, pré-release com pré-release).
func (v versao) compare(o versao) int {
	if c := slices.Compare(v.nucleo[:], o.nucleo[:]); c != 0 {
		return c
	}
	for i := range min(len(v.pre), len(o.pre)) {
		if c := compararIdentificador(v.pre[i], o.pre[i]); c != 0 {
			return c
		}
	}
	return len(v.pre) - len(o.pre)
}

func compararIdentificador(a, b string) int {
	na, aNum := numerico(a)
	nb, bNum := numerico(b)
	switch {
	case aNum && bNum:
		return na - nb
	case aNum:
		return -1
	case bNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func numerico(s string) (int, bool) {
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func parseRelease(bruta string) (versao, bool) {
	resto, ok := strings.CutPrefix(bruta, "v")
	if !ok {
		return versao{}, false
	}
	nucleo, pre, temPre := strings.Cut(resto, "-")
	partes := strings.Split(nucleo, ".")
	if len(partes) != 3 {
		return versao{}, false
	}
	var v versao
	for i, parte := range partes {
		n, ok := numerico(parte)
		if !ok || (len(parte) > 1 && parte[0] == '0') {
			return versao{}, false
		}
		v.nucleo[i] = n
	}
	if !temPre {
		return v, true
	}
	for _, id := range strings.Split(pre, ".") {
		if id == "" || strings.Trim(id, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-") != "" {
			return versao{}, false
		}
		if _, ok := numerico(id); ok && len(id) > 1 && id[0] == '0' {
			return versao{}, false
		}
		v.pre = append(v.pre, id)
	}
	return v, true
}
