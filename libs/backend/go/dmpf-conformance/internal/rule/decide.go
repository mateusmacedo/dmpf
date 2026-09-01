package rule

// Endpoint é o lado de uma aresta já classificado: o bloco, o bounded context e
// a superfície pública da unidade que contém o package.
type Endpoint struct {
	CanonicalKey             string
	Block                    Block
	BoundedContext           string
	PublicIntegrationSurface bool
}

// Decision é o resultado de decide(): C1 e C2 avaliadas de forma INDEPENDENTE,
// como exige RFC §7.1. Uma aresta pode reprovar nas duas ao mesmo tempo.
type Decision struct {
	C1 bool
	C2 bool
}

// Allowed reporta a conjunção. `P` na matriz não é autorização final: a aresta
// permitida por C1 continua sujeita a C2 e à política de capabilities (§6).
func (d Decision) Allowed() bool { return d.C1 && d.C2 }

// SameBoundedContext é o predicado de RFC §7.2: comparação exata de strings.
func SameBoundedContext(source, target Endpoint) bool {
	return source.BoundedContext == target.BoundedContext
}

// PublicIntegrationSurface é o segundo termo de C2 (RFC §7.2). Um contract
// package é superfície pública por construção; qualquer unidade pode declará-la,
// exceto `domain` — o manifesto reprova essa declaração com M002 antes daqui.
func PublicIntegrationSurface(target Endpoint) bool {
	return target.Block == BlockContract || target.PublicIntegrationSurface
}

// Decide é a função de RFC §7.1:
// decide(source_block, target_block, source_bc, target_bc, target_surface).
// C1 é a matriz de §7.3; C2 é `same_bounded_context OR public_integration_surface`.
func Decide(source, target Endpoint) Decision {
	return Decision{
		C1: AllowedByMatrix(source.Block, target.Block),
		C2: SameBoundedContext(source, target) || PublicIntegrationSurface(target),
	}
}

// DiagnoseEdge traduz a decisão nos diagnósticos de RFC §10.3. C1 falsa emite
// DMPF-D001; C2 falsa emite DMPF-D002; as duas falsas emitem os dois.
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
