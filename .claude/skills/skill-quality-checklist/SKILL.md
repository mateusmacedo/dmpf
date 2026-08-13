---
name: skill-quality-checklist
description: |
  Use esta skill quando o usuário pedir para "verificar qualidade", "checklist de qualidade",
  "validar antes de deploy", "revisar segurança", "verificar performance",
  ou mencionar validação pré-release, quality gates ou revisão completa.
  Cobre verificações de segurança, robustez, performance, banco de dados, APIs, filas e código.
  Para review detalhado de PR, ver `skill-code-review`.
model: sonnet
---

# Checklist de qualidade

## Objetivo
Checklist para revisões pré-release: segurança, robustez, performance, banco de dados, APIs, filas e código.

## Quando usar
- Antes de finalizar uma entrega ou PR.
- Ao validar requisitos de qualidade em review.
- Ao preparar release ou deploy.

Antes de considerar um código pronto, vale passar pelos pontos abaixo.

## Segurança

- [ ] Input externo validado/saneado (body, query, headers, APIs).
- [ ] Sem exposição de dados sensíveis (tokens, stack traces, ids internos).
- [ ] Riscos considerados: XSS, CSRF, SQL/NoSQL injection.
- [ ] Autenticação e autorização aplicadas nas rotas.
- [ ] Rate limiting em endpoints públicos.

## Robustez

- [ ] Erros tratados com fallback gracioso.
- [ ] Falhas de rede consideradas (timeout, retry, circuit breaker).
- [ ] Mensagens de erro amigáveis (sem expor internos).
- [ ] DTOs validados nos controllers/routes.

## Performance

- [ ] Queries otimizadas (sem N+1, joins/selects adequados).
- [ ] Índices nos campos de busca/filtro frequentes.
- [ ] Paginação em listagens.
- [ ] Cache aplicado onde faz sentido.

## Banco de dados

- [ ] Migrations testadas (up e down).
- [ ] Connection pool dimensionado.
- [ ] Transações para operações que precisam ser atômicas.
- [ ] Campos obrigatórios com NOT NULL e defaults corretos.

## API

- [ ] Response DTOs definem o que é exposto.
- [ ] Status HTTP consistentes (201 para criação, 204 para delete, etc.).
- [ ] Paginação com metadados (total, page, limit).
- [ ] Versionamento de API respeitado (quando aplicável).

## Filas

- [ ] Error handling nos consumers/workers.
- [ ] Retries com backoff exponencial.
- [ ] Dead-letter queue para jobs com falhas repetidas.
- [ ] Concorrência adequada à capacidade do worker.

## Resiliência

- [ ] Graceful shutdown (fecha conexões, drena filas).
- [ ] Falhas de conexão com DB tratadas (reconexão).
- [ ] Timeouts em chamadas externas.
- [ ] Health check disponível.

## Código

- [ ] TypeScript strict (evitar `any`).
- [ ] Formatação segue o padrão do projeto.
- [ ] Comentários apenas para decisões não óbvias.
- [ ] Imports organizados.

## Exemplos

### Validação de input

```typescript
// Sem validação
const userId = req.query.id
await db.users.find(userId)

// Validado
const userId = z.string().uuid().parse(req.query.id)
await db.users.find(userId)
```

### Erro gracioso

```typescript
// Expõe detalhes internos
try {
  const data = await api.fetch()
} catch (error) {
  return { error: error.message }
}

// Log interno + mensagem amigável
try {
  const data = await api.fetch()
} catch (error) {
  console.error('API error:', error)
  return { error: 'Não foi possível carregar os dados' }
}
```

### Query otimizada

```typescript
// N+1
const users = await userRepository.find()
for (const user of users) {
  user.orders = await orderRepository.findByUserId(user.id)
}

// Eager loading / join
const users = await userRepository.find({
  relations: ['orders'],
})
```

### Graceful shutdown

```typescript
const gracefulShutdown = async (signal: string) => {
  console.log(`Received ${signal}, shutting down...`)
  await httpServer.close()
  await dataSource.destroy()
  await redisClient.quit()
  process.exit(0)
}

process.on('SIGTERM', () => gracefulShutdown('SIGTERM'))
process.on('SIGINT', () => gracefulShutdown('SIGINT'))
```

---

## 🔹 Go: checklist de qualidade

A maioria dos itens são universais. Os pontos abaixo destacam o que muda em Go.

### Segurança (Go)

- [ ] Input externo validado (`go-playground/validator`, `ozzo-validation` ou refinamento manual).
- [ ] Queries parametrizadas (`db.QueryContext` com `$1`, `$2`).
- [ ] Sem `crypto/md5`/`sha1` para autenticação.
- [ ] `crypto/rand` (não `math/rand`) para tokens/IDs sensíveis.
- [ ] `bcrypt`/`argon2` para senhas; `subtle.ConstantTimeCompare` em comparações sensíveis.
- [ ] Headers de segurança (Helmet equivalente: `unrolled/secure`).
- [ ] Rate limiting (`golang.org/x/time/rate` ou middleware `chi`/`echo`).
- [ ] `govulncheck ./...` sem CVEs alcançadas.

### Robustez (Go)

- [ ] Erros propagados com `fmt.Errorf("%w", err)`.
- [ ] `errcheck` linter ativo (sem erros silenciosamente ignorados).
- [ ] `context.Context` em todas as operações I/O (timeout/cancelamento).
- [ ] Timeout em chamadas externas via `context.WithTimeout`.
- [ ] `defer rows.Close()` em queries.
- [ ] Recovery middleware em servidor HTTP (panic não derruba o processo).

### Performance (Go)

- [ ] Sem N+1 (JOIN/Preload).
- [ ] Índices alinhados a queries reais (definidos em migration).
- [ ] `db.SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime` configurados.
- [ ] Cache (Redis/ristretto) com TTL e invalidação.
- [ ] Operações pesadas em fila (asynq/river) — não no request.
- [ ] Paginação (offset ou cursor) em listagens.
- [ ] pprof exposto em staging para troubleshooting.

### Banco de dados (Go)

- [ ] Migrations versionadas (golang-migrate, atlas, goose) com up + down testados.
- [ ] `database/sql` ou `sqlx`/`sqlc` usados consistentemente.
- [ ] Transações via `db.BeginTx(ctx, nil)` com `defer tx.Rollback()` (idempotente após commit).
- [ ] `NOT NULL` e defaults definidos no schema.

### API (Go)

- [ ] Request/response structs com tags `json` e (quando aplicável) `validate`.
- [ ] Status HTTP consistentes (`http.StatusCreated`, `http.StatusNoContent` etc.).
- [ ] Paginação com metadados.
- [ ] Versionamento via path (`/v1`) ou header.

### Filas (Go)

- [ ] Handler retorna `error` para retry automático.
- [ ] Backoff exponencial configurado.
- [ ] Dead-letter queue ou `MaxRetry` configurado.
- [ ] `context.Context` propagado para queries e clients no handler.

### Resiliência (Go)

- [ ] Graceful shutdown via `signal.NotifyContext` ou `signal.Notify`.
- [ ] `http.Server.Shutdown(ctx)` com timeout.
- [ ] Workers de fila param de aceitar jobs e drenam in-flight.
- [ ] Pools de banco/Redis fechados ordenadamente.
- [ ] Health check (`/healthz`, `/readyz`) com checagem real de dependências.

### Código (Go)

- [ ] `gofmt -l .` sem diff.
- [ ] `go vet ./...` sem warnings.
- [ ] `golangci-lint run` passa (errcheck, staticcheck, gocyclo, ...).
- [ ] `go test ./... -race` passa.
- [ ] `go mod tidy` sem mudanças não relacionadas.
- [ ] Acrônimos em maiúsculas (`UserID`, `URLParser`).
- [ ] Sem `interface{}` em hot path.

### Exemplos (Go)

#### Validação de input

```go
type CreateUserRequest struct {
    Email string `json:"email" validate:"required,email,max=255"`
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Age   int    `json:"age"   validate:"gte=0,lte=150"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    if err := validate.Struct(req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    // ...
}
```

#### Erro gracioso

```go
func (h *Handler) Fetch(w http.ResponseWriter, r *http.Request) {
    data, err := h.client.Fetch(r.Context())
    if err != nil {
        slog.ErrorContext(r.Context(), "fetch failed", "err", err)
        http.Error(w, "não foi possível carregar os dados", http.StatusBadGateway)
        return
    }
    writeJSON(w, http.StatusOK, data)
}
```

#### Graceful shutdown

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    srv := &http.Server{Addr: ":8080", Handler: router}

    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatal(err)
        }
    }()

    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Printf("shutdown error: %v", err)
    }
    db.Close()
    rdb.Close()
}
```
