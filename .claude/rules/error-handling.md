---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Tratamento de erros

## Hierarquia de erros

Um padrão comum é estender uma classe base para erros de domínio:

```
DomainError (base)
├── NotFoundError
├── UnauthorizedError
├── ValidationError
├── ConflictError
└── ForbiddenError
```

Erros de domínio são lançados nas camadas internas (use cases, serviços) e capturados onde faz sentido — por exemplo, controllers, handler global ou exception filter. A forma exata depende do framework.

## Formato de resposta (exemplo)

```ts
// Sucesso
{ success: true, data: { ... } }

// Erro
{ success: false, error: 'Mensagem amigável', statusCode: 400 }
```

Esse shape é apenas um exemplo. Alguns projetos preferem respostas no padrão REST idiomático (sem envelope), outros adotam `problem+json`. Use o padrão definido pelo projeto.

## Helpers de resposta HTTP (exemplo)

Métodos auxiliares facilitam a escrita dos controllers:

```ts
ok(data)          // 200
created(data)     // 201
badRequest(msg)   // 400
unauthorized(msg) // 401
notFound(msg)     // 404
conflict(msg)     // 409
```

## Regras duras

- Não use catch vazio (`catch {}`) sem tratamento nem log deliberado.
- Não exponha detalhes internos (stack trace, mensagens do banco) diretamente ao cliente.
- Registre erros com contexto útil (identificadores do request, endpoint, payload já sanitizado).
- Retorne status HTTP compatível com o tipo do erro.

---

## 🔹 Go: tratamento de erros

Go trata erros como valores, não como exceções. O fluxo é estruturalmente diferente.

### Hierarquia via interface e wrapping

Não há classes; o contrato é `error` (interface com `Error() string`). A "hierarquia" se constrói com tipos próprios e wrapping:

```go
type DomainError struct {
    Code    string
    Message string
    Cause   error
}

func (e *DomainError) Error() string { return e.Message }
func (e *DomainError) Unwrap() error { return e.Cause }

// Erros sentinela para identificação
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrConflict     = errors.New("conflict")
)
```

Identificação no caller:

```go
if errors.Is(err, ErrNotFound) { ... }

var domainErr *DomainError
if errors.As(err, &domainErr) { ... }
```

### Propagação

- Funções devolvem `(T, error)`; o caller decide o que fazer.
- Wrapping com `fmt.Errorf("contexto: %w", err)` preserva a cadeia para `errors.Is`/`errors.As`.
- `panic` é reservado para erros de programação (nil pointer impossível, invariante violada). Não é fluxo de erro normal.

### Formato de resposta HTTP

O padrão é o mesmo (envelope, problem+json, REST puro). O que muda é a forma de mapear:

```go
func toHTTPStatus(err error) int {
    switch {
    case errors.Is(err, ErrNotFound):     return http.StatusNotFound
    case errors.Is(err, ErrUnauthorized): return http.StatusUnauthorized
    case errors.Is(err, ErrConflict):     return http.StatusConflict
    default:                              return http.StatusInternalServerError
    }
}
```

Middlewares dos frameworks (Gin, Echo, Chi) costumam centralizar esse mapeamento.

### Regras duras (Go)

- Não use `_ = err` para descartar erro silenciosamente — o linter (`errcheck`) já reclama. Se for proposital, comentário explicando.
- Não exponha mensagens de erro do banco, do `database/sql` ou de libs internas direto ao cliente.
- Wrappe com contexto a cada camada: `fmt.Errorf("user repo: find by id: %w", err)`.
- Use `recover` apenas em fronteiras (handler HTTP, worker de fila) para virar `panic` em log + 500.
- Loggar erro apenas onde é tratado, não em cada camada (evita log duplicado).
