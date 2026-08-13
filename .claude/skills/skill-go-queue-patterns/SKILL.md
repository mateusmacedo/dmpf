---
name: skill-go-queue-patterns
description: |
  Use esta skill ao trabalhar com filas em Go: asynq, river, machinery, watermill.
  Cobre setup de queues, workers, tipagem de jobs, retry, dead-letter, monitoramento e
  graceful shutdown. Os padrões aplicam-se a sistemas baseados em Redis (asynq), Postgres
  (river) ou message brokers genéricos (watermill).
model: sonnet
---

# Go queues — padrões

## Objetivo

Compilar padrões comuns ao usar filas em Go: setup, workers, tipagem de jobs, estratégias de retry e tratamento de erros.

## Quando usar

- Ao criar ou modificar queues e workers.
- Ao definir jobs e suas tipagens.
- Ao configurar retry e dead-letter queues.
- Ao implementar monitoramento de progresso.
- Ao depurar jobs falhando ou travados.

## Escolha de biblioteca

| Biblioteca | Backend | Característica |
|------------|---------|----------------|
| `hibiken/asynq` | Redis | Idiomático Go, dashboard web, similar a BullMQ |
| `riverqueue/river` | Postgres | Sem Redis, transacional com seu DB |
| `RichardKnop/machinery` | Redis/AMQP | Multi-broker, mais antigo |
| `ThreeDotsLabs/watermill` | Múltiplos | Abstração de pub/sub e CDC |
| Cron + tabela própria | Postgres | Simples, controle total |

`asynq` cobre a maioria dos casos com Redis. `river` ganha quando você quer transacionalidade com o DB de aplicação.

---

## Setup com asynq

### Conexão e cliente

```go
import "github.com/hibiken/asynq"

redisOpt := asynq.RedisClientOpt{
    Addr:     cfg.RedisAddr,
    Password: cfg.RedisPassword,
}

// Cliente para enfileirar
client := asynq.NewClient(redisOpt)
defer client.Close()

// Servidor para processar
srv := asynq.NewServer(redisOpt, asynq.Config{
    Concurrency: 10,
    Queues: map[string]int{
        "critical": 6,
        "default":  3,
        "low":      1,
    },
    StrictPriority: false,
})
```

### Convenção de nomes

| Padrão | Exemplo | Uso |
|--------|---------|-----|
| `recurso:acao` | `documents:generate` | Task type principal |
| `recurso:acao:subtipo` | `documents:generate:pdf` | Especialização |
| `scheduled:recurso` | `scheduled:cleanup` | Tasks recorrentes |

`task type` é um identificador string; queue é separada (config do servidor).

---

## Definição de job

### Payload tipado

```go
package documentjob

const TaskGenerate = "documents:generate"

type GeneratePayload struct {
    DocumentID     string                 `json:"documentId"`
    UserID         string                 `json:"userId"`
    OrganizationID string                 `json:"organizationId"`
    TemplateID     string                 `json:"templateId"`
    Variables      map[string]any         `json:"variables"`
}

type GenerateResult struct {
    DocumentURL string `json:"documentUrl"`
    PageCount   int    `json:"pageCount"`
}
```

### Constructor de task

```go
func NewGenerateTask(p GeneratePayload, opts ...asynq.Option) (*asynq.Task, error) {
    payload, err := json.Marshal(p)
    if err != nil {
        return nil, fmt.Errorf("marshal payload: %w", err)
    }
    defaultOpts := []asynq.Option{
        asynq.MaxRetry(3),
        asynq.Timeout(5 * time.Minute),
        asynq.Queue("default"),
    }
    return asynq.NewTask(TaskGenerate, payload, append(defaultOpts, opts...)...), nil
}
```

### Enfileirar

```go
task, err := documentjob.NewGenerateTask(documentjob.GeneratePayload{
    DocumentID: "doc-123",
    UserID:     "user-456",
    TemplateID: "tpl-001",
    Variables:  map[string]any{"clientName": "João"},
})
if err != nil {
    return err
}

info, err := client.EnqueueContext(ctx, task,
    asynq.Queue("critical"),
    asynq.MaxRetry(5),
    asynq.TaskID(fmt.Sprintf("doc-%s", documentID)), // idempotência
)
if err != nil {
    return fmt.Errorf("enqueue: %w", err)
}
slog.Info("task enqueued", "id", info.ID, "queue", info.Queue)
```

`TaskID` deduplica: se já houver task com mesmo ID na fila, falha com `ErrDuplicateTask`.

---

## Worker / handler

### Handler básico

```go
func HandleGenerate(ctx context.Context, t *asynq.Task) error {
    var p GeneratePayload
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        // payload corrompido — não retentar
        return fmt.Errorf("unmarshal: %w: %w", err, asynq.SkipRetry)
    }

    doc, err := generateDocument(ctx, p)
    if err != nil {
        return fmt.Errorf("generate doc: %w", err)
    }

    url, err := uploadDocument(ctx, doc)
    if err != nil {
        return fmt.Errorf("upload: %w", err)
    }

    slog.Info("document generated",
        "documentID", p.DocumentID,
        "url", url,
        "pages", doc.PageCount,
    )
    return nil
}
```

### Registrar handlers

```go
mux := asynq.NewServeMux()
mux.HandleFunc(documentjob.TaskGenerate, documentjob.HandleGenerate)
mux.HandleFunc(emailjob.TaskSend, emailjob.HandleSend)
mux.Use(loggingMiddleware, metricsMiddleware)

if err := srv.Run(mux); err != nil {
    log.Fatal(err)
}
```

`srv.Run` bloqueia. Use `srv.Start` + canal de sinais para controle fino.

### Worker como struct (DI)

```go
type GenerateWorker struct {
    docRepo  document.Repository
    storage  storage.Service
    log      *slog.Logger
}

func NewGenerateWorker(docRepo document.Repository, storage storage.Service, log *slog.Logger) *GenerateWorker {
    return &GenerateWorker{docRepo: docRepo, storage: storage, log: log}
}

func (w *GenerateWorker) Handle(ctx context.Context, t *asynq.Task) error {
    var p GeneratePayload
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return fmt.Errorf("unmarshal: %w: %w", err, asynq.SkipRetry)
    }
    // ... lógica usando w.docRepo, w.storage
    return nil
}

// composition root
mux.HandleFunc(documentjob.TaskGenerate, generateWorker.Handle)
```

Permite injeção de dependências sem singletons globais.

---

## Retry e error handling

### Retry automático

`asynq` reatenta automaticamente quando o handler retorna erro. Configure via `MaxRetry`:

```go
asynq.NewTask(taskType, payload,
    asynq.MaxRetry(5),
    asynq.Timeout(10 * time.Minute),
    asynq.Deadline(time.Now().Add(1 * time.Hour)),
)
```

Backoff exponencial padrão: `2^retry_count` segundos. Customizável via `RetryDelayFunc`:

```go
srv := asynq.NewServer(redisOpt, asynq.Config{
    RetryDelayFunc: func(n int, err error, t *asynq.Task) time.Duration {
        var rateErr *RateLimitError
        if errors.As(err, &rateErr) {
            return rateErr.RetryAfter
        }
        return time.Duration(math.Pow(2, float64(n))) * time.Second
    },
})
```

### Não retentar (SkipRetry)

```go
if errors.Is(err, ErrInvalidTemplate) {
    return fmt.Errorf("invalid template %s: %w: %w", p.TemplateID, err, asynq.SkipRetry)
}
```

`asynq.SkipRetry` move o job para a fila `archived` sem reatentar.

### Erros customizados

```go
var (
    ErrInvalidTemplate = errors.New("invalid template")
    ErrTransient       = errors.New("transient error")
)

type RateLimitError struct {
    RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
    return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}
```

### Handler com lógica condicional

```go
func (w *GenerateWorker) Handle(ctx context.Context, t *asynq.Task) error {
    var p GeneratePayload
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return fmt.Errorf("unmarshal: %w: %w", err, asynq.SkipRetry)
    }

    doc, err := w.generate(ctx, p)
    if err != nil {
        if errors.Is(err, ErrInvalidTemplate) {
            return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
        }
        return fmt.Errorf("generate: %w", err) // retentado
    }
    return w.save(ctx, doc)
}
```

### Compensação em falha definitiva

```go
mux.Use(func(next asynq.Handler) asynq.Handler {
    return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
        err := next.ProcessTask(ctx, t)
        if err == nil {
            return nil
        }
        // Última tentativa?
        retried, _ := asynq.GetRetryCount(ctx)
        maxRetry, _ := asynq.GetMaxRetry(ctx)
        if retried >= maxRetry {
            if compErr := compensate(ctx, t); compErr != nil {
                slog.Error("compensation failed", "err", compErr, "taskID", t.Type())
            }
        }
        return err
    })
})
```

Para créditos/billing, compense no último retry. Idempotência protege contra falhas no compensador.

---

## Idempotência

### Por payload

```go
type SendEmailPayload struct {
    IdempotencyKey string `json:"idempotencyKey"` // ex: hash(userID, action, day)
    To             string `json:"to"`
    Subject        string `json:"subject"`
    Body           string `json:"body"`
}

func HandleSend(ctx context.Context, t *asynq.Task) error {
    var p SendEmailPayload
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return fmt.Errorf("unmarshal: %w: %w", err, asynq.SkipRetry)
    }

    sent, err := store.AlreadySent(ctx, p.IdempotencyKey)
    if err != nil {
        return fmt.Errorf("check idempotency: %w", err)
    }
    if sent {
        return nil // sucesso silencioso
    }

    if err := mailer.Send(ctx, p.To, p.Subject, p.Body); err != nil {
        return fmt.Errorf("send: %w", err)
    }

    return store.MarkSent(ctx, p.IdempotencyKey)
}
```

### Por TaskID

```go
client.EnqueueContext(ctx, task, asynq.TaskID("send-welcome-"+userID))
```

Se a task já existe na fila, retorna `ErrDuplicateTask`. Útil para evitar enfileirar duas vezes.

---

## Monitoramento

### Logging middleware

```go
func loggingMiddleware(next asynq.Handler) asynq.Handler {
    return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
        start := time.Now()
        retried, _ := asynq.GetRetryCount(ctx)
        taskID, _ := asynq.GetTaskID(ctx)

        err := next.ProcessTask(ctx, t)

        attrs := []any{
            "type", t.Type(),
            "taskID", taskID,
            "retry", retried,
            "duration", time.Since(start),
        }
        if err != nil {
            attrs = append(attrs, "err", err)
            slog.Error("task failed", attrs...)
        } else {
            slog.Info("task done", attrs...)
        }
        return err
    })
}
```

### Métricas

```go
import "github.com/hibiken/asynq"

inspector := asynq.NewInspector(redisOpt)

queues := []string{"critical", "default", "low"}
for _, q := range queues {
    info, err := inspector.GetQueueInfo(q)
    if err != nil {
        continue
    }
    slog.Info("queue stats",
        "queue", q,
        "pending", info.Pending,
        "active", info.Active,
        "retry", info.Retry,
        "archived", info.Archived,
    )
}
```

Exporte para Prometheus via middleware customizado ou exporter dedicado (`asynq` tem integração via `asynqmon`).

### Dashboard (asynqmon)

```bash
go install github.com/hibiken/asynq/tools/asynq@latest
asynq dash
```

Web UI para inspecionar filas, retentar manualmente, archivar tasks.

---

## Tasks recorrentes

### Periodic tasks

```go
import "github.com/hibiken/asynq"

scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{
    Location: time.UTC,
})

if _, err := scheduler.Register("0 2 * * *", asynq.NewTask("scheduled:cleanup", nil)); err != nil {
    log.Fatal(err)
}

if err := scheduler.Run(); err != nil {
    log.Fatal(err)
}
```

Sintaxe cron padrão. `scheduler.Run` bloqueia.

### Delayed tasks

```go
client.EnqueueContext(ctx, task,
    asynq.ProcessIn(30 * time.Minute),     // delay relativo
    asynq.ProcessAt(specificTime),         // delay absoluto
)
```

---

## Graceful shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

if err := srv.Start(mux); err != nil {
    log.Fatal(err)
}

<-ctx.Done()
slog.Info("shutdown signal received")

srv.Shutdown()  // bloqueia até jobs em voo terminarem (até ShutdownTimeout)
slog.Info("workers stopped")
```

Configure `ShutdownTimeout` na config do servidor (default 8s).

---

## Anti-patterns

| Anti-pattern | Preferir |
|--------------|----------|
| Operação síncrona bloqueante sem `context` | Sempre propagar `ctx` para I/O |
| Job sem tipagem de payload | Struct + JSON |
| Retry infinito | `MaxRetry` + dead letter (`archived`) |
| `panic` no handler | Retornar erro; deixar lib recuperar |
| Esquecer `client.Close()`/`srv.Shutdown()` | `defer` ou `signal.NotifyContext` |
| Compartilhar `client` entre processos sem reuso | Reaproveitar conexão Redis |
| Job com payload gigante (MBs) | Guardar payload em S3/DB; mandar referência |
| Sem idempotência em retries | `IdempotencyKey` ou checagem por estado |
| Dependência de tempo via `time.Now()` no handler | Injetar clock para testar |

---

## river (alternativa Postgres)

Quando o backlog precisa ser transacional com o DB de aplicação:

```go
import "github.com/riverqueue/river"

type SendEmailArgs struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
}

func (SendEmailArgs) Kind() string { return "send_email" }

type SendEmailWorker struct {
    river.WorkerDefaults[SendEmailArgs]
    mailer Mailer
}

func (w *SendEmailWorker) Work(ctx context.Context, job *river.Job[SendEmailArgs]) error {
    return w.mailer.Send(ctx, job.Args.To, job.Args.Subject)
}

workers := river.NewWorkers()
river.AddWorker(workers, &SendEmailWorker{mailer: mailer})

riverClient, _ := river.NewClient(riverpgxv5.New(pool), &river.Config{
    Queues:  map[string]river.QueueConfig{"default": {MaxWorkers: 10}},
    Workers: workers,
})
riverClient.Start(ctx)
```

Vantagens: transação com DB principal, sem Redis. Trade-off: throughput menor, depende de Postgres.

---

## Variações comuns

- **Worker pool customizado**: goroutines + channels + Redis Streams (sem lib).
- **Watermill**: abstrai brokers (Kafka, NATS, AMQP) — adequado para event-driven.
- **machinery**: multi-broker com chaining e canvas (mais complexo).
- **Pipeline engine**: orquestrar passos no mesmo job, reportando progresso por step.

Adapte ao padrão do projeto.

---

## Checklist

- [ ] Task type com convenção `recurso:acao`.
- [ ] Payload tipado em struct + tags `json`.
- [ ] `MaxRetry` e `Timeout` configurados por task.
- [ ] `asynq.SkipRetry` para erros não recuperáveis.
- [ ] `IdempotencyKey` ou `TaskID` para evitar processamento duplicado.
- [ ] Compensação em falha definitiva quando aplicável.
- [ ] Logging middleware com `taskID`, `retry`, `duration`.
- [ ] `signal.NotifyContext` + `srv.Shutdown()` para graceful shutdown.
- [ ] Workers como structs com DI (sem globals).
- [ ] Dashboard (`asynqmon`) ou métricas Prometheus em produção.
- [ ] Sem payload pesado — referenciar via S3/DB.
- [ ] Conexão Redis configurada via env.
