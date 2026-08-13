# Tipos Avançados — Detalhamento

## Discriminated Unions

### Padrão Result

```ts
type Success<T> = { success: true; data: T }
type Failure = { success: false; error: string }
type Result<T> = Success<T> | Failure

const result = await fetchData()
if (result.success) {
  result.data // tipado como T
} else {
  result.error // tipado como string
}
```

### Variante ok/error

```ts
type ProfileResult =
  | { ok: true; profile: Profile }
  | { ok: false; status: number; error: string }
```

### Mensagens cross-context

```ts
type Message =
  | { type: 'CAPTURE_START'; payload: { url: string } }
  | { type: 'AUTH_STATE_CHANGED' }
  | { type: 'CHECK_SUBSCRIPTION' }
```

### Eliminar null em branches tipados

```ts
type ActiveSubscription = {
  error?: undefined
  isActive: true
  profile: Profile
}

type InactiveSubscription = {
  profile?: undefined
  error: string
  isActive: false
}

type Status = ActiveSubscription | InactiveSubscription

// Dentro do branch, profile existe com tipo Profile (não Profile | undefined)
if (status.isActive) {
  status.profile
}
```

---

## Type Guards

### Com type predicate

```ts
const isStreamError = (error: unknown): error is StreamError =>
  error instanceof Error && 'code' in error

const isPdfDocument = (doc: Document): doc is PdfDocument =>
  doc.type === 'pdf'
```

### Com `in` operator

```ts
const isPetitionMetadata = (
  metadata?: SourceMetadata,
): metadata is PetitionMetadata =>
  !!metadata && 'slug' in metadata && 'shortLink' in metadata
```

### Em filter (narrowing)

```ts
const elements = items
  .map(item => findElement(item))
  .filter((el): el is HTMLElement => el !== null)

const strings = mixed
  .filter((item): item is string => typeof item === 'string')
```

### Nullish guard

```ts
const isNullish = (value: unknown): value is null | undefined =>
  value === null || value === undefined
```

### Exhaustive check com `never`

```ts
const handleStatus = (status: Status) => {
  switch (status) {
    case 'active': return 'Ativo'
    case 'inactive': return 'Inativo'
    default: {
      const exhaustive: never = status
      return exhaustive
    }
  }
}
```

O branch `never` garante, em compile time, que adicionar um novo valor à união força o tratamento.

---

## Generics

### Constraint com `extends`

```ts
const updateItem = <T extends { id: string }>(
  items: T[],
  updated: T,
): T[] => items.map(item => (item.id === updated.id ? updated : item))
```

### Default type parameter

```ts
const useFetch = <T = unknown>(url: string) => {}
```

### Utility types derivados

```ts
type PromiseValue<T> = T extends PromiseLike<infer V>
  ? PromiseValue<V>
  : T

type UseCaseReturn<Fn extends (...args: any) => any> =
  Awaited<ReturnType<Fn>>

type DebouncedFn<T extends (...args: any[]) => any> = {
  (...args: Parameters<T>): void
  cancel: () => void
  flush: () => ReturnType<T> | undefined
}
```

### Conditional types

```ts
type WriteOperation = Extract<DbOperation, 'insert' | 'update'>

type OperationResult<T extends DbOperation> =
  T extends WriteOperation ? AffectedRows : QueryResult
```

---

## Mapped Types

### `Record` para mapeamento

```ts
const PERMISSIONS: Record<PermissionName, PermissionConfig> = { /* ... */ }
const LABELS: Record<PlanType, string> = { /* ... */ }
```

### Mapped optional

```ts
type HandlerMap = {
  [Key in EventType]?: (data: string | number) => Promise<void>
}
```

### `Partial` para updates

```ts
type UpdateOptions<T> = {
  data: PaginatedData<T>
  itemId: string
  updates: Partial<T>
}
```

---

## Tipos globais (`.d.ts`)

Arquivos `.d.ts` sem `export` podem disponibilizar tipos globalmente:

```ts
// plan.d.ts — sem export = global
type PlanType = 'basic' | 'advanced' | 'ia' | 'teams'
type PlanState = 'active' | 'inactive' | 'canceled'
```

Use com cuidado: tipos globais dificultam rastrear origem e acoplam todos os módulos. Em muitos projetos, imports explícitos são preferíveis.

### Module augmentation

```ts
// environment.d.ts
declare global {
  namespace NodeJS {
    interface ProcessEnv {
      DATABASE_URL: string
      REDIS_URL: string
      JWT_SECRET: string
      NODE_ENV: 'development' | 'production' | 'test'
    }
  }
}

// express.d.ts
declare global {
  namespace Express {
    interface Request {
      userId?: string
      role?: string
    }
  }
}
```

---

## Padrões adicionais

### `readonly` em arrays imutáveis

```ts
type ListOfAllowedPermissions =
  | readonly ['fullAccess']
  | readonly PlanType[]
  | readonly []

const shuffled = <T>(items: readonly T[]): T[] => {
  const copy = [...items]
  // mutar copy, não items
  return copy
}
```

### Indexed access

```ts
type UserSummary = {
  role: User['role']
  email: User['email']
}
```

### `satisfies` para validar sem widening

```ts
const message = {
  role: 'human',
  content: text,
} satisfies LangGraphMessage
```

### `Extract` / `Exclude`

```ts
type AdminRole = Extract<UserRole, 'admin' | 'superadmin'>
type NonAdminRole = Exclude<UserRole, 'admin' | 'superadmin'>
type WritePermission = Extract<Permission, 'create' | 'update' | 'delete'>
```

### `Partial<Record<K, V>>`

```ts
type ErrorHandlerMap = Partial<
  Record<ErrorCode, { statusCode: number; message: string }>
>

const rateLimits: Partial<Record<UserRole, number>> = {
  admin: 1000,
  user: 100,
}
```

### `Error &` para augmentar `Error`

```ts
type StreamError = Error & { code: StreamErrorCode }
type DatabaseError = Error & { table: string; constraint?: string }
```

### Intersecção para composição de payloads

```ts
type BasePayload = { timestamp: string; userId?: string }
type CollectionUpdate = BasePayload & { collection: Collection }
type DocumentFinished = BasePayload & { id: string; status: 'finished' }
```

### `keyof typeof` para derivar union de const object

```ts
const NOTIFICATION_TYPES = {
  email: { label: 'E-mail', icon: 'mail' },
  push: { label: 'Push', icon: 'bell' },
  sms: { label: 'SMS', icon: 'phone' },
} as const satisfies Record<string, NotificationConfig>

type NotificationType = keyof typeof NOTIFICATION_TYPES
// → 'email' | 'push' | 'sms'
```

Adicionar ou remover chave atualiza a união automaticamente.

### Tuple type alias

```ts
type DateRange = [start: Date, end: Date]
type Coordinate = [lat: number, lng: number]
```

### `Partial<Omit<T, 'discriminant'>>`

```ts
type UpdateUserParams = {
  userId: string
  overrides?: Partial<Omit<User, 'id' | 'createdAt'>>
}
```

### `Partial<T['field']>` para sub-objetos parciais

```ts
type UpdateSubscriptionParams = Partial<{
  billing: Partial<Subscription['billing']>
  limits: Partial<Subscription['limits']>
}>
```
