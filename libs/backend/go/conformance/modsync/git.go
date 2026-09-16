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

func maiorRelease(tags []string, dir string) string {
	var maior versao
	achou := false
	for _, tag := range tags {
		bruta, ok := strings.CutPrefix(tag, dir+"/")
		if !ok {
			continue
		}
		v, ok := parseRelease(bruta)
		if ok && (!achou || slices.Compare(v[:], maior[:]) > 0) {
			maior, achou = v, true
		}
	}
	if !achou {
		return versaoInicial
	}
	return maior.String()
}

type versao [3]int

func (v versao) String() string {
	return fmt.Sprintf("v%d.%d.%d", v[0], v[1], v[2])
}

func parseRelease(bruta string) (versao, bool) {
	resto, ok := strings.CutPrefix(bruta, "v")
	if !ok {
		return versao{}, false
	}
	partes := strings.Split(resto, ".")
	if len(partes) != 3 {
		return versao{}, false
	}
	var v versao
	for i, parte := range partes {
		if parte == "" || (len(parte) > 1 && parte[0] == '0') || strings.Trim(parte, "0123456789") != "" {
			return versao{}, false
		}
		n, err := strconv.Atoi(parte)
		if err != nil {
			return versao{}, false
		}
		v[i] = n
	}
	return v, true
}
