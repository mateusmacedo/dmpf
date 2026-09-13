---
name: skill-validation
description: |
  Orientação para validação de dados em aplicações backend: definição de
  schemas, inferência de tipos, validação de request, tratamento de erros
  de validação e classes de erro tipadas. Exemplos usam Zod como referência;
  o princípio se aplica a qualquer biblioteca de validação equivalente.
model: opus
---

# Validação

## Objetivo

Padrões para validação de dados: schemas declarativos, inferência de tipos, validação de entrada e erros de domínio. A biblioteca concreta (Zod, io-ts, class-validator, Joi, Yup, etc.) é parâmetro do projeto; os exemplos usam Zod como uma opção comum.

## Quando aplicar

- Validação de payloads e inputs externos.
- Compartilhamento de schemas entre camadas (client/server, módulos internos).
- Padronização de erros de validação.
- Criação de classes de erro tipadas (ex.: `AppError`, `ValidationError`).

## Schemas (exemplo com Zod)

```typescript
import { z } from 'zod'

export const UserSchema = z.object({
  id: z.string().uuid(),
  email: z.string().email(),
  name: z.string().min(1).max(100),
  role: z.enum(['admin', 'user']),
  createdAt: z.date(),
})

export const CreateUserSchema = UserSchema.omit({
  id: true,
  createdAt: true,
})

export const UpdateUserSchema = CreateUserSchema.partial()

export type User = z.infer<typeof UserSchema>
export type CreateUserInput = z.infer<typeof CreateUserSchema>
export type UpdateUserInput = z.infer<typeof UpdateUserSchema>
```

Derivar tipos a partir do schema evita divergência entre validação e tipo estático.

## Validação de request

```typescript
const validateRequest = <T>(schema: z.Schema<T>, data: unknown): T => {
  const result = schema.safeParse(data)

  if (!result.success) {
    throw new ValidationError(result.error.errors)
  }

  return result.data
}

const data = validateRequest(IncomingPayloadSchema, requestBody)
```

Executar a validação antes de qualquer efeito colateral reduz a superfície para dados inválidos.

## Classes de erro

```typescript
export class AppError extends Error {
  constructor(
    public code: string,
    message: string,
    public details?: unknown
  ) {
    super(message)
    this.name = 'AppError'
  }
}

export class NotFoundError extends AppError {
  constructor(resource: string, id: string) {
    super('NOT_FOUND', `${resource} with id ${id} not found`)
  }
}

export class ValidationError extends AppError {
  constructor(details: unknown) {
    super('VALIDATION_ERROR', 'Invalid input', details)
  }
}

export class UnauthorizedError extends AppError {
  constructor(message = 'Unauthorized') {
    super('UNAUTHORIZED', message)
  }
}
```

## Error handler

```typescript
type ErrorResult = {
  success: false
  error: { code: string; message: string; details?: unknown }
}

export const handleError = (error: unknown): ErrorResult => {
  console.error('Error:', error)

  if (error instanceof AppError) {
    return {
      success: false,
      error: {
        code: error.code,
        message: error.message,
        details: error.details,
      },
    }
  }

  if (error instanceof z.ZodError) {
    return {
      success: false,
      error: {
        code: 'VALIDATION_ERROR',
        message: 'Invalid input',
        details: error.errors,
      },
    }
  }

  return {
    success: false,
    error: {
      code: 'INTERNAL_ERROR',
      message: 'An unexpected error occurred',
    },
  }
}
```

Evite expor detalhes internos em mensagens destinadas a clientes externos.

## Padrões avançados

### Validadores customizados

```typescript
const PasswordSchema = z
  .string()
  .min(8, 'Senha deve ter no mínimo 8 caracteres')
  .regex(/[A-Z]/, 'Deve conter letra maiúscula')
  .regex(/[0-9]/, 'Deve conter número')
  .regex(/[^a-zA-Z0-9]/, 'Deve conter caractere especial')

const SlugSchema = z
  .string()
  .regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, 'Slug inválido')
```

### Transformações

```typescript
const DateStringSchema = z
  .string()
  .transform((val) => new Date(val))
  .pipe(z.date())

const TrimmedStringSchema = z
  .string()
  .transform((val) => val.trim())

const NormalizedEmailSchema = z
  .string()
  .email()
  .transform((val) => val.toLowerCase())
```

### Refinements

```typescript
const DateRangeSchema = z
  .object({
    startDate: z.date(),
    endDate: z.date()
  })
  .refine(
    (data) => data.endDate > data.startDate,
    { message: 'Data final deve ser após data inicial' }
  )

const PasswordConfirmSchema = z
  .object({
    password: z.string().min(8),
    confirmPassword: z.string()
  })
  .refine(
    (data) => data.password === data.confirmPassword,
    {
      message: 'Senhas não conferem',
      path: ['confirmPassword']
    }
  )
```

### Union e discriminated union

```typescript
const IdSchema = z.union([z.string().uuid(), z.number().int()])

const EventSchema = z.discriminatedUnion('type', [
  z.object({
    type: z.literal('click'),
    x: z.number(),
    y: z.number()
  }),
  z.object({
    type: z.literal('keypress'),
    key: z.string()
  })
])
```

Discriminated union tende a ser mais eficiente e oferece narrowing mais preciso.

### Coerção

```typescript
const QueryParamsSchema = z.object({
  page: z.coerce.number().int().positive().default(1),
  limit: z.coerce.number().int().min(1).max(100).default(20),
  active: z.coerce.boolean().default(true)
})

const params = QueryParamsSchema.parse({
  page: '2',
  limit: '50',
  active: 'true'
})
```

Útil para query strings e form data, onde tudo chega como string.

### Validação assíncrona

```typescript
const UniqueEmailSchema = z
  .string()
  .email()
  .refine(
    async (email) => {
      const exists = await db.users.findByEmail(email)
      return !exists
    },
    { message: 'Email já cadastrado' }
  )

const result = await UniqueEmailSchema.safeParseAsync(email)
```

Use com cautela: validações assíncronas aumentam acoplamento do schema com I/O. Quando possível, separe validação de formato (síncrona) de verificação de unicidade (na camada de use case/service).

## Validação de formulários

```typescript
const LoginSchema = z.object({
  email: z
    .string({ error: 'Email obrigatório' })
    .email('Email inválido'),
  password: z
    .string({ error: 'Senha obrigatória' })
    .min(8, 'Senha deve ter no mínimo 8 caracteres'),
})
```

## Schemas recursivos

```typescript
const categorySchema: z.ZodType<Category> = z.object({
  name: z.string(),
  subcategories: z.lazy(() => z.array(categorySchema)),
})

type Category = {
  name: string
  subcategories: Category[]
}
```

## Records

```typescript
const permissionsSchema = z.record(
  z.string(),
  z.boolean(),
)
// { [key: string]: boolean }
```

## Checklist

- [ ] Inputs validados no limite da aplicação.
- [ ] Tipos derivados dos schemas quando possível.
- [ ] Erros tipados e diferenciados por código.
- [ ] Mensagens de erro legíveis, sem vazar internals.
- [ ] Transformações para normalizar dados (trim, lowercase, etc.).
- [ ] Refinements para regras compostas.
- [ ] Coerção em entradas que chegam como string.

---

## 🔹 Go: validação em Go

Go não tem schemas declarativos como Zod. As bibliotecas mais comuns são baseadas em struct tags ou DSL programática.

### Bibliotecas

| Biblioteca | Estilo | Quando usar |
| ------------ | -------- | ------------- |
| `go-playground/validator` | Tags em struct (`validate:"..."`) | Padrão de fato em projetos web |
| `ozzo-validation` | DSL programática | Quando regras dinâmicas |
| `go-ozzo/ozzo-validation/is` | Helpers (Email, URL, UUID) | Complementa ozzo |
| Manual | Função `Validate() error` | Casos simples ou domínio puro |

### `validator` — schema via tags

```go
import "github.com/go-playground/validator/v10"

type CreateUserRequest struct {
    Email string `json:"email" validate:"required,email,max=255"`
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Role  string `json:"role"  validate:"required,oneof=admin user"`
    Age   *int   `json:"age"   validate:"omitempty,gte=0,lte=150"`
}

var validate = validator.New()

func ValidateRequest[T any](data T) error {
    return validate.Struct(data)
}
```

### Validação de request HTTP

```go
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    if err := validate.Struct(req); err != nil {
        var ves validator.ValidationErrors
        if errors.As(err, &ves) {
            writeJSON(w, http.StatusBadRequest, formatErrors(ves))
            return
        }
        http.Error(w, "validation failed", http.StatusBadRequest)
        return
    }
    // ...
}

func formatErrors(ves validator.ValidationErrors) map[string][]string {
    out := map[string][]string{}
    for _, fe := range ves {
        out[fe.Field()] = append(out[fe.Field()], fe.Tag())
    }
    return out
}
```

### Hierarquia de erros tipados

```go
type AppError struct {
    Code    string
    Message string
    Details any
    Cause   error
}

func (e *AppError) Error() string  { return e.Message }
func (e *AppError) Unwrap() error  { return e.Cause }

var (
    ErrNotFound     = &AppError{Code: "NOT_FOUND", Message: "recurso não encontrado"}
    ErrUnauthorized = &AppError{Code: "UNAUTHORIZED", Message: "não autorizado"}
)

func ValidationError(details any) *AppError {
    return &AppError{Code: "VALIDATION_ERROR", Message: "input inválido", Details: details}
}
```

### Validadores customizados

```go
validate.RegisterValidation("strong_password", func(fl validator.FieldLevel) bool {
    s := fl.Field().String()
    return len(s) >= 8 &&
        regexp.MustCompile(`[A-Z]`).MatchString(s) &&
        regexp.MustCompile(`[0-9]`).MatchString(s) &&
        regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(s)
})

type RegisterRequest struct {
    Password string `validate:"required,strong_password"`
}
```

### Refinements (validação cruzada)

```go
type DateRange struct {
    StartDate time.Time `validate:"required"`
    EndDate   time.Time `validate:"required,gtfield=StartDate"`
}

type PasswordConfirm struct {
    Password        string `validate:"required,min=8"`
    ConfirmPassword string `validate:"required,eqfield=Password"`
}
```

`gtfield`, `eqfield`, `nefield`, `gtefield` cobrem comparações entre campos.

### Coerção (query strings)

```go
type ListQuery struct {
    Page   int    `form:"page,default=1" validate:"gte=1"`
    Limit  int    `form:"limit,default=20" validate:"gte=1,lte=100"`
    Active bool   `form:"active,default=true"`
    Status string `form:"status,omitempty" validate:"omitempty,oneof=draft published"`
}
```

Gin/Echo fazem o binding automaticamente. Em `net/http` puro, parse manual com `strconv`.

### `ozzo-validation` (DSL programática)

```go
import (
    validation "github.com/go-ozzo/ozzo-validation/v4"
    "github.com/go-ozzo/ozzo-validation/v4/is"
)

type User struct {
    Email string
    Name  string
    Age   int
}

func (u User) Validate() error {
    return validation.ValidateStruct(&u,
        validation.Field(&u.Email, validation.Required, is.Email),
        validation.Field(&u.Name, validation.Required, validation.Length(1, 100)),
        validation.Field(&u.Age, validation.Min(0), validation.Max(150)),
    )
}
```

Vantagem: regras dinâmicas (ex.: tornar campo obrigatório baseado em outro). Desvantagem: mais verboso.

### Validação manual (domínio)

Em entidades de domínio, expor construtor que valida invariantes em vez de tag:

```go
type Email struct{ value string }

func NewEmail(s string) (Email, error) {
    s = strings.TrimSpace(strings.ToLower(s))
    if !emailRegex.MatchString(s) {
        return Email{}, errors.New("email inválido")
    }
    return Email{value: s}, nil
}

func (e Email) String() string { return e.value }
```

Mantém o domínio independente da biblioteca de validação (idiomático em Go).

### Validação assíncrona (unicidade)

Em Go, validações que precisam de I/O ficam fora do schema — geralmente no use case:

```go
func (uc *CreateUser) Execute(ctx context.Context, in CreateUserRequest) (*User, error) {
    if err := validate.Struct(in); err != nil {
        return nil, ValidationError(in)
    }

    existing, err := uc.users.FindByEmail(ctx, in.Email)
    if err != nil {
        return nil, err
    }
    if existing != nil {
        return nil, &AppError{Code: "EMAIL_TAKEN", Message: "email já cadastrado"}
    }
    // ...
}
```

Isso separa validação síncrona (formato) de regra de negócio (unicidade).

### Discriminated union

Go não tem; emule com tag e type switch:

```go
type Event struct {
    Type    string          `json:"type" validate:"required,oneof=click keypress"`
    Payload json.RawMessage `json:"payload"`
}

type ClickEvent struct {
    X int `json:"x" validate:"required"`
    Y int `json:"y" validate:"required"`
}

func ParseEvent(raw []byte) (any, error) {
    var ev Event
    if err := json.Unmarshal(raw, &ev); err != nil {
        return nil, err
    }
    switch ev.Type {
    case "click":
        var c ClickEvent
        if err := json.Unmarshal(ev.Payload, &c); err != nil {
            return nil, err
        }
        return c, nil
    // ...
    }
    return nil, errors.New("tipo desconhecido")
}
```

### Anti-patterns (Go)

- Validar dentro do handler com `if`/`return` repetitivos — usar `validator.Struct`.
- Misturar validação de formato com regra de negócio (unicidade) no mesmo nível.
- Ignorar `error` de `validate.Struct`.
- Tags `validate` inconsistentes entre tipos relacionados.

### Checklist (Go)

- [ ] Inputs validados via `validator` ou validação manual em DTOs/requests.
- [ ] Validação no limite (handler/middleware), antes do use case.
- [ ] Validators customizados para regras específicas do domínio.
- [ ] Erros tipados (`*AppError` com `Code` e `Details`).
- [ ] Validação assíncrona (unicidade) feita na camada de use case, não no schema.
- [ ] Mensagens de erro sem vazar internals.
- [ ] Tags `validate` revisadas em PRs (não dá erro de build se ausentes).
