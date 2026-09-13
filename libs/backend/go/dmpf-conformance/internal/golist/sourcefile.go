package golist

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
)

// Único uso de go/parser no verificador, e ele nunca decide: a decisão é sobre
// o package que o compilador resolveu. Aqui o texto só aponta onde a aresta
// nasce, para quem for ler o diagnóstico.
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

// O cache vive na instância, não no package: como global, faria o resultado
// depender do histórico do processo — em teste, de qual caso rodou antes.
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
