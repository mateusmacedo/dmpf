package fitness

import conffit "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/fitness"

// Vector is one row of RFC §11.3, the map every normative rule has to an
// executable pair (RAS-01). Cell is the matrix cell the pair exercises, zero
// when the vector is not about one cell; SingleStack names the only stack that
// realizes the pair, and Reason says why the other cannot (RAS-15, RAS-16).
type Vector struct {
	ID          string
	Rule        string
	Cell        int
	Code        conffit.Code
	SingleStack string
	Reason      string
	Where       string
}

const (
	whereCells    = "fitness/cells_test.go"
	whereChecker  = "dmpf-conformance/internal/conformance/vectors_test.go (reused, FIT-02)"
	whereEdges    = "fitness/edges_test.go"
	whereDomain   = "fitness/domaintest_test.go"
	whereV31      = "fitness/v31_test.go"
	whereDistkit  = "distkit/*_test.go"
	whereRegister = "fitness/v27_test.go (registered, not executed)"
)

// Transcribed by hand from RFC §11.3 and §11.4; the checker's own vector table
// is not imported, so the two registers can disagree and the test notices it.
var Vectors = []Vector{
	{ID: "V13", Rule: "Domínio sem I/O (P0-1)", Cell: 5, Code: conffit.CodeD001, Where: whereCells},
	{ID: "V14", Rule: "Wire fora do domínio (P0-2)", Cell: 6, Code: conffit.CodeD001, Where: whereCells},
	{ID: "V15", Rule: "Domínio não importa porta", Cell: 4, Code: conffit.CodeD001, Where: whereCells},
	{ID: "V16", Rule: "Aplicação não conhece driver", Cell: 11, Code: conffit.CodeD001, Where: whereCells},
	{ID: "V17", Rule: "Porta não conhece driver", Cell: 23, Code: conffit.CodeD001, Where: whereCells},
	{ID: "V18", Rule: "domain → domain intra-context", Cell: 1, Where: whereCells},
	{ID: "V19", Rule: "domain → domain inter-context", Cell: 1, Code: conffit.CodeD002, Where: whereCells},
	{ID: "V20", Rule: "Integração por superfície pública", Cell: 14, Code: conffit.CodeD002, Where: whereCells},
	{ID: "V21", Rule: "Capability externa por bloco", Code: conffit.CodeE001, Where: whereChecker},
	{ID: "V22", Rule: "Pureza transitiva", Code: conffit.CodeE002, Where: whereChecker},
	{ID: "V23", Rule: "Entrypoint fora da allowlist", Code: conffit.CodeE001, Where: whereChecker},
	{ID: "V24", Rule: "Import não resolvido", Code: conffit.CodeE003, Where: whereChecker},
	{ID: "V25", Rule: "Import dinâmico indeterminável", Code: conffit.CodeE004, Where: whereChecker},
	{ID: "V26", Rule: "Alias e barrel não mudam a aresta", Code: conffit.CodeD001, Where: whereChecker},
	{ID: "V27", Rule: "import type conta", Code: conffit.CodeD001, SingleStack: "typescript",
		Reason: "Go não tem import apagado em compilação: toda aresta é de runtime e já cai em V13..V17 (RFC §11.4, RAS-15)",
		Where:  whereRegister},
	{ID: "V28", Rule: "Código gerado não isenta", Code: conffit.CodeD001, Where: whereEdges},
	{ID: "V29", Rule: "Teste não reclassifica o SUT", Where: whereDomain},
	{ID: "V30", Rule: "Exclusão não esconde aresta", Code: conffit.CodeD001, Where: whereDomain},
	{ID: "V31", Rule: "Vedação a exactly-once E2E (P0-3)", Where: whereV31},
	{ID: "V32", Rule: "Efeito idempotente sob redelivery (P0-3)", Where: whereDistkit},
}

// Gaps lists the vectors the given stack does not realize and nobody explained:
// a single-stack vector with a Reason is an asymmetry on record (RAS-16); one
// without is a hole in the proof.
func Gaps(stack string) []Vector {
	var out []Vector
	for _, v := range Vectors {
		if v.SingleStack != "" && v.SingleStack != stack && v.Reason == "" {
			out = append(out, v)
		}
	}
	return out
}

func Lookup(id string) (Vector, bool) {
	for _, v := range Vectors {
		if v.ID == id {
			return v, true
		}
	}
	return Vector{}, false
}
