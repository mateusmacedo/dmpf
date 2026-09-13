---
name: skill-bullmq-patterns
description: |
  Use esta skill ao trabalhar com BullMQ: queues, workers, consumers, jobs, retry e error handling.
  Os padrões descritos são aplicáveis a qualquer backend Node.js que use BullMQ; muitos também
  se aplicam, com ajustes, a outras bibliotecas de filas.
model: opus
---

# BullMQ Patterns

## Objetivo

Compilar padrões comuns ao usar BullMQ: setup de queues, workers/consumers, tipagem de jobs, estratégias de retry e tratamento de erros.

## Quando usar

- Ao criar ou modificar queues e workers.
- Ao definir jobs e suas tipagens.
- Ao configurar retry e dead-letter queues.
- Ao implementar monitoramento de progresso.
- Ao depurar jobs falhando ou travados.

## Queue setup

### Conexão com Redis

```typescript
import { Queue } from 'bullmq'

const connection = {
  host: process.env.REDIS_HOST ?? 'localhost',
  port: Number(process.env.REDIS_PORT ?? 6379),
}

const documentQueue = new Queue('documents.generate', {
  connection,
  defaultJobOptions: {
    attempts: 3,
    backoff: { type: 'exponential', delay: 2000 },
    removeOnComplete: { count: 1000 },
    removeOnFail: { count: 5000 },
  },
})
```

### Convenção de nomes (sugestão)

| Padrão | Exemplo | Uso |
| -------- | --------- | ----- |
| `recurso.acao` | `documents.generate` | Fila principal |
| `recurso.acao.subtipo` | `documents.generate.pdf` | Fila especializada |
| `scheduled.recurso` | `scheduled.cleanup` | Jobs agendados |

## Definição de job

### Tipagem dos dados

```typescript
type GenerateDocumentJobData = {
  documentId: string
  userId: string
  organizationId: string
  templateId: string
  variables: Record<string, unknown>
}

type GenerateDocumentJobResult = {
  documentUrl: string
  pageCount: number
}
```

### Adicionar à fila

```typescript
await documentQueue.add(
  'generate-pdf',
  {
    documentId: '123',
    userId: 'user-456',
    organizationId: 'org-789',
    templateId: 'tpl-001',
    variables: { clientName: 'Joao' },
  } satisfies GenerateDocumentJobData,
  {
    priority: 1,
    jobId: `doc-${documentId}`,
  },
)
```

Recomendação: tipar `jobData` de forma explícita; `satisfies` é uma opção prática em TypeScript. Evite `any` sem justificativa.

## Worker / consumer

### Worker básico

```typescript
import { Worker, Job } from 'bullmq'

const worker = new Worker<GenerateDocumentJobData, GenerateDocumentJobResult>(
  'documents.generate',
  async (job: Job<GenerateDocumentJobData>) => {
    await job.updateProgress(10)

    const document = await generateDocument(job.data)
    await job.updateProgress(80)

    const url = await uploadDocument(document)
    await job.updateProgress(100)

    return { documentUrl: url, pageCount: document.pages }
  },
  {
    connection,
    concurrency: 5,
    limiter: { max: 10, duration: 60_000 },
  },
)
```

### Eventos do ciclo de vida

```typescript
worker.on('completed', (job, result) => {
  logger.info(`Job ${job.id} concluído`, { result })
})

worker.on('failed', (job, error) => {
  logger.error(`Job ${job?.id} falhou`, { error: error.message })
})

worker.on('stalled', (jobId) => {
  logger.warn(`Job ${jobId} travou (stalled)`)
})
```

Jobs "stalled" geralmente indicam problemas de concorrência ou timeout; vale a pena monitorar.

## Error handling

### Erros customizados

```typescript
export class JobProcessingError extends Error {
  constructor(
    message: string,
    public readonly jobId: string,
    public readonly shouldRetry: boolean = true,
  ) {
    super(message)
    this.name = 'JobProcessingError'
  }
}

export class CreditRefundError extends Error {
  constructor(
    public readonly userId: string,
    public readonly credits: number,
  ) {
    super(`Falha ao reembolsar ${credits} créditos para ${userId}`)
    this.name = 'CreditRefundError'
  }
}
```

### Retry com lógica condicional

```typescript
const worker = new Worker('documents.generate', async (job) => {
  try {
    return await processDocument(job.data)
  } catch (error) {
    if (error instanceof TemplateMissingError) {
      throw new UnrecoverableError(error.message)
    }
    throw error
  }
})
```

### Compensação em falha definitiva

```typescript
worker.on('failed', async (job, error) => {
  if (job && job.attemptsMade >= (job.opts.attempts ?? 3)) {
    await creditService.refund(job.data.userId, job.data.creditCost)
    logger.warn('Créditos reembolsados após falha definitiva', {
      jobId: job.id,
      userId: job.data.userId,
    })
  }
})
```

### Estratégias de retry

| Estratégia | Config | Quando considerar |
| ------------ | -------- | ------------------- |
| Exponencial | `{ type: 'exponential', delay: 2000 }` | API externa, rate limit |
| Fixa | `{ type: 'fixed', delay: 5000 }` | Falha temporária previsível |
| Custom | `backoffStrategy` callback | Lógica condicional |

## Monitoramento

### Progresso do job

```typescript
await job.updateProgress({ step: 'generating', percent: 50 })

const events = new QueueEvents('documents.generate', { connection })
events.on('progress', ({ jobId, data }) => {
  io.to(`job:${jobId}`).emit('progress', data)
})
```

### Métricas da queue

```typescript
const counts = await queue.getJobCounts(
  'active', 'completed', 'delayed', 'failed', 'waiting',
)
logger.info('Queue metrics', counts)
```

## Anti-patterns

| Anti-pattern | Alternativa |
| -------------- | ------------- |
| Operação síncrona bloqueante no worker | Adotar `async/await` no processamento |
| Ignorar evento `stalled` | Escutar e logar |
| Job data sem tipagem | Definir type explícito |
| `removeOnComplete: true` para tudo | Preferir `{ count: N }` para manter histórico |
| Retry infinito | Limitar `attempts` e ter dead-letter |
| Processar sem `updateProgress` | Reportar progresso, útil para UI e monitoramento |

## Variações comuns por projeto

Cada projeto tende a adotar convenções próprias. Alguns padrões observados:

- Classe abstrata base para consumers (ex.: `DocumentGenerator`) padronizando setup e eventos.
- Streaming de progresso em tempo real (ex.: Socket.IO + Redis adapter).
- Pipeline engine orquestrando vários passos dentro de um único job, com progresso por passo.
- Jobs agendados delegados a outra biblioteca (ex.: Agenda, BullMQ `repeatable jobs`, cron externo).

Use como inspiração; adapte à arquitetura do projeto.

## Checklist

- [ ] Queue com `defaultJobOptions` (attempts, backoff, removeOnComplete).
- [ ] Job data tipado.
- [ ] Worker com `concurrency` definido.
- [ ] Eventos `completed`, `failed` e `stalled` tratados.
- [ ] `UnrecoverableError` para falhas que não devem ser retentadas.
- [ ] Compensação (ex.: reembolso de créditos) em falha definitiva, quando aplicável.
- [ ] Progresso reportado via `updateProgress`.
- [ ] Conexão com Redis configurada via variáveis de ambiente.
