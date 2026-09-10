---
name: skill-performance
description: |
  Use esta skill quando o usuário pedir para "otimizar performance", "query lenta",
  "cache Redis", "fila BullMQ", "connection pool", "memory leak", "event loop",
  ou mencionar performance backend, otimização de queries, cache, filas ou monitoramento.
  Cobre queries ORM, cache Redis, filas, connection pooling, operações assíncronas,
  memória/event loop e monitoramento.
  Para padrões de arquitetura, ver `skill-architecture-patterns`.
model: opus
---

# Performance (backend)

## Objetivo

Otimização de performance no backend: queries, cache, filas, pooling, operações assíncronas, memória e monitoramento.

## Quando usar

- Ao otimizar queries lentas ou resolver N+1.
- Ao implementar ou revisar estratégia de cache.
- Ao ajustar filas, pooling ou processos de longa duração.

---

## 1. Otimização de queries (TypeORM como exemplo)

### Selecionar apenas colunas necessárias

```typescript
// Carrega todas as colunas (inclui blobs, textos longos)
const users = await userRepo.find()

// Seleciona só o que precisa
const users = await userRepo
  .createQueryBuilder('user')
  .select(['user.id', 'user.name', 'user.email'])
  .getMany()
```

### Evitar N+1

```typescript
// N+1: 1 query para orders + N queries para items
const orders = await orderRepo.find()
for (const order of orders) {
  order.items = await itemRepo.find({ where: { orderId: order.id } })
}

// Eager join — uma query só
const orders = await orderRepo.find({ relations: ['items'] })

// Query Builder para joins complexos
const orders = await orderRepo
  .createQueryBuilder('order')
  .leftJoinAndSelect('order.items', 'item')
  .where('order.status = :status', { status: 'active' })
  .getMany()
```

### Índices

```typescript
@Entity()
export class User {
  @Index()
  @Column()
  email: string

  @Index()
  @Column()
  tenantId: string
}

@Index(['tenantId', 'status'])
@Entity()
export class Document { /* ... */ }
```

Regra prática: crie índice para colunas usadas frequentemente em `WHERE`, `JOIN` ou `ORDER BY`.

### Paginação

```typescript
// Offset-based — simples, ok para datasets pequenos
const users = await userRepo.find({ skip: 20, take: 10 })

// Cursor-based — mais eficiente em grandes volumes
const users = await userRepo
  .createQueryBuilder('user')
  .where('user.id > :cursor', { cursor: lastId })
  .orderBy('user.id', 'ASC')
  .take(10)
  .getMany()
```

Para tabelas grandes ou paginação via API, cursor-based costuma ser preferível.

---

## 2. Cache (Redis como exemplo)

### Cache-aside

```typescript
async getUser(id: string): Promise<User> {
  const cached = await redis.get(`user:${id}`)
  if (cached) return JSON.parse(cached)

  const user = await userRepo.findOneBy({ id })
  await redis.set(`user:${id}`, JSON.stringify(user), 'EX', 3600)
  return user
}
```

### TTL por tipo de dado (sugestões)

| Tipo | TTL sugerido | Motivo |
| ------ | ------------- | -------- |
| Sessão / token | 15-30 min | Segurança |
| Dados do usuário | 5-15 min | Mudança moderada |
| Dados de referência | 1-24h | Raramente muda |
| Config / feature flags | 1-5 min | Precisa atualizar rápido |

### Invalidação

```typescript
async updateUser(id: string, data: UpdateUserDto) {
  await userRepo.update(id, data)
  await redis.del(`user:${id}`)
}
```

### Armadilhas comuns

| Problema | Causa | Mitigação |
| ---------- | ------- | ----------- |
| Cache stampede | Muitos requests simultâneos em miss | Mutex/locking no rebuild |
| Dados stale | Cache sem TTL | Sempre definir TTL |
| Memória | Chaves sem expiração acumulam | Monitorar uso de memória |

---

## 3. Filas (BullMQ como exemplo)

### Concorrência

```typescript
const worker = new Worker('email', processEmail, {
  concurrency: 5, // ajustar ao CPU disponível
})
```

Começar com um valor moderado (3-5), medir e ajustar. CPU-bound pede menos; I/O-bound suporta mais.

### Backoff exponencial

```typescript
await queue.add('send-email', payload, {
  attempts: 3,
  backoff: { type: 'exponential', delay: 1000 },
})
```

### Rate limiting

```typescript
const worker = new Worker('external-api', callApi, {
  limiter: { max: 10, duration: 1000 }, // 10 jobs/segundo
})
```

### Prioridade

```typescript
await queue.add('urgent', data, { priority: 1 })
await queue.add('normal', data, { priority: 5 })
await queue.add('background', data, { priority: 10 })
```

Variações comuns por projeto: consumers podem herdar de uma classe base (ajustando concorrência localmente) ou aparecer como Steps de um pipeline com retries/backoff configurados em YAML. Ajuste ao padrão do projeto.

---

## 4. Connection pooling

### PostgreSQL (TypeORM)

```typescript
{
  type: 'postgres',
  extra: {
    max: 20,
    idleTimeoutMillis: 10000,
    connectionTimeoutMillis: 3000,
  },
}
```

Referência: `max connections ≈ (CPU cores * 2) + discos`. 10-20 costuma atender a maioria.

### Redis

```typescript
// Reutilizar conexão — evitar abrir/fechar por request
const redis = new Redis({ host: '...', maxRetriesPerRequest: 3 })
export { redis }
```

### MongoDB

```typescript
{
  uri: 'mongodb://...',
  options: {
    maxPoolSize: 10,
    minPoolSize: 2,
    maxIdleTimeMS: 30000,
  },
}
```

---

## 5. Operações assíncronas

### `Promise.all` para operações independentes

```typescript
// Sequencial desnecessário — soma dos tempos
const user = await userRepo.findOneBy({ id })
const orders = await orderRepo.find({ where: { userId: id } })
const notifications = await notificationRepo.count({ userId: id })

// Paralelo — tempo do mais lento
const [user, orders, notifCount] = await Promise.all([
  userRepo.findOneBy({ id }),
  orderRepo.find({ where: { userId: id } }),
  notificationRepo.count({ userId: id }),
])
```

### Offload para fila

```typescript
// Processamento pesado no request bloqueia a resposta
app.post('/documents', async (req, res) => {
  const doc = await createDocument(req.body)
  await generatePdf(doc)       // 10-30s
  await sendEmail(doc.userId)  // 2-5s
  res.json(doc)
})

// Responder rápido e processar em background
app.post('/documents', async (req, res) => {
  const doc = await createDocument(req.body)
  await pdfQueue.add('generate', { docId: doc.id })
  await emailQueue.add('notify', { userId: doc.userId })
  res.status(202).json(doc)
})
```

### Streams para respostas grandes

```typescript
// Carrega tudo na memória
const rows = await repo.find()
res.json(rows)

// Stream
const stream = await repo.createQueryBuilder('row').stream()
stream.pipe(new Transform({ /* formatar */ })).pipe(res)
```

---

## 6. Memória e event loop

### Memory leaks comuns

| Causa | Exemplo | Mitigação |
| ------- | --------- | ----------- |
| Listeners não removidos | `emitter.on()` sem `off()` | Remover no cleanup / usar `once()` |
| Streams não fechados | ReadStream sem `.destroy()` | Fechar em `finally` |
| Closures retendo referências | Callbacks capturando objetos grandes | Capturar só o necessário |
| Cache in-memory sem limite | `Map` crescendo sem bound | LRU com tamanho máximo |

### Não bloquear o event loop

```typescript
// Bloqueia o event loop (CPU-bound síncrono)
const hash = crypto.pbkdf2Sync(password, salt, 100000, 64, 'sha512')

// Versão assíncrona
const hash = await new Promise((resolve, reject) => {
  crypto.pbkdf2(password, salt, 100000, 64, 'sha512', (err, key) =>
    err ? reject(err) : resolve(key)
  )
})
```

---

## 7. Monitoramento

### Métricas essenciais

| Métrica | O que observar | Alertar quando |
| --------- | --------------- | ---------------- |
| Response time (p95) | Latência das rotas | Acima do SLA definido |
| Slow queries | Queries acima de threshold | > ~200ms |
| Queue backlog | Jobs pendentes | Crescendo por muito tempo |
| Error rate | Percentual de 5xx | Acima do limite acordado |
| Heap usage | Memória do processo | Próximo do limite |

### Log de queries lentas (TypeORM)

```typescript
{
  logging: ['query'],
  maxQueryExecutionTime: 200, // log para queries > 200ms
}
```

### Timing de operações

```typescript
console.time('create-document')
await createDocument(data)
console.timeEnd('create-document')
```

---

## Checklist

- [ ] Queries selecionam apenas colunas necessárias.
- [ ] Sem N+1 (joins ou relations carregados).
- [ ] Índices para colunas de WHERE/JOIN frequentes.
- [ ] Cache com TTL e invalidação correta.
- [ ] Operações pesadas offloaded para filas.
- [ ] Connection pools configurados adequadamente.
- [ ] Sem bloqueio do event loop com operações síncronas pesadas.
- [ ] Monitoramento de response time e slow queries ativo.

---

## 🔹 Go: performance backend em Go

Os princípios (evitar N+1, cachear com TTL, offload para fila, pool de conexões) são universais. Em Go há ferramentas e idiomas próprios — pprof e race detector são built-in, goroutines substituem o event loop como modelo de concorrência.

### 1. Otimização de queries

#### Selecionar apenas colunas necessárias

```go
// database/sql — explícito por padrão
rows, err := db.QueryContext(ctx, `SELECT id, name, email FROM users`)

// sqlc — query gerada com colunas específicas
users, err := q.ListUserSummaries(ctx)

// GORM
var users []User
db.Select("id", "name", "email").Find(&users)
```

#### Evitar N+1

```go
// N+1: 1 query + N queries
rows, _ := db.QueryContext(ctx, `SELECT id FROM orders WHERE user_id = $1`, userID)
for rows.Next() {
    var orderID string
    rows.Scan(&orderID)
    db.QueryContext(ctx, `SELECT * FROM items WHERE order_id = $1`, orderID)  // N+1
}

// JOIN — uma query
const q = `
SELECT o.id, o.created_at, i.id, i.name
FROM orders o
LEFT JOIN items i ON i.order_id = o.id
WHERE o.user_id = $1`

// GORM com Preload
db.Preload("Items").Where("user_id = ?", userID).Find(&orders)
```

#### Índices

Em Go o índice é definido na migration, não na entidade:

```sql
-- migrations/20240101_users.up.sql
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_documents_tenant_status ON documents(tenant_id, status);
```

GORM permite `gorm:"index"` na tag, mas migrations explícitas (golang-migrate, atlas, goose) são preferidas em produção.

#### Paginação

```go
// Offset-based
const q = `SELECT id, name FROM users ORDER BY id LIMIT $1 OFFSET $2`
rows, err := db.QueryContext(ctx, q, limit, offset)

// Cursor-based — mais eficiente
const q = `SELECT id, name FROM users WHERE id > $1 ORDER BY id LIMIT $2`
rows, err := db.QueryContext(ctx, q, lastID, limit)
```

#### Sempre fechar `rows`

```go
rows, err := db.QueryContext(ctx, q)
if err != nil {
    return err
}
defer rows.Close()  // crítico — leak de conexão sem isso
```

`go vet` (sqlclosecheck) e `golangci-lint` detectam ausência.

---

### 2. Cache (Redis com `go-redis`)

#### Cache-aside

```go
import "github.com/redis/go-redis/v9"

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    key := "user:" + id
    cached, err := s.redis.Get(ctx, key).Bytes()
    if err == nil {
        var u User
        if err := json.Unmarshal(cached, &u); err == nil {
            return &u, nil
        }
    }

    u, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, err
    }

    payload, _ := json.Marshal(u)
    s.redis.Set(ctx, key, payload, time.Hour)
    return u, nil
}
```

#### Invalidação

```go
func (s *UserService) Update(ctx context.Context, id string, in UpdateInput) error {
    if err := s.repo.Update(ctx, id, in); err != nil {
        return err
    }
    return s.redis.Del(ctx, "user:"+id).Err()
}
```

#### Cache local (in-memory)

Para dados muito acessados, `ristretto` (Dgraph) ou `bigcache` (Allegro) reduzem hops para Redis:

```go
import "github.com/dgraph-io/ristretto"

cache, _ := ristretto.NewCache(&ristretto.Config{
    NumCounters: 1e7,
    MaxCost:     1 << 28, // 256MB
    BufferItems: 64,
})
```

`sync.Map` é raramente a melhor escolha para cache — falta de TTL e LRU.

---

### 3. Filas (asynq, river)

#### asynq

```go
import "github.com/hibiken/asynq"

server := asynq.NewServer(redisOpt, asynq.Config{
    Concurrency: 5,
    Queues: map[string]int{
        "critical": 6,
        "default":  3,
        "low":      1,
    },
})
```

#### Backoff exponencial

```go
client.Enqueue(task,
    asynq.MaxRetry(3),
    asynq.Timeout(30*time.Second),
)

// Customizar via interface RetryDelayFunc
asynq.Config{
    RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
        return time.Duration(1<<n) * time.Second  // 1, 2, 4, 8...
    },
}
```

#### Rate limiting

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(10, 10)  // 10 jobs/seg, burst 10

func handle(ctx context.Context, t *asynq.Task) error {
    if err := limiter.Wait(ctx); err != nil {
        return err
    }
    // processar
}
```

---

### 4. Connection pooling

#### `database/sql`

```go
db, err := sql.Open("postgres", dsn)
db.SetMaxOpenConns(20)
db.SetMaxIdleConns(10)
db.SetConnMaxIdleTime(10 * time.Minute)
db.SetConnMaxLifetime(time.Hour)
```

`db.Stats()` expõe métricas (`OpenConnections`, `WaitCount`, `WaitDuration`) — ótimas para alertas.

#### Redis (`go-redis`)

```go
rdb := redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    PoolSize:     20,
    MinIdleConns: 5,
    PoolTimeout:  4 * time.Second,
})
```

---

### 5. Operações concorrentes

#### `errgroup` para paralelismo com erro propagado

```go
import "golang.org/x/sync/errgroup"

func loadDashboard(ctx context.Context, userID string) (*Dashboard, error) {
    g, ctx := errgroup.WithContext(ctx)

    var user *User
    var orders []*Order
    var notifs int

    g.Go(func() error {
        var err error
        user, err = userRepo.FindByID(ctx, userID)
        return err
    })
    g.Go(func() error {
        var err error
        orders, err = orderRepo.ListByUser(ctx, userID)
        return err
    })
    g.Go(func() error {
        var err error
        notifs, err = notifRepo.Count(ctx, userID)
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return &Dashboard{User: user, Orders: orders, NotifCount: notifs}, nil
}
```

`errgroup` cancela as outras goroutines automaticamente quando uma falha.

#### Limitar concorrência

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10)  // Go 1.20+

for _, item := range items {
    item := item
    g.Go(func() error {
        return process(ctx, item)
    })
}
return g.Wait()
```

---

### 6. Memória, GC e profiling

#### pprof (built-in)

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Acessar via `go tool pprof http://localhost:6060/debug/pprof/heap` ou `/profile` (CPU).

#### Memory leaks comuns

| Causa | Mitigação |
| ------- | ----------- |
| Goroutine sem `context` para cancelar | Sempre passar `context.Context` |
| Map crescendo sem limite | LRU (`hashicorp/golang-lru`) |
| `time.Tick` sem `Stop()` em short-lived | Usar `time.NewTicker` + `defer t.Stop()` |
| Slice retendo array enorme após slicing | `dst := append([]T(nil), small...)` |
| Closure capturando objeto grande | Capturar só os campos necessários |

#### `sync.Pool` para objetos reusáveis

```go
var bufPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}

func handle() {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufPool.Put(buf)
    }()
    // usar buf
}
```

Útil em hot paths (ex.: serialização). Não use para dados sensíveis (pool não zera entre usos).

#### `strings.Builder` em vez de `+=` em loops

```go
// Aloca a cada iteração
s := ""
for _, p := range parts {
    s += p
}

// Aloca uma vez (com capacidade pré-definida)
var b strings.Builder
b.Grow(estimatedSize)
for _, p := range parts {
    b.WriteString(p)
}
result := b.String()
```

---

### 7. Monitoramento

#### Prometheus (instrumentação)

```go
import "github.com/prometheus/client_golang/prometheus"

var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Buckets: prometheus.DefBuckets,
    },
    []string{"method", "route", "status"},
)
```

#### Métricas essenciais (Go)

| Métrica | Fonte |
| --------- | ------- |
| Response time (p95, p99) | histograma Prometheus por rota |
| Slow queries | log de `database/sql` ou interceptor |
| Goroutine count | `runtime.NumGoroutine()` exportado |
| Heap usage | `runtime.ReadMemStats()` ou `expvar` |
| GC pause | `MemStats.PauseNs` |
| Queue backlog | métricas do worker (asynq, river) |

#### Tracing (OpenTelemetry)

```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer("myapp")
ctx, span := tracer.Start(ctx, "createDocument")
defer span.End()
```

`slog` (stdlib) integra com OpenTelemetry para correlacionar logs, traces e métricas.

---

### Anti-patterns (Go performance)

- Goroutine sem `context` (impossibilita cancelamento e timeout).
- `defer` em loop apertado (custo acumula; chamar manualmente quando aplicável).
- Conversão `[]byte(s)` ou `string(b)` em hot path (cópia).
- `interface{}` em hot path (alocação no heap por boxing).
- `time.Sleep` em testes/coordination — usar channels ou `errgroup`.
- Mutex global protegendo dados raramente escritos — preferir `sync.RWMutex` ou `atomic`.

### Checklist (Go)

- [ ] Queries com colunas específicas (não `SELECT *`).
- [ ] Sem N+1 (JOIN em SQL ou Preload em GORM).
- [ ] `defer rows.Close()` em todas as queries.
- [ ] Índices alinhados a queries reais.
- [ ] `db.SetMaxOpenConns` e `SetConnMaxLifetime` configurados.
- [ ] Cache com TTL e invalidação no update.
- [ ] Operações pesadas em fila (asynq, river) — não no request.
- [ ] `errgroup` com `SetLimit` para paralelismo controlado.
- [ ] `context.Context` propagado em toda chamada I/O.
- [ ] pprof exposto em ambiente de dev/staging.
- [ ] `go test -race` passa.
- [ ] Métricas Prometheus + tracing OTel ativos em produção.
