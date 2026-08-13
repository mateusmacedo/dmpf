# Padrões de Dados — Schemas, Async, Erros, Services, DTOs, Use Cases e Jobs

Exemplos usam Zod como referência; os princípios se aplicam a qualquer biblioteca de validação equivalente.

## Schemas + TypeScript

### Schema como fonte, tipo inferido

```ts
// schema.ts
export const createUserSchema = z.object({
  name: z.string().min(1),
  email: z.string().email(),
  role: z.enum(['admin', 'user']),
})

// types.ts
import type { z } from 'zod'
import type { createUserSchema } from './schema'
export type CreateUserDto = z.infer<typeof createUserSchema>
```

Derivar tipos do schema evita divergência entre validação runtime e tipo estático.

### Constantes + enum do schema

```ts
const ROLES = ['admin', 'user', 'viewer'] as const
const roleSchema = z.enum(ROLES)
type Role = typeof ROLES[number]
```

### `safeParse` para runtime, `parse` para startup

```ts
// Runtime — sem exceção
const result = schema.safeParse(data)
if (!result.success) return fallback

// Startup / validação de env — exceção intencional
const env = envSchema.parse(process.env)
```

### `z.input` vs `z.infer`

```ts
// z.infer = tipo de SAÍDA (após transforms)
type CreateUserDto = z.infer<typeof createUserSchema>

// z.input = tipo de ENTRADA (antes de transforms)
type CreateUserInput = z.input<typeof createUserSchema>
```

### Discriminated union com schema

```ts
const jobResultSchema = z.discriminatedUnion('status', [
  z.object({
    status: z.literal('completed'),
    data: z.object({
      fileUrl: z.string(),
      pageCount: z.number(),
    }),
  }),
  z.object({
    status: z.literal('failed'),
    error: z.enum(['timeout', 'invalid_template', 'generation_error']),
  }),
])
```

---

## Async

### Tipo de retorno explícito quando complexo

```ts
export const loadUser = async (id: string): Promise<User | null> => {}
```

Para APIs públicas e retornos não óbvios, declarar o tipo ajuda a API consumidora.

### Timer typing

```ts
let timer: ReturnType<typeof setTimeout> | undefined
```

### `error: unknown` em catch

```ts
try {
  await riskyOperation()
} catch (error: unknown) {
  console.error('Failed:', error)
}
```

`unknown` força narrowing antes de uso, evitando suposições sobre o erro.

### `Awaited` para resultado de async

```ts
type UserData = Awaited<ReturnType<typeof loadUser>>
```

### `Promise.race` com timeout

```ts
const rawResponse = await Promise.race([
  fetch('/api/check-subscription').then(r => r.json()),
  new Promise<never>((_, reject) => {
    timerId = setTimeout(() => reject(new Error('timeout')), 5_000)
  }),
])
```

### Async handler com tratamento de erros

```ts
const handleCreateUser = async (
  request: HttpRequest
): Promise<HttpResponse> => {
  try {
    const user = await createUser(request.body)
    return { statusCode: 201, data: user }
  } catch (error: unknown) {
    return { statusCode: 500, data: { error: 'internal_error' } }
  }
}
```

---

## Erros

### Union de códigos

```ts
type ErrorCode = 'network_error' | 'not_authenticated' | 'timeout'

type ApiError = {
  code: ErrorCode
  message: string
}
```

### Classe de erro customizada

```ts
export class AppError extends Error {
  constructor(
    public readonly code: ErrorCode,
    message: string,
    public readonly statusCode = 400,
  ) {
    super(message)
    this.name = 'AppError'
  }
}
```

### Erro com código tipado

```ts
type StreamErrorCode =
  | 'OFFLINE'
  | 'TIMEOUT'
  | 'SERVICE_UNAVAILABLE'

type StreamError = Error & { code: StreamErrorCode }
```

### Result pattern

```ts
type Result<T, E = string> =
  | { ok: true; data: T }
  | { ok: false; error: E }
```

### Catch vazio com comentário

```ts
.catch(() => {
  // noop — motivo explícito aqui
})
```

Catches silenciosos geralmente indicam falha não tratada. Quando forem realmente inócuos, justifique com comentário.

---

## Nullability

- `null` para ausência explícita (ex.: payload de API onde a chave existe com valor nulo).
- `undefined` para campos opcionais.
- `??` sobre `||` para nullish (evita falso positivo com `0` e `''`).
- `?.` para acesso condicional.
- Evitar `!` (non-null assertion): esconde a decisão de null-handling. Use type guards ou narrowing explícito.

### Discriminated unions eliminam null

```ts
if (result.ok) {
  result.data.email // narrowed, sem null
}
```

---

## Services — Camada de dados

### Naming de tipos

Padrão sugerido:

| Tipo | Convenção | Exemplo |
|------|-----------|---------|
| Body (POST/PATCH) | `[Feature][Action]Body` | `SubscriptionCreateBody` |
| Response | `[Feature][Action]Response` | `SubscriptionCreateResponse` |
| Path params | `[Feature][Action]PathParams` | `DocumentFindByIdPathParams` |
| Query params | `[Feature]FindQueryParams` | `PetitionFindQueryParams` |
| Request composto | `[Feature][Action]Request` | Compõe body + pathParams |

### Composição de request

```ts
type DocumentUpdateRequest = {
  body: DocumentUpdateBody
  pathParams: DocumentUpdatePathParams
}

const documentUpdate = async ({
  body,
  pathParams,
}: DocumentUpdateRequest): Promise<DocumentUpdateResponse> => { /* ... */ }
```

### Naming de funções

| Ação | Convenção | Exemplo |
|------|-----------|---------|
| Listar | `[domain]FindAll` | `petitionFindAll` |
| Buscar por ID | `[domain]FindById` | `documentFindById` |
| Criar | `[domain]Create` | `subscriptionCreate` |
| Atualizar | `[domain]Update` | `documentUpdate` |
| Deletar | `[domain]Delete` | `subscriptionDelete` |

Domínio primeiro, ação depois. A alternativa `findAllPetitions` também é válida; consistência dentro do projeto importa mais que a escolha.

### Retorno de serviço

```ts
export const subscriptionCreate = async (
  body: SubscriptionCreateBody,
): Promise<SubscriptionCreateResponse> => {
  const result = await repository.save(body)
  if (!result) throw new AppError('creation_failed', 'Erro ao criar assinatura.')
  return result
}
```

### Adapter — API externa → tipo de domínio

```ts
// types.ts — tipo cru da API externa
type PaymentGatewayResponse = {
  txn_id: string
  txn_status: string
  amount_cents: number
}

// adapter.ts — transforma em tipo de domínio
const paymentAdapter = (raw: PaymentGatewayResponse): Payment => ({
  transactionId: raw.txn_id,
  status: raw.txn_status as PaymentStatus,
  amount: raw.amount_cents / 100,
})
```

---

## TypeORM Entity Typing

### Mapeamento Column ↔ TypeScript

| TypeORM Column | Tipo TypeScript |
|----------------|-----------------|
| `varchar`, `text` | `string` |
| `int`, `bigint` | `number` |
| `timestamp`, `date` | `Date` |
| `boolean` | `boolean` |
| `jsonb`, `json` | `Record<string, unknown>` |
| `enum` | Union type (`'active' \| 'inactive'`) |
| `uuid` | `string` |

### Tipo da entity separado da entity class

```ts
// types.ts
type UserEntity = {
  id: string
  email: string
  name: string
  role: 'admin' | 'user'
  metadata?: Record<string, unknown>
  createdAt: Date
  updatedAt: Date
}
```

### Relações criam FK

```ts
type PetitionEntity = {
  id: string
  title: string
  userId: string          // foreign key gerada por @ManyToOne
  user?: UserEntity       // relação carregada (opcional)
}
```

### Enum via union type

```ts
const DOCUMENT_STATUSES = ['draft', 'published', 'archived'] as const
type DocumentStatus = typeof DOCUMENT_STATUSES[number]
```

---

## DTO Typing

### Baseado em schema de validação

```ts
// schema.ts
export const createUserSchema = z.object({
  name: z.string().min(1),
  email: z.string().email(),
  role: z.enum(['admin', 'user']).default('user'),
})

// types.ts
export type CreateUserDto = z.infer<typeof createUserSchema>
```

### Baseado em classe (class-validator)

```ts
class CreateUserDto {
  @IsNotEmpty()
  name: string

  @IsEmail()
  email: string

  @IsOptional()
  @IsEnum(['admin', 'user'])
  role?: string
}
```

---

## Use Case Typing

### Tipos Input/Output separados

```ts
// types.ts
type CreateUserInput = {
  name: string
  email: string
  role: 'admin' | 'user'
}

type CreateUserOutput = {
  id: string
  name: string
  email: string
  createdAt: Date
}
```

### Setup function (factory/DI)

```ts
// types.ts
type Dependencies = {
  userRepository: UserRepository
  hashService: HashService
}

type Setup = (deps: Dependencies) => UseCase
type UseCase = (input: CreateUserInput) => Promise<CreateUserOutput>

// index.ts
export const setup: Setup = ({ userRepository, hashService }) =>
  async (input) => {
    const hashed = await hashService.hash(input.password)
    return userRepository.save({ ...input, password: hashed })
  }
```

### Namespace para agrupar I/O de repository

```ts
namespace LoadUserRepository {
  export type Input = { id: string }
  export type Output = User | null
}

namespace SaveUserRepository {
  export type Input = Omit<User, 'id' | 'createdAt'>
  export type Output = User
}
```

---

## BullMQ Job Typing

### Dados do job tipados

```ts
type DocumentGenerateJobData = {
  documentId: string
  userId: string
  templateType: string
}

type DocumentGenerateJobResult = {
  fileUrl: string
  pageCount: number
}
```

### Processor tipado

```ts
const processDocumentGenerate = async (
  job: Job<DocumentGenerateJobData>,
): Promise<DocumentGenerateJobResult> => {
  const { documentId, templateType } = job.data
  // ... lógica de geração
  return { fileUrl: url, pageCount: pages }
}
```

### Queue com tipos genéricos

```ts
type QueueMap = {
  'document:generate': DocumentGenerateJobData
  'email:send': EmailSendJobData
  'subscription:expire': SubscriptionExpireJobData
}

const addJob = <K extends keyof QueueMap>(
  queue: K,
  data: QueueMap[K],
) => { /* ... */ }
```
