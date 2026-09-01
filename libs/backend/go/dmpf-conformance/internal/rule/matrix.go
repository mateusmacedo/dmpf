package rule

import "slices"

// Block é um dos seis blocos de RFC §4.1. O conjunto é FECHADO: valor fora dele
// não é ignorado nem tratado como não classificado — reprova com DMPF-M002.
type Block string

const (
	BlockDomain      Block = "domain"
	BlockApplication Block = "application"
	BlockApp         Block = "app"
	BlockPort        Block = "port"
	BlockProvider    Block = "provider"
	BlockContract    Block = "contract"
)

// blocks é o conjunto fechado, na ordem das linhas e colunas de RFC §7.3.
var blocks = []Block{
	BlockDomain,
	BlockApplication,
	BlockApp,
	BlockPort,
	BlockProvider,
	BlockContract,
}

// Blocks devolve o conjunto fechado dos seis blocos, na ordem normativa.
func Blocks() []Block {
	out := make([]Block, len(blocks))
	copy(out, blocks)
	return out
}

// IsBlock reporta se o valor pertence ao conjunto fechado de RFC §4.1.
func IsBlock(v Block) bool {
	return slices.Contains(blocks, v)
}

// matrixRow é uma linha da matriz 6×6 de RFC §7.3, na ordem de Blocks().
type matrixRow [6]bool

// Os dois valores da matriz, nomeados como a RFC os grafa. Nomes longos
// justamente para não colidirem com identificadores locais do package.
const (
	permitida = true  // P — ainda sujeita a C2 e à política de capabilities
	proibida  = false // ✗
)

// matrix é a transcrição literal da matriz 6×6 de RFC §7.3, mantida como dado
// versionado e revisável em PR — não como switch espalhado pelo código.
// Linha é origem, coluna é destino, ambas na ordem de Blocks().
//
//	De ↓ / Para →   domain  application  app  port  provider  contract
//	domain             P         ✗        ✗     ✗       ✗         ✗
//	application        P         P        ✗     P       ✗         ✗
//	app                P         P        P     P       P         P
//	port               P         ✗        ✗     P       ✗         ✗
//	provider           P         ✗        ✗     P       P         P
//	contract           ✗         ✗        ✗     ✗       ✗         P
var matrix = map[Block]matrixRow{
	BlockDomain:      {permitida, proibida, proibida, proibida, proibida, proibida},
	BlockApplication: {permitida, permitida, proibida, permitida, proibida, proibida},
	BlockApp:         {permitida, permitida, permitida, permitida, permitida, permitida},
	BlockPort:        {permitida, proibida, proibida, permitida, proibida, proibida},
	BlockProvider:    {permitida, proibida, proibida, permitida, permitida, permitida},
	BlockContract:    {proibida, proibida, proibida, proibida, proibida, permitida},
}

// blockIndex devolve a posição do bloco na ordem de Blocks().
func blockIndex(b Block) (int, bool) {
	i := slices.Index(blocks, b)
	return i, i >= 0
}

// AllowedByMatrix é a condição C1 de RFC §7.1: o par (origem, destino) é
// PERMITIDA na matriz de §7.3. Bloco fora do conjunto fechado devolve false —
// o default de toda ramificação ausente é reprovar.
func AllowedByMatrix(source, target Block) bool {
	row, ok := matrix[source]
	if !ok {
		return false
	}
	j, ok := blockIndex(target)
	if !ok {
		return false
	}
	return row[j]
}
