# Go — regras core

Detalhamento das convenções básicas: formatação, naming, visibilidade, erros, context, defer, concorrência mínima.

## Formatação

`gofmt` é parte da toolchain. CI deve rejeitar PR com `gofmt -l .` retornando arquivos.

```bash
gofmt -l .                # lista arquivos com diff
gofmt -w .                # reescreve in-place
goimports -w .            # gofmt + organização de imports
gofumpt -w .              # gofmt mais estrito (opcional)
```

Tabs (não espaços) — imposto pelo gofmt. Não há configuração de "estilo" como Prettier.

## Naming

### Pacotes

- Curtos, lowercase, sem `_` ou camelCase: `user`, `httputil`, `dbtest`.
- Não repetir o nome do pacote no símbolo: `user.New`, não `user.NewUser`.
- Evitar `util`, `common`, `helpers` genéricos.

### Variáveis

- Locais e curtas: `i`, `j`, `k`, `n`, `r` (Reader), `w` (Writer), `b` (Buffer).
- **Quanto maior o escopo, mais descritivo** — `userRepository` em escopo de pacote, `r` dentro de método.
- `err` para erro retornado.
- `ctx` para `context.Context`.

```go
// Idiomático
for i, item := range items {
    if err := process(ctx, item); err != nil {
        return fmt.Errorf("item %d: %w", i, err)
    }
}
```

### Constantes

- `PascalCase` se exportada, `camelCase` se privada.
- Não há convenção `SCREAMING_SNAKE_CASE` em Go.

```go
const (
    MaxRetries     = 3
    defaultTimeout = 10 * time.Second
)
```

### Receivers

- 1-2 letras derivadas do tipo: `u *User`, `r *Repository`, `c *Client`.
- Consistência: todos os métodos de `User` usam `u` (não misturar `u`, `user`, `self`).
- Pointer receiver para mutação ou tipos grandes; value receiver para imutáveis pequenos.
- Mistura é aceitável quando faz sentido, mas mantenha consistência por tipo.

### Acrônimos

Sempre em maiúsculas:

```go
// Correto
type UserID string
type URLParser struct{}
type HTTPClient struct{}
type APIKey string

// Errado
type UserId string       // ❌
type UrlParser struct{}  // ❌
type HttpClient struct{} // ❌
```

### Getters

Em Go, getters não usam prefixo `Get`:

```go
type User struct {
    name string
}

// Idiomático
func (u *User) Name() string { return u.name }

// Não idiomático
func (u *User) GetName() string { return u.name } // ❌
```

Setters podem usar prefixo `Set` quando necessário: `SetName(...)`.

### Booleanos

```go
// Prefixos consistentes
type Permission struct {
    IsActive      bool
    HasPermission bool
    CanEdit       bool
}
```

### Interfaces

- Single-method com sufixo `er`: `Reader`, `Writer`, `Closer`, `Stringer`.
- Multi-method com nome do papel: `UserStore`, `Clock`, `EventBus`.

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type UserStore interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, u *User) error
}
```

## Visibilidade

- `PascalCase` exporta; `camelCase` é privado ao pacote. Não há `export`/`public`.
- `internal/` no path = restrito ao módulo (garantia da toolchain).
- Não há `private` para módulo dentro do mesmo pacote — granularidade é o pacote.

```
myapp/
├── internal/        # apenas myapp pode importar
│   └── user/
└── pkg/             # qualquer módulo pode importar (API pública)
    └── client/
```

## Imports

- Caminhos absolutos definidos em `go.mod`: `myapp/internal/user`.
- Sem caminhos relativos (`./foo`, `../bar`).
- `goimports` agrupa em 3 blocos automaticamente:

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"

    "myapp/internal/user"
    "myapp/internal/shared/httputil"
)
```

- Aliases somente quando há colisão ou para clareza:

```go
import (
    pgrepo "myapp/internal/infra/postgres"
    redisrepo "myapp/internal/infra/redis"
)
```

## Erros

### Padrão básico

```go
func fetchUser(ctx context.Context, id string) (*User, error) {
    if id == "" {
        return nil, errors.New("id obrigatório")
    }
    // ...
}
```

### Wrapping

```go
if err := repo.Save(ctx, u); err != nil {
    return fmt.Errorf("save user %s: %w", u.ID, err)
}
```

`%w` preserva a cadeia. `%v` apenas formata sem permitir `errors.Is`/`errors.As`.

### Sentinel errors

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
)

if errors.Is(err, ErrNotFound) {
    // ...
}
```

### Tipos customizados

```go
type ValidationError struct {
    Field string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s %s", e.Field, e.Reason)
}

var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Println(ve.Field)
}
```

### Erros com código (combina sentinel + tipo)

```go
type AppError struct {
    Code    string
    Message string
    Cause   error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Cause }
```

### Anti-patterns

```go
// Comparar por string — frágil
if err.Error() == "not found" {} // ❌

// Wrapping sem contexto
return fmt.Errorf("%w", err) // ❌ — equivale a `return err`

// Ignorar erro
data, _ := json.Marshal(input) // ❌

// Panic em código de aplicação
if err != nil { panic(err) } // ❌
```

## Context

```go
// Convenção: primeiro parâmetro
func (s *Service) DoWork(ctx context.Context, in Input) error {
    // Propagar para chamadas filhas
    if err := s.repo.Save(ctx, in); err != nil {
        return err
    }
    return nil
}
```

### Timeout

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

### Valores

```go
type ctxKey string
const userKey ctxKey = "user"

ctx = context.WithValue(ctx, userKey, currentUser)
user, _ := ctx.Value(userKey).(*User)
```

Use chaves tipadas (não `string` cru) para evitar colisão.

### Anti-patterns

- `context.Background()` em código de request — perde cancelamento upstream.
- `context.TODO()` em código de produção — apenas placeholder de desenvolvimento.
- Armazenar `Context` em struct — passe como argumento.

## Defer

`defer` empilha — última chamada executa primeiro. Útil para cleanup pareado:

```go
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()

mu.Lock()
defer mu.Unlock()

rows, err := db.Query(q)
if err != nil {
    return err
}
defer rows.Close()
```

### Cuidados

- `defer` em loop apertado acumula custo — chame manualmente quando relevante.
- `defer` captura argumentos no momento do `defer`, não da execução:

```go
i := 1
defer fmt.Println(i)  // imprime "1"
i = 2                 // muda i, mas defer já capturou
```

- Para capturar valor atualizado, use closure:

```go
defer func() { fmt.Println(i) }()
```

## Funções

### Múltiplos retornos

```go
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}
```

### Named returns (use com moderação)

```go
func parse(input string) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    // ...
    return
}
```

Útil para defer + recover ou documentação. Em outros casos, retornos explícitos são mais claros.

### Variadic

```go
func Sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

Sum(1, 2, 3)
nums := []int{1, 2, 3}
Sum(nums...) // "spread"
```

## Slices, maps, structs

### Slice

```go
s := []int{}            // não-nil, len 0
var s []int             // nil, len 0
s := make([]int, 0, 10) // capacidade pré-alocada

s = append(s, 1)        // pode reslicear
```

### Map

```go
m := map[string]int{}
m := make(map[string]int)

v, ok := m["key"]       // ok detecta presença

delete(m, "key")
```

### Struct literal

```go
u := User{Name: "Maria", Email: "maria@x.com"}  // nomeado
u := User{"Maria", "maria@x.com"}               // posicional — frágil
```

Sempre prefira nomeado para resistir a mudanças de ordem.

### Zero value útil

```go
var b bytes.Buffer
b.WriteString("hello")  // funciona sem inicialização

var u User
u.Name = "Maria"        // ok se zero value for válido
```

Projete tipos para terem zero value útil quando possível.

## Concorrência básica

### Goroutine

```go
go func() {
    if err := process(ctx); err != nil {
        log.Println(err)
    }
}()
```

Goroutine sem `context` é candidata a leak. Sempre passe `ctx`.

### Channel

```go
ch := make(chan int, 10) // buffered

go func() {
    defer close(ch)
    for i := 0; i < 10; i++ {
        ch <- i
    }
}()

for v := range ch {
    fmt.Println(v)
}
```

### sync.WaitGroup

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(item Item) {
        defer wg.Done()
        process(item)
    }(item)
}
wg.Wait()
```

### Mutex / RWMutex

```go
type Counter struct {
    mu    sync.Mutex
    count int
}

func (c *Counter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

`RWMutex` quando há muito mais leituras que escritas.

### errgroup (preferido para paralelismo com erro)

```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(ctx)
for _, url := range urls {
    url := url
    g.Go(func() error {
        return fetch(ctx, url)
    })
}
if err := g.Wait(); err != nil {
    return err
}
```

## Documentação

Funções/tipos exportados começam o comentário pelo nome:

```go
// User represents a user account.
type User struct{ /* ... */ }

// FindByID retorna o usuário com o ID informado ou ErrNotFound.
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) { /* ... */ }
```

Pacote tem comentário em `doc.go` ou no primeiro arquivo:

```go
// Package user implementa o domínio de contas de usuário.
package user
```

## Linters recomendados

`golangci-lint` agrega vários:

- `gofmt`/`goimports` — formatação.
- `errcheck` — erros não verificados.
- `govet` — análise estática stdlib.
- `staticcheck` — análise avançada.
- `unused` — código morto.
- `gocyclo`/`gocognit`/`funlen` — complexidade.
- `gosec` — vulnerabilidades.
- `revive` — substituto de `golint` (deprecated).
- `contextcheck` — `Context` propagado.
- `bodyclose` — `resp.Body.Close()` esquecido.
- `sqlclosecheck` — `rows.Close()` esquecido.

Configurar via `.golangci.yml` no root do repo.
