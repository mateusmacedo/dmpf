package golist

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
)

// arquivoDoImport devolve o arquivo que introduz o import, para a mensagem do
// diagnóstico.
//
// Este é o ÚNICO uso de go/parser no verificador, e ele nunca decide nada: a
// decisão é sempre sobre o package resolvido pelo toolchain (RFC §3.5). Aqui o
// texto do import serve apenas para apontar ao humano onde a aresta nasce.
func (s *Source) arquivoDoImport(p listPackage, importPath string) string {
	if p.Dir == "" {
		return ""
	}
	for _, f := range p.arquivosDeProducao() {
		if s.importaPath(filepath.Join(p.Dir, f), importPath) {
			return filepath.Join(p.ImportPath, f)
		}
	}
	return ""
}

// importaPath consulta o cache de imports do próprio Source. O cache vive na
// instância, não no package: como global, ele sobreviveria entre execuções no
// mesmo processo e faria o resultado depender do histórico — em teste, de qual
// caso rodou antes.
func (s *Source) importaPath(arquivo, importPath string) bool {
	if s.imports == nil {
		s.imports = map[string]map[string]bool{}
	}
	imports, ok := s.imports[arquivo]
	if !ok {
		imports = map[string]bool{}
		fset := token.NewFileSet()
		if f, err := parser.ParseFile(fset, arquivo, nil, parser.ImportsOnly); err == nil {
			for _, spec := range f.Imports {
				if v, err := strconv.Unquote(spec.Path.Value); err == nil {
					imports[v] = true
				}
			}
		}
		s.imports[arquivo] = imports
	}
	return imports[importPath]
}
