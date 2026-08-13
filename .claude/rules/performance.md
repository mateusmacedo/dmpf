---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Performance

As recomendações abaixo são gerais; adapte aos frameworks e ferramentas adotados.

## Banco de dados

- Selecione apenas as colunas necessárias.
- Use query builder (ou equivalente) para consultas complexas; evite carregar relações desnecessárias.
- Previna N+1 usando joins ou estratégias de carregamento apropriadas.
- Crie índices para colunas que participam de filtros e ordenações frequentes.
- Evite retornar coleções ilimitadas em tabelas grandes; paginar é um default seguro.

## Cache

- Estratégia comum: cache-aside (verifica cache, busca no banco em caso de miss e popula o cache).
- Defina TTL adequado; cache eterno sem motivo técnico tende a gerar dados stale.
- Invalide o cache em operações de escrita (write-through ou invalidação explícita).
- Chaves com namespace descritivo facilitam debug (exemplo: `user:{id}:profile`).

## Filas

- Ajuste a concorrência ao tipo de job (CPU-bound tende a pedir concorrência baixa; I/O-bound suporta mais).
- Backoff exponencial em retries de jobs sujeitos a erros transientes.
- Rate limiting em jobs que chamam APIs externas.
- Priorize jobs críticos de forma explícita.

## Connection pooling

- Configure o pool de conexões de banco conforme o ambiente (tamanho mínimo/máximo).
- Reutilize conexões de cache; evite abrir uma conexão nova por request.

## Operações assíncronas

- Use `Promise.all` (ou alternativas com limite de concorrência) para operações independentes.
- Evite `await` sequencial quando as promessas poderiam rodar em paralelo.
- Evite bloquear o event loop com operações síncronas pesadas.

## Memória

- Monitore heap em processos de longa duração.
- Feche streams e remova listeners para evitar vazamentos.
- Atenção a acumuladores em closures de consumers de fila.

## Resposta da API

- Paginar datasets grandes (offset/limit ou cursor-based).
- Compressão para respostas grandes.
- Monitorar tempo de resposta e alertar acima de limiares definidos pelo SLO do projeto.

---

## 🔹 Go: performance e observabilidade

Go entrega vantagens de runtime (sem event loop bloqueável, GC tunável, goroutines baratas) mas tem armadilhas próprias.

### Banco de dados

- `database/sql` é a interface padrão; `sqlx` adiciona scan ergonômico, `sqlc` gera código tipado a partir de SQL.
- ORMs (`gorm`, `ent`) existem e têm armadilhas semelhantes ao TypeORM (N+1, eager loading custoso).
- `database/sql` exige `Rows.Close()` explícito; vazamentos de connection vêm desse esquecimento — `defer rows.Close()` é obrigatório.
- `sql.DB` já é um pool; configure `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime` no startup.
- Use `context.Context` em todas as queries (`db.QueryContext`) para permitir cancelamento por timeout.

### Cache

- Cache local: `sync.Map`, `golang-lru` ou `ristretto` (mais sofisticado, com TinyLFU).
- Cache distribuído: clientes Redis (`go-redis`) e Memcached (`gomemcache`) são padrão.
- Cuidado com cache em memória em apps que escalam horizontalmente — coerência fica por sua conta.

### Filas e concorrência

- Goroutines são baratas mas não gratuitas; controle a concorrência com worker pools (canal + N goroutines consumindo) ou `errgroup.WithContext`.
- Bibliotecas de fila persistente: `asynq`, `river`, `machinery`.
- Backoff: `cenkalti/backoff` ou implementação manual com `time.Sleep` + jitter.
- Rate limiting: `golang.org/x/time/rate` (token bucket oficial).

### Connection pooling

- Pool de DB já citado.
- Para HTTP outbound, **reuse** `http.Client` — criar um por request vaza connections. Configure `Transport` com `MaxIdleConnsPerHost` e timeouts (`Timeout` no client é mandatório).

### Operações assíncronas

- Não há `Promise.all`; equivalentes:
  - `errgroup.Group` (`golang.org/x/sync/errgroup`) — paraleliza com short-circuit em erro.
  - `sync.WaitGroup` — paraleliza sem propagar erro.
  - `chan` + select para coleta de resultados.
- Evite goroutines "fire-and-forget" sem `context` — vazam em shutdown.

### Memória e GC

- Profile com `pprof` (built-in via `net/http/pprof` ou `runtime/pprof`).
- Métricas relevantes: heap allocations (`go test -bench -benchmem`), goroutines vivas, GC pause.
- Vazamentos comuns: goroutine bloqueada em canal sem reader, ticker sem `Stop()`, `time.After` em loops longos (use `time.NewTimer` + `Reset`).
- Pré-aloque slices/maps quando o tamanho é conhecido (`make([]T, 0, n)`).

### Resposta da API

- Compressão via middleware (`gziphandler`, middleware nativo do framework).
- Streaming de resposta com `http.Flusher` para grandes payloads.
- Métricas: Prometheus (`prometheus/client_golang`) é o default de fato em Go.
- Tracing: OpenTelemetry tem SDK Go maduro (`go.opentelemetry.io/otel`).
