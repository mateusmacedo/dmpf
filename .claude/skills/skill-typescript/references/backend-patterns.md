# Padrões de Backend — Entities, Repositories, DTOs, DI, Jobs e Middleware

Os exemplos usam TypeORM, NestJS, Express e BullMQ como referência. Os princípios são transferíveis para equivalentes (Prisma, Fastify, outras libs de fila).

## TypeORM Entity Patterns

### Tipo da entity

```ts
type UserEntity = {
  id: string
  email: string
  name: string
  createdAt: Date
  updatedAt: Date
}
```

### Mapeamento de tipos (Column ↔ TypeScript)

| Column Type | TypeScript |
|-------------|-----------|
| `varchar`, `text` | `string` |
| `int`, `bigint` | `number` |
| `timestamp`, `date` | `Date` |
| `boolean` | `boolean` |
| `jsonb` | `Record<string, unknown>` |
| `uuid` | `string` |

### Relações

```ts
type PetitionEntity = {
  id: string
  title: string
  userId: string        // FK gerada pela relação @ManyToOne
  user?: UserEntity     // carregada opcionalmente
}
```

### Enum via union type

```ts
const DOCUMENT_STATUSES = ['draft', 'published', 'archived'] as const
type DocumentStatus = typeof DOCUMENT_STATUSES[number]
```

Union type costuma ser mais leve em runtime que `enum`; `enum` continua válido quando requerido por integração.

---

## Tipagem de repositório

### Interface simples

```ts
type UserRepository = {
  findById: (id: string) => Promise<User | null>
  save: (user: User) => Promise<User>
  delete: (id: string) => Promise<void>
}
```

### Namespace para agrupar I/O

Um padrão comum em arquiteturas mais estruturadas é agrupar Input/Output por operação:

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

### Repository genérico

```ts
type BaseRepository<T extends { id: string }> = {
  findById: (id: string) => Promise<T | null>
  save: (entity: Omit<T, 'id'>) => Promise<T>
  delete: (id: string) => Promise<void>
}
```

---

## DTOs

### Baseado em schema de validação

```ts
const createUserSchema = z.object({
  name: z.string().min(1),
  email: z.string().email(),
  role: z.enum(['admin', 'user']).default('user'),
})

type CreateUserDto = z.infer<typeof createUserSchema>
```

### Baseado em classe (class-validator / NestJS)

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

Escolher entre schema e classe depende do framework, ferramentas de validação e preferência do projeto.

---

## NestJS — Decorators tipados

### Controller com params tipados

```ts
@Get(':id')
const findById = (@Param('id') id: string): Promise<User> => {
  return userService.findById(id)
}
```

### Guard com request tipado

```ts
type AuthenticatedUser = {
  userId: string
  role: 'admin' | 'user'
  email: string
}

const user = request.user as AuthenticatedUser
```

### Pipe transform tipado

```ts
type ParsedQuery = {
  page: number
  limit: number
  sort: 'asc' | 'desc'
}

const parseQuery = (raw: Record<string, string>): ParsedQuery => ({
  page: Number(raw.page) || 1,
  limit: Math.min(Number(raw.limit) || 20, 100),
  sort: raw.sort === 'desc' ? 'desc' : 'asc',
})
```

---

## BullMQ Job Types

### Dados e resultado tipados

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
  return { fileUrl: url, pageCount: pages }
}
```

### Queue map tipado

```ts
type QueueMap = {
  'document:generate': DocumentGenerateJobData
  'email:send': EmailSendJobData
  'subscription:expire': SubscriptionExpireJobData
}
```

---

## Express Request/Response

### Tipos genéricos

```ts
type HttpRequest = {
  body: Record<string, unknown>
  params: Record<string, string>
  query: Record<string, string>
  headers: Record<string, string>
  locals?: { userId?: string; role?: string }
}

type HttpResponse<T = unknown> = {
  statusCode: number
  data: T
}
```

### Handler tipado

```ts
type HttpHandler = (
  request: HttpRequest,
) => Promise<HttpResponse>
```

---

## Factory / DI

Padrão de injeção por função (alternativa a containers de DI):

### Setup function

```ts
type Dependencies = {
  userRepository: UserRepository
  hashService: HashService
}

type Setup = (deps: Dependencies) => UseCase
type UseCase = (input: Input) => Promise<Output>
```

### Implementação

```ts
export const setup: Setup = ({ userRepository, hashService }) =>
  async (input) => {
    const hashed = await hashService.hash(input.password)
    return userRepository.save({ ...input, password: hashed })
  }
```

Containers de DI (tsyringe, InversifyJS, módulo do NestJS) são alternativas equivalentes, com trade-offs de verbosidade e magia.

---

## Middleware

### Middleware como tipo

```ts
type Middleware = {
  handle: (httpRequest: HttpRequest) => Promise<HttpResponse | void>
}
```

### Composição

```ts
type MiddlewareChain = Middleware[]

const applyMiddlewares = async (
  chain: MiddlewareChain,
  request: HttpRequest,
): Promise<HttpResponse | void> => {
  for (const mw of chain) {
    const result = await mw.handle(request)
    if (result) return result
  }
}
```

### Auth middleware tipado

```ts
type AuthMiddleware = Middleware & {
  roles?: UserRole[]
}
```
