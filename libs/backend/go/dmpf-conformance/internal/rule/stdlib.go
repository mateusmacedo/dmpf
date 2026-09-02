package rule

// Os builtins são tratados como qualquer outra dependência: `net/http` dá
// acesso à rede, `crypto` é computação. Sem esta tabela o bloco `domain`
// importaria `os` livremente, porque a allowlist do manifesto só cobre
// dependência de terceiro.
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

	// `fmt` e `time` misturam símbolo puro e de I/O: `Sprintf` e `Errorf`
	// computam, `Println` escreve na saída; `Duration` é tipo, `Now()` lê o
	// relógio. A verificação é por package, e nessa granularidade não dá para
	// separar — marcá-los impuros proibiria `fmt.Errorf` no domínio.
	//
	// Ficam como puros, o que o lint local já permitia. O custo é conhecido:
	// `fmt.Println` numa unidade de domínio passa sem detecção.

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

	// Parsear e formatar código é determinístico; `go/build` consulta o disco.
	"go/ast":              CapPure,
	"go/token":            CapPure,
	"go/parser":           CapPure,
	"go/format":           CapPure,
	"go/types":            CapPure,
	"go/build":            CapIOFilesystem,
	"go/build/constraint": CapPure,
}

// StdlibCapability casa por import path EXATO: herdar por prefixo classifica
// errado sempre que a subárvore amplia a capability do prefixo — `io` é `pure`
// e `io/ioutil` expõe `ReadFile`. Ausência devolve false, e não saber reprova
// em bloco que nega por padrão.
func StdlibCapability(importPath string) (Capability, bool) {
	c, ok := stdlibCapability[importPath]
	return c, ok
}
