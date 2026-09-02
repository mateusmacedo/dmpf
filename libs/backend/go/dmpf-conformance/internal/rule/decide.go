package rule

type Endpoint struct {
	CanonicalKey             string
	Block                    Block
	BoundedContext           string
	PublicIntegrationSurface bool
}

// As duas condições são independentes: uma aresta pode reprovar nas duas.
type Decision struct {
	C1 bool
	C2 bool
}

// Estar permitido na matriz não é autorização final: a aresta segue sujeita ao
// contexto e à política de dependências externas.
func (d Decision) Allowed() bool { return d.C1 && d.C2 }

// Comparação exata de strings, sem normalização.
func SameBoundedContext(source, target Endpoint) bool {
	return source.BoundedContext == target.BoundedContext
}

// Um contract é superfície pública por construção. Em `domain` a declaração é
// inválida, e o manifesto já a reprovou antes de chegar aqui.
func PublicIntegrationSurface(target Endpoint) bool {
	return target.Block == BlockContract || target.PublicIntegrationSurface
}

// A aresta passa se o par de blocos é permitido E os dois lados podem se falar.
func Decide(source, target Endpoint) Decision {
	return Decision{
		C1: AllowedByMatrix(source.Block, target.Block),
		C2: SameBoundedContext(source, target) || PublicIntegrationSurface(target),
	}
}

// C1 falsa emite D001, C2 falsa emite D002, as duas falsas emitem os dois.
func DiagnoseEdge(source, target Endpoint, sourceFile string) []Diagnostic {
	d := Decide(source, target)
	var out []Diagnostic
	if !d.C1 {
		out = append(out, Diagnostic{
			Code:         CodeD001,
			CanonicalKey: source.CanonicalKey,
			Target:       target.CanonicalKey,
			SourceFile:   sourceFile,
			Detail:       "bloco " + string(source.Block) + " não pode depender de " + string(target.Block),
		})
	}
	if !d.C2 {
		out = append(out, Diagnostic{
			Code:         CodeD002,
			CanonicalKey: source.CanonicalKey,
			Target:       target.CanonicalKey,
			SourceFile:   sourceFile,
			Detail:       "bounded context " + source.BoundedContext + " não pode depender de " + target.BoundedContext + " sem superfície pública",
		})
	}
	return out
}
