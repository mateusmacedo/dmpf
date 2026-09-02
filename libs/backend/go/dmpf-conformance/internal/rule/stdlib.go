package rule

// stdlibCapability atribui capability aos packages da biblioteca padrão.
//
// RFC §6.3 é explícita: builtins recebem capability como qualquer outra
// dependência — `net/http` é `io.network`, `crypto` é `pure`. Sem esta tabela o
// bloco `domain` poderia importar `os` e `net/http` livremente, porque a
// allowlist do manifesto só cobre dependência de terceiro.
//
// A chave é o prefixo do import path; o casamento é do prefixo mais específico.
// Package de stdlib ausente da tabela NÃO é tratado como puro: cai em
// `pure` apenas quando declarado aqui, e o default de ausência está em
// StdlibCapability.
var stdlibCapability = map[string]Capability{
	// io.filesystem
	"io/ioutil":     CapIOFilesystem, // ReadFile/WriteFile: nunca herdou `pure` de `io` por acidente
	"os":            CapIOFilesystem,
	"path/filepath": CapIOFilesystem,
	"io/fs":         CapIOFilesystem,
	"embed":         CapIOFilesystem,

	// io.network
	"net":           CapIONetwork,
	"net/http":      CapIONetwork,
	"net/url":       CapPure, // manipulação de URL é computação; o acesso é em net/http
	"net/rpc":       CapIONetwork,
	"net/smtp":      CapIONetwork,
	"net/mail":      CapPure,
	"net/textproto": CapIONetwork,

	// io.storage
	"database/sql": CapIOStorage,

	// io.clock
	"time": CapIOClock,

	// io.random
	"math/rand":    CapIORandom,
	"math/rand/v2": CapIORandom,
	"crypto/rand":  CapIORandom,

	// wire.codec
	"encoding":        CapWireCodec,
	"encoding/json":   CapWireCodec,
	"encoding/xml":    CapWireCodec,
	"encoding/gob":    CapWireCodec,
	"encoding/csv":    CapWireCodec,
	"encoding/asn1":   CapWireCodec,
	"encoding/pem":    CapWireCodec,
	"encoding/base64": CapPure, // transformação determinística de bytes
	"encoding/hex":    CapPure,
	"encoding/binary": CapPure,
	"mime":            CapWireCodec,
	"mime/multipart":  CapWireCodec,
	"text/template":   CapWireCodec,
	"html/template":   CapWireCodec,

	// observability
	"log":           CapObservability,
	"log/slog":      CapObservability,
	"runtime/pprof": CapObservability,
	"runtime/trace": CapObservability,
	"expvar":        CapObservability,
	"testing":       CapObservability,

	// runtime.framework
	"plugin":  CapRuntimeFramework,
	"os/exec": CapIOFilesystem,
	"syscall": CapIOFilesystem,
	"runtime": CapRuntimeFramework,
	"reflect": CapRuntimeFramework,
	"unsafe":  CapRuntimeFramework,

	// LIMITAÇÃO DECLARADA — packages de símbolo misto.
	//
	// `fmt` e `time` misturam símbolos puros e de I/O: `fmt.Sprintf` e
	// `fmt.Errorf` são computação, `fmt.Println` escreve em os.Stdout;
	// `time.Duration` é tipo, `time.Now()` é io.clock. RFC §3.3 fixa o PACKAGE
	// como unidade de verificação em Go, então a granularidade disponível aqui
	// não distingue os dois casos, e classificar o package inteiro como impuro
	// proibiria `fmt.Errorf` no domínio.
	//
	// A escolha é `pure`, alinhada ao `.golangci.yml` do KRN-01, que já permite
	// `fmt` e `time` no bloco `domain`. O custo é conhecido: `fmt.Println` em
	// unidade `domain` não é detectado. Distinguir por símbolo exigiria
	// type-check de cada call site, escopo que esta entrega não abre.

	// pure — computação determinística
	"errors":          CapPure,
	"fmt":             CapPure,
	"strconv":         CapPure,
	"strings":         CapPure,
	"bytes":           CapPure,
	"sort":            CapPure,
	"slices":          CapPure,
	"maps":            CapPure,
	"cmp":             CapPure,
	"iter":            CapPure,
	"math":            CapPure,
	"math/big":        CapPure,
	"math/bits":       CapPure,
	"unicode":         CapPure,
	"unicode/utf8":    CapPure,
	"unicode/utf16":   CapPure,
	"regexp":          CapPure,
	"regexp/syntax":   CapPure,
	"crypto":          CapPure,
	"crypto/sha256":   CapPure,
	"crypto/sha512":   CapPure,
	"crypto/sha1":     CapPure,
	"crypto/md5":      CapPure,
	"crypto/hmac":     CapPure,
	"crypto/subtle":   CapPure,
	"hash/fnv":        CapPure,
	"hash/crc32":      CapPure,
	"math/cmplx":      CapPure,
	"strings/builder": CapPure,
	"hash":            CapPure,
	"container/list":  CapPure,
	"container/heap":  CapPure,
	"path":            CapPure,
	"io":              CapPure,
	"bufio":           CapPure,
	"context":         CapPure,
	"sync":            CapPure,
	"sync/atomic":     CapPure,
	"structs":         CapPure,

	// Toolchain de análise: parsear e formatar código é computação
	// determinística. `go/build` fica de fora porque consulta o disco.
	"go/ast":    CapPure,
	"go/token":  CapPure,
	"go/parser": CapPure,
	"go/format": CapPure,
	"go/types":  CapPure,
	// `go/build` consulta o disco para resolver packages; o subpacote
	// `constraint` só parseia expressões de build tag.
	"go/build":            CapIOFilesystem,
	"go/build/constraint": CapPure,
}

// StdlibCapability devolve a capability de um package da biblioteca padrão.
//
// O casamento é EXATO. Herdar por prefixo é inseguro sempre que uma subárvore
// amplia a capability do prefixo — `io` é `pure` e `io/ioutil` expõe `ReadFile`
// e `WriteFile`, que são `io.filesystem`. Um prefixo permissivo classificaria a
// subárvore inteira errado, e a única forma de errar para o lado seguro é
// exigir entrada declarada.
//
// O segundo retorno é false quando o package não está na tabela: nesse caso o
// verificador NÃO sabe classificá-lo, e não saber reprova (RFC §3.6) em bloco
// default deny. Tratar desconhecido como `pure` seria a porta de entrada
// silenciosa que a política existe para fechar.
func StdlibCapability(importPath string) (Capability, bool) {
	c, ok := stdlibCapability[importPath]
	return c, ok
}
