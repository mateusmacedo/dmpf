package rule

import "slices"

// O conjunto é fechado: valor fora dele não é ignorado nem
// tratado como não classificado: reprova com DMPF-M002.
type Block string

const (
	BlockDomain      Block = "domain"
	BlockApplication Block = "application"
	BlockApp         Block = "app"
	BlockPort        Block = "port"
	BlockProvider    Block = "provider"
	BlockContract    Block = "contract"
)

// A ordem é a das linhas e colunas da matriz abaixo, e matrixRow depende dela.
var blocks = []Block{
	BlockDomain,
	BlockApplication,
	BlockApp,
	BlockPort,
	BlockProvider,
	BlockContract,
}

func Blocks() []Block {
	out := make([]Block, len(blocks))
	copy(out, blocks)
	return out
}

func IsBlock(v Block) bool {
	return slices.Contains(blocks, v)
}

type matrixRow [6]bool

const (
	permitida = true  // P — ainda sujeita a C2 e à política de capabilities
	proibida  = false // ✗
)

// Quais dependências entre blocos são permitidas, transcrito da norma como dado
// revisável em PR e não como condicional espalhada. Linha é origem, coluna é
// destino.
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

func blockIndex(b Block) (int, bool) {
	i := slices.Index(blocks, b)
	return i, i >= 0
}

// Metade da decisão: se o par de blocos é permitido, ignorando qual contexto
// cada lado habita. Bloco desconhecido devolve false — o default de toda
// ramificação ausente é reprovar.
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
