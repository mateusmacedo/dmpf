---
name: skill-go
description: >-
  Use esta skill ao escrever, editar ou revisar código Go (.go) de backend.
  Reúne convenções comuns: estrutura `internal/`, naming idiomático (acrônimos
  em maiúsculas, receivers curtos, sem prefixo `Get`), erros como valores com
  wrapping, "Accept interfaces, return structs", interfaces no consumidor,
  context.Context propagado, struct tags (`json`, `db`, `validate`), generics
  com constraints, sem getters/setters triviais. Use também quando o usuário
  mencionar erros de tipo, generics, type assertion, type switch, channels,
  goroutines, errors.Is/As, embedding ou ao tipar entities, repositories, DTOs,
  use cases ou respostas de API em Go.
model: opus
---

# Go — padrões e convenções

Guia de referência com padrões idiomáticos de Go em backend. Adapte ao padrão vigente do projeto — quando houver divergência, preserve o padrão local.

## Preferências recomendadas

- `gofmt`/`goimports` aplicados automaticamente (CI deve falhar em diff).
- Estrutura `cmd/<app>/main.go` + `internal/<contexto>/...` para módulos privados.
- Receivers curtos (1-2 letras): `u *User`, não `this`/`self`.
- Acrônimos em maiúsculas: `UserID`, `URLParser`, `HTTPClient` (não `UserId`/`UrlParser`/`HttpClient`).
- Sem prefixo `Get` em getters: `User.Name()`, não `User.GetName()`.
- Booleanos com prefixo: `IsActive`, `HasPermission`, `CanEdit`.
- Funções/tipos exportados (`PascalCase`); privados ao pacote (`camelCase`).
- Erros como valores: funções retornam `(T, error)` e quem chama trata.
- `context.Context` como primeiro parâmetro em I/O, RPC e queries.
- "Accept interfaces, return structs" — interfaces no consumidor.

## Regras comuns (ajustáveis)

| Regra | Enforcement típico |
|-------|--------------------|
| `gofmt`/`goimports` aplicado | CI + pre-commit |
| `go vet ./...` sem warnings | CI |
| Erros não ignorados | `errcheck` (golangci-lint) |
| Imports não usados | `go vet` |
| Funções pequenas | `funlen`, `gocyclo`, `gocognit` |
| Sem variáveis sombreadas | `govet -shadow` |
| Sem `interface{}` em hot path | revisão + benchmark |
| Receivers consistentes por tipo | revisão |
| Context.Context propagado | `contextcheck` |
| Static analysis | `staticcheck` |
| Race-free | `go test -race` |
| CVEs alcançadas | `govulncheck ./...` |

## Naming sugerido

| Categoria | Convenção | Exemplo |
|-----------|-----------|---------|
| Pacote | `lowercase`, curto, sem `_` | `user`, `httputil`, `dbtest` |
| Tipo exportado | `PascalCase` | `User`, `UserRepository` |
| Tipo privado | `camelCase` | `userRow`, `pgRepo` |
| Função/método exportado | `PascalCase` | `FindByID`, `NewUser` |
| Função/método privado | `camelCase` | `toDomain`, `validate` |
| Constante | `PascalCase` ou `camelCase` | `MaxRetries`, `defaultTimeout` |
| Variável local | `camelCase` curto | `u`, `ctx`, `err`, `cnt` |
| Receiver | 1-2 letras | `u *User`, `r *Repo` |
| Interface | sufixo `er` (1-método) | `Reader`, `Saver` |
| Interface multi-método | nome do papel | `UserStore`, `Clock` |

## Estrutura de projeto sugerida

```
.
├── cmd/
│   └── api/
│       └── main.go               # composition root
├── internal/
│   ├── user/                     # bounded context
│   │   ├── user.go               # entidade
│   │   ├── repository.go         # interface
│   │   ├── usecase_create.go
│   │   └── http.go               # handler
│   ├── infra/
│   │   ├── postgres/
│   │   │   └── user_repository.go
│   │   └── redis/
│   └── shared/
│       └── httputil/
├── go.mod
└── go.sum
```

## Erros

- Funções com falha possível retornam `(T, error)`.
- Wrap com contexto: `fmt.Errorf("operation: %w", err)` (preserva cadeia).
- Sentinelas: `var ErrNotFound = errors.New("not found")`.
- Tipos customizados: `type ValidationError struct { ... }` com método `Error()`.
- Identificação: `errors.Is(err, ErrNotFound)` para sentinelas; `errors.As(err, &ve)` para tipos.
- `panic` apenas para programming errors (precondição violada); nunca para fluxo.

## Concorrência (resumo)

- Goroutines sempre com `context.Context` para cancelamento.
- `errgroup` para paralelismo com erro propagado.
- Channel para coordenação; `sync.Mutex`/`RWMutex`/`atomic` para estado pequeno.
- Sempre `defer close(ch)` ou planejar fim explícito.
- `go test -race` em CI.

## Imports

- Caminhos absolutos via `go.mod`: `myapp/internal/user`.
- `goimports` agrupa em três blocos: stdlib, externos, internos.
- Aliases (`alias "longo/path"`) só quando há colisão.
- Sem cycle de imports — Go falha em build.

## Generics (Go 1.18+)

```go
func Map[T, U any](s []T, f func(T) U) []U {
    out := make([]U, len(s))
    for i, v := range s {
        out[i] = f(v)
    }
    return out
}

type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](xs []T) T {
    var total T
    for _, x := range xs {
        total += x
    }
    return total
}
```

Use generics para coleções/utilitários genuinamente polimórficos. Para casos simples, função concreta é mais clara.

## Referências detalhadas (carregar sob demanda)

| Arquivo | Conteúdo |
|---------|----------|
| `references/core-rules.md` | gofmt, naming, visibility, erros, context, defer |
| `references/advanced-types.md` | Generics, interfaces, type assertions/switches, embedding |
| `references/data-patterns.md` | Validação, structs JSON, services, use cases, errgroup |
| `references/backend-patterns.md` | HTTP handlers, repositórios, middleware, DI, configuração |

## Checklist

- [ ] `gofmt`/`goimports` aplicado (sem diff).
- [ ] `go vet ./...` sem warnings.
- [ ] `golangci-lint run` passa (errcheck, staticcheck, gocyclo).
- [ ] `go test ./... -race` passa.
- [ ] Naming idiomático: `UserID`, receivers 1-2 letras, sem prefixo `Get`.
- [ ] Erros propagados com `%w`; sentinelas e tipos onde fizer sentido.
- [ ] `context.Context` como primeiro parâmetro em I/O.
- [ ] Interfaces declaradas no consumidor, structs concretas no implementador.
- [ ] Sem `interface{}` em hot path.
- [ ] Goroutines com `context` para cancelamento.
- [ ] `defer rows.Close()` em queries.
- [ ] Tags `json`/`db`/`validate` consistentes.
- [ ] Funções/tipos exportados com comentário godoc iniciando pelo nome.
- [ ] `govulncheck ./...` em CI.
