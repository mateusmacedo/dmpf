package rule_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const arquivoOraculo = "matrix_oracle_test.go"

// simbolosDeProducao são os identificadores pelos quais o oráculo poderia
// derivar da matriz de produção, direta ou indiretamente.
var simbolosDeProducao = []string{
	"matrix", "matrixRow", "AllowedByMatrix", "blockIndex",
	"blocks", "Blocks", "Decide", "DiagnoseEdge", "permitida", "proibida",
}

// TestOraculoNaoDerivaDaProducao é o controle ESTRUTURAL da independência.
//
// O red control em runtime (matrix_redcontrol_test.go) não basta: ele muta
// `matrix` depois da inicialização do package, então um oráculo derivado —
// `var oracle = buildOracleFromMatrix()` — snapshotaria os valores no init,
// divergiria da matriz mutada e PASSARIA no controle. Ele prova que o oráculo é
// um snapshot, não que a origem é independente.
//
// Este teste prova a origem: lê o AST da declaração de `oracle` e exige que ela
// seja literal em toda a extensão, sem referência a símbolo de produção. Um
// oráculo derivado não compila essa exigência.
func TestOraculoNaoDerivaDaProducao(t *testing.T) {
	fset := token.NewFileSet()
	arquivo, err := parser.ParseFile(fset, arquivoOraculo, nil, 0)
	if err != nil {
		t.Fatalf("parse de %s: %v", arquivoOraculo, err)
	}

	decl := declaracaoDeOraculo(t, arquivo)

	var celulas int
	ast.Inspect(decl, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			for _, proibido := range simbolosDeProducao {
				if v.Name == proibido {
					t.Errorf("declaração de oracle referencia o símbolo de produção %q em %s: o oráculo estaria derivado da matriz",
						v.Name, fset.Position(v.Pos()))
				}
			}
		case *ast.CallExpr:
			t.Errorf("declaração de oracle contém chamada de função em %s: o valor precisa ser literal, não computado",
				fset.Position(v.Pos()))
		case *ast.CompositeLit:
			if len(v.Elts) == 4 {
				celulas++
				exigeBooleanoLiteral(t, fset, v.Elts[3])
			}
		}
		return true
	})

	if celulas != 36 {
		t.Errorf("declaração de oracle tem %d células literais de 4 campos, esperadas 36", celulas)
	}
}

// exigeBooleanoLiteral garante que a decisão da célula é `true` ou `false`
// escrito à mão, nunca uma expressão que possa consultar a produção.
func exigeBooleanoLiteral(t *testing.T, fset *token.FileSet, e ast.Expr) {
	t.Helper()
	id, ok := e.(*ast.Ident)
	if !ok || (id.Name != "true" && id.Name != "false") {
		t.Errorf("decisão da célula em %s não é literal booleano", fset.Position(e.Pos()))
	}
}

func declaracaoDeOraculo(t *testing.T, arquivo *ast.File) ast.Node {
	t.Helper()
	for _, d := range arquivo.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name.Name == "oracle" {
					return vs
				}
			}
		}
	}
	t.Fatalf("declaração de `oracle` não encontrada em %s: o controle de independência deixou de vigiar alguma coisa", arquivoOraculo)
	return nil
}

// TestArquivoDoOraculoNaoImportaProducao fecha a última via: o oráculo vive em
// `package rule` e enxerga os símbolos internos sem import. Se um dia migrar
// para package externo, um import de produção reabriria a derivação.
func TestArquivoDoOraculoNaoImportaProducao(t *testing.T) {
	fset := token.NewFileSet()
	arquivo, err := parser.ParseFile(fset, arquivoOraculo, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse de %s: %v", arquivoOraculo, err)
	}
	for _, imp := range arquivo.Imports {
		caminho := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(caminho, "dmpf-conformance/internal/") {
			t.Errorf("arquivo do oráculo importa produção: %s", caminho)
		}
	}
}
