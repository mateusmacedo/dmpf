package rule

import "slices"

// Capability é a natureza do acesso que a dependência dá ao código.
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

// Na ordem da tabela normativa, que é a que a documentação usa.
func Capabilities() []Capability {
	out := make([]Capability, len(capabilities))
	copy(out, capabilities)
	return out
}

func IsCapability(c Capability) bool { return slices.Contains(capabilities, c) }

// Que tipo de acesso externo cada bloco tolera, como dado versionado pelo mesmo
// motivo da matriz: norma revisável em PR, não condicional espalhada.
//
// Telemetria fica fora de `domain` e `port` ainda que a biblioteca de log seja
// tecnicamente pura — a exceção que mais se pede, e por isso explícita. Em
// `application` ela passa: a norma nega io, framework e wire ali, e nada diz
// sobre telemetria. É leitura do complemento, e a única desta entrega; o
// racional e as alternativas estão em docs/adr/031.
var capabilityPolicy = map[Block][]Capability{
	BlockDomain:      {CapPure},
	BlockPort:        {CapPure},
	BlockApplication: {CapPure, CapObservability},
	BlockContract:    {CapPure, CapWireCodec},
}

var permissiveBlocks = []Block{BlockProvider, BlockApp}

func isPermissiveBlock(b Block) bool { return slices.Contains(permissiveBlocks, b) }

// CapabilityAllowed reprova valor fora do conjunto fechado: o default de toda
// ramificação ausente é reprovar.
func CapabilityAllowed(b Block, c Capability) bool {
	if !IsBlock(b) || !IsCapability(c) {
		return false
	}
	if isPermissiveBlock(b) {
		return true
	}
	return slices.Contains(capabilityPolicy[b], c)
}

// AllowlistEntry é uma entrada de `external[]` no manifesto. A capability é
// sempre declarada, nunca inferida do nome do pacote.
type AllowlistEntry struct {
	Package     string
	Entrypoints []string
	Capability  Capability
	Versions    string
}

// Para autorizar a raiz do pacote, declare `"entrypoints": ["."]`.
const EntrypointRoot = "."

// Covers exige a raiz DECLARADA como ".": um default implícito devolveria a
// autorização que a regra dos entrypoints retira de pacote com subpath impuro.
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

// Sem os quatro — pacote, faixa de versões, entrypoints e capability — a
// entrada não descreve o que autoriza, e não autoriza nada.
func (e AllowlistEntry) Complete() bool {
	return e.Package != "" && e.Versions != "" &&
		len(e.Entrypoints) > 0 && IsCapability(e.Capability)
}

// Exceção nomeada: um par (unidade, dependência) por vez. Exceção por
// categoria, prefixo ou diretório é proibida — a forma do registro é o que
// impede que ela vire política paralela não revisada.
type ExceptionEntry struct {
	Unit       string
	Dependency string
	Reason     string
	Owner      string
	ReviewBy   string
}

// Valid: exceção incompleta não é exceção, e não autoriza nada.
func (e ExceptionEntry) Valid() bool {
	return e.Unit != "" && e.Dependency != "" && e.Reason != "" &&
		e.Owner != "" && e.ReviewBy != ""
}
