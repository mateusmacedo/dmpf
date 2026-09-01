package rule

import "slices"

// Capability é a natureza do acesso que uma dependência externa dá ao código
// (RFC §6.1). O conjunto é fechado.
type Capability string

const (
	CapPure             Capability = "pure"
	CapIOStorage        Capability = "io.storage"
	CapIOMessaging      Capability = "io.messaging"
	CapIONetwork        Capability = "io.network"
	CapIOFilesystem     Capability = "io.filesystem"
	CapIOClock          Capability = "io.clock"
	CapIORandom         Capability = "io.random"
	CapRuntimeFramework Capability = "runtime.framework"
	CapWireCodec        Capability = "wire.codec"
	CapObservability    Capability = "observability"
)

var capabilities = []Capability{
	CapIOStorage, CapIOMessaging, CapIONetwork, CapIOFilesystem,
	CapIOClock, CapIORandom, CapRuntimeFramework, CapWireCodec,
	CapObservability, CapPure,
}

// Capabilities devolve o conjunto fechado de RFC §6.1, na ordem da tabela.
func Capabilities() []Capability {
	out := make([]Capability, len(capabilities))
	copy(out, capabilities)
	return out
}

// IsCapability reporta se o valor pertence ao conjunto fechado de RFC §6.1.
func IsCapability(c Capability) bool { return slices.Contains(capabilities, c) }

// capabilityPolicy é a transcrição da tabela de RFC §6.2, mantida como dado
// versionado pelo mesmo motivo que a matriz 6×6: é norma revisável em PR, não
// condicional espalhada pelo código.
//
//	domain       default deny  -> apenas pure
//	port         default deny  -> apenas pure (assinatura de porta não expõe driver, SDK nem wire)
//	application  default deny  -> pure e observability; nenhuma io.*, runtime.framework nem wire.codec
//	contract     restrita      -> pure e wire.codec
//	provider     permissiva    -> qualquer
//	app          permissiva    -> qualquer, no composition root
//
// `observability` fica fora de `domain` e de `port` ainda que a biblioteca seja
// tecnicamente pura: o princípio 11 é explícito, telemetria vive em adapters e
// providers. É a exceção que mais se pede, e por isso está declarada.
var capabilityPolicy = map[Block][]Capability{
	BlockDomain:      {CapPure},
	BlockPort:        {CapPure},
	BlockApplication: {CapPure, CapObservability},
	BlockContract:    {CapPure, CapWireCodec},
}

// permissiveBlocks são os blocos sem restrição de capability (RFC §6.2).
var permissiveBlocks = []Block{BlockProvider, BlockApp}

// isPermissiveBlock reporta se o bloco aceita qualquer capability (RFC §6.2).
func isPermissiveBlock(b Block) bool { return slices.Contains(permissiveBlocks, b) }

// CapabilityAllowed reporta se o bloco pode usar a capability. Bloco fora do
// conjunto fechado devolve false — o default de toda ramificação ausente é
// reprovar.
func CapabilityAllowed(b Block, c Capability) bool {
	if !IsBlock(b) || !IsCapability(c) {
		return false
	}
	if isPermissiveBlock(b) {
		return true
	}
	return slices.Contains(capabilityPolicy[b], c)
}

// AllowlistEntry é uma entrada da allowlist do manifesto (RFC §6.3): o pacote,
// a faixa de versões, os entrypoints permitidos e a capability atribuída.
//
// A atribuição de capability é DECLARADA, nunca inferida do nome do pacote
// (RFC §6.1).
type AllowlistEntry struct {
	Package     string
	Entrypoints []string
	Capability  Capability
	Versions    string
}

// EntrypointRoot é como o exemplo canônico de RFC §10.1 declara a raiz do
// pacote: `"entrypoints": ["."]`.
const EntrypointRoot = "."

// Covers reporta se a entrada autoriza este import path.
//
// A raiz precisa ser DECLARADA como ".": lista vazia não autoriza nada. A regra
// dos entrypoints é o que torna a allowlist utilizável — um pacote grande pode
// ter subpath puro e subpath impuro, e só o primeiro é declarado —, e um
// default implícito para a raiz devolveria a autorização que a regra retira.
func (e AllowlistEntry) Covers(importPath string) bool {
	for _, ep := range e.Entrypoints {
		if ep == EntrypointRoot {
			if importPath == e.Package {
				return true
			}
			continue
		}
		if importPath == ep {
			return true
		}
	}
	return false
}

// faltando lista os elementos ausentes, para a mensagem do diagnóstico.
func (e AllowlistEntry) faltando() []string {
	var out []string
	if e.Package == "" {
		out = append(out, "package")
	}
	if e.Versions == "" {
		out = append(out, "versions")
	}
	if len(e.Entrypoints) == 0 {
		out = append(out, "entrypoints")
	}
	if !IsCapability(e.Capability) {
		out = append(out, "capability")
	}
	return out
}

// Complete reporta se a entrada declara os quatro elementos que RFC §6.3 exige:
// identificador do pacote, faixa de versões, entrypoints permitidos e
// capability atribuída. Entrada incompleta não autoriza nada.
func (e AllowlistEntry) Complete() bool {
	return e.Package != "" && e.Versions != "" &&
		len(e.Entrypoints) > 0 && IsCapability(e.Capability)
}

// ExceptionEntry é uma exceção NOMINAL (RFC §6.4): o par (unidade, dependência)
// com justificativa, owner e data de revisão. Exceção por categoria, prefixo ou
// diretório é proibida — a forma do registro é o que impede que ela vire
// política paralela não revisada.
type ExceptionEntry struct {
	Unit       string
	Dependency string
	Reason     string
	Owner      string
	ReviewBy   string
}

// Valid reporta se a exceção tem os cinco elementos que a tornam nominal.
// Exceção incompleta não é exceção: não autoriza nada.
func (e ExceptionEntry) Valid() bool {
	return e.Unit != "" && e.Dependency != "" && e.Reason != "" &&
		e.Owner != "" && e.ReviewBy != ""
}
