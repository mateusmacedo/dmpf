---
name: skill-error-handling
description: >
  Use esta skill quando o usuário pedir para "hierarquia de erros", "error middleware",
  "exception filter", "tratamento de exceção", "try/catch pattern", "Result pattern",
  ou mencionar CustomError, erros de domínio, HTTP helpers, logging de erros, retry em falhas.
  Cobre hierarquia de erros customizados, middleware de erro (Express), exception filters (NestJS),
  HTTP response helpers, Result pattern, logging estruturado e timeouts em chamadas externas.
  Para validação de input, ver `skill-validation`.
model: opus
---

# Error Handling

## Objetivo

Tratamento de erros: hierarquia de erros customizados, middlewares, exception filters, HTTP helpers e logging.

## Quando usar

- Ao definir hierarquia de erros de domínio.
- Ao configurar middleware de erro (Express) ou exception filter (NestJS).
- Ao integrar logging e monitoramento.

## Princípios

1. Evite expor detalhes internos — mensagens amigáveis para o cliente.
2. Registre erros completos para debugging.
3. Erros de domínio são lançados nos use cases e capturados na borda (controller/middleware/filter).
4. Permita retry quando fizer sentido.

## Hierarquia de erros customizados

```typescript
abstract class CustomError extends Error {
  abstract statusCode: number
  abstract code: string

  constructor(message: string) {
    super(message)
    this.name = this.constructor.name
  }
}

class NotFoundError extends CustomError {
  statusCode = 404
  code = 'NOT_FOUND'

  constructor(resource: string, id?: string) {
    super(id ? `${resource} ${id} não encontrado` : `${resource} não encontrado`)
  }
}

class UnauthorizedError extends CustomError {
  statusCode = 401
  code = 'UNAUTHORIZED'

  constructor(message = 'Não autorizado') {
    super(message)
  }
}

class ValidationError extends CustomError {
  statusCode = 400
  code = 'VALIDATION_ERROR'

  constructor(message: string, public readonly fields?: Record<string, string>) {
    super(message)
  }
}

class InsufficientCreditsError extends CustomError {
  statusCode = 402
  code = 'INSUFFICIENT_CREDITS'

  constructor(required: number, available: number) {
    super(`Créditos insuficientes: necessário ${required}, disponível ${available}`)
  }
}

class ConflictError extends CustomError {
  statusCode = 409
  code = 'CONFLICT'

  constructor(message: string) {
    super(message)
  }
}
```

## Middleware de erro (Express)

```typescript
const errorMiddleware = (
  err: Error,
  req: Request,
  res: Response,
  _next: NextFunction
) => {
  if (err instanceof CustomError) {
    return res.status(err.statusCode).json({
      error: err.code,
      message: err.message,
    })
  }

  console.error('Unhandled error:', err)
  return res.status(500).json({
    error: 'INTERNAL_ERROR',
    message: 'Erro interno do servidor',
  })
}
```

## Exception filter (NestJS)

```typescript
@Catch()
class AllExceptionsFilter implements ExceptionFilter {
  catch(exception: unknown, host: ArgumentsHost) {
    const ctx = host.switchToHttp()
    const response = ctx.getResponse<Response>()

    if (exception instanceof CustomError) {
      return response.status(exception.statusCode).json({
        error: exception.code,
        message: exception.message,
      })
    }

    if (exception instanceof HttpException) {
      return response.status(exception.getStatus()).json({
        error: 'HTTP_ERROR',
        message: exception.message,
      })
    }

    console.error('Unhandled exception:', exception)
    return response.status(500).json({
      error: 'INTERNAL_ERROR',
      message: 'Erro interno do servidor',
    })
  }
}
```

## HTTP response helpers

Úteis em Express. Em NestJS os controllers costumam retornar o payload diretamente.

```typescript
const ok = <T>(res: Response, data: T) =>
  res.status(200).json(data)

const created = <T>(res: Response, data: T) =>
  res.status(201).json(data)

const badRequest = (res: Response, message: string) =>
  res.status(400).json({ error: 'BAD_REQUEST', message })

const unauthorized = (res: Response, message = 'Não autorizado') =>
  res.status(401).json({ error: 'UNAUTHORIZED', message })

const notFound = (res: Response, message = 'Recurso não encontrado') =>
  res.status(404).json({ error: 'NOT_FOUND', message })

const conflict = (res: Response, message: string) =>
  res.status(409).json({ error: 'CONFLICT', message })
```

## Erros de domínio em use cases

```typescript
// Use case lança erros de domínio
const executeTransferCredits = async (input: TransferCreditsInput) => {
  const sender = await userRepository.findById(input.senderId)
  if (!sender) throw new NotFoundError('User', input.senderId)

  if (sender.credits < input.amount) {
    throw new InsufficientCreditsError(input.amount, sender.credits)
  }

  const receiver = await userRepository.findById(input.receiverId)
  if (!receiver) throw new NotFoundError('User', input.receiverId)

  sender.credits -= input.amount
  receiver.credits += input.amount

  await userRepository.save(sender)
  await userRepository.save(receiver)

  return { sender, receiver }
}

// Controller delega ao middleware/filter
const transferCreditsController = async (req: Request, res: Response, next: NextFunction) => {
  try {
    const result = await executeTransferCredits(req.body)
    return ok(res, result)
  } catch (error) {
    next(error)
  }
}
```

## Try/catch

```typescript
const fetchData = async () => {
  try {
    const response = await fetch('/api/data')

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }

    return await response.json()
  } catch (error) {
    console.error('Fetch error:', error)
    throw new Error('Não foi possível carregar os dados')
  }
}
```

## Result pattern

```typescript
type Result<T, E = Error> =
  | { success: true; data: T }
  | { success: false; error: E }

const safeParseJSON = <T>(json: string): Result<T> => {
  try {
    return { success: true, data: JSON.parse(json) }
  } catch (error) {
    return { success: false, error: error as Error }
  }
}

// Uso
const result = safeParseJSON<User>(data)
if (result.success) {
  console.log(result.data)
} else {
  console.error(result.error)
}
```

## Logging estruturado

```typescript
const logError = (
  error: unknown,
  context?: Record<string, unknown>
) => {
  const entry = {
    level: 'error',
    message: error instanceof Error ? error.message : String(error),
    stack: error instanceof Error ? error.stack : undefined,
    timestamp: new Date().toISOString(),
    ...context,
  }

  console.error(JSON.stringify(entry))
}

// Uso
try {
  await riskyOperation()
} catch (error) {
  logError(error, { userId, action: 'riskyOperation' })
}
```

## Network errors com timeout

```typescript
const fetchWithTimeout = async (
  url: string,
  options?: RequestInit & { timeout?: number }
) => {
  const { timeout = 10000, ...fetchOptions } = options ?? {}

  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  try {
    const response = await fetch(url, {
      ...fetchOptions,
      signal: controller.signal,
    })
    return response
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') {
      throw new Error('Tempo limite excedido')
    }
    throw error
  } finally {
    clearTimeout(timeoutId)
  }
}
```

## Checklist

- [ ] Hierarquia de erros customizados definida.
- [ ] Middleware de erro (Express) ou exception filter (NestJS) configurado.
- [ ] Erros de domínio lançados nos use cases.
- [ ] Controllers delegam erros ao middleware/filter.
- [ ] Try/catch em operações async relevantes.
- [ ] Mensagens amigáveis ao cliente.
- [ ] Logging completo no servidor.
- [ ] Timeout em chamadas externas.
- [ ] Sem catch vazio.
- [ ] Sem stack trace exposto ao cliente.

---

## 🔹 Go: tratamento de erros idiomático

Em Go, erros são valores retornados, não exceções. Os princípios (não vazar internals, registrar contexto, propagar) são iguais; a mecânica difere.

### Princípios em Go

1. Função que pode falhar retorna `(T, error)`.
2. Quem chama trata o erro imediatamente — geralmente com `if err != nil { return ... }`.
3. Wrap erros com contexto via `fmt.Errorf("%w", err)` ao propagar.
4. Use sentinelas (`errors.Is`) para identidade e tipos customizados (`errors.As`) para extrair detalhes.

### Hierarquia via tipos customizados

Não há herança. Use struct + interface `error`:

```go
package apperror

import (
    "errors"
    "fmt"
    "net/http"
)

type AppError struct {
    Code    string
    Message string
    Status  int
    Cause   error
}

func (e *AppError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

// Helpers para construir erros tipados
func NotFound(resource, id string) *AppError {
    return &AppError{
        Code:    "NOT_FOUND",
        Message: fmt.Sprintf("%s %s não encontrado", resource, id),
        Status:  http.StatusNotFound,
    }
}

func Unauthorized(msg string) *AppError {
    if msg == "" {
        msg = "não autorizado"
    }
    return &AppError{Code: "UNAUTHORIZED", Message: msg, Status: http.StatusUnauthorized}
}

func Validation(msg string, details map[string]string) *AppError {
    return &AppError{
        Code:    "VALIDATION_ERROR",
        Message: msg,
        Status:  http.StatusBadRequest,
    }
}

func Conflict(msg string) *AppError {
    return &AppError{Code: "CONFLICT", Message: msg, Status: http.StatusConflict}
}
```

### Sentinel errors

Para erros sem dados associados, use variáveis sentinela exportadas:

```go
package user

import "errors"

var (
    ErrNotFound          = errors.New("user not found")
    ErrInsufficientFunds = errors.New("insufficient funds")
    ErrEmailTaken        = errors.New("email already taken")
)
```

Comparação:

```go
u, err := repo.FindByID(ctx, id)
if errors.Is(err, user.ErrNotFound) {
    return apperror.NotFound("User", id)
}
```

### Wrapping com contexto

```go
func (uc *CreateUserUseCase) Execute(ctx context.Context, in Input) (*User, error) {
    if err := in.Validate(); err != nil {
        return nil, fmt.Errorf("validate input: %w", err)
    }

    u, err := uc.users.Save(ctx, NewUser(in))
    if err != nil {
        return nil, fmt.Errorf("save user: %w", err)
    }
    return u, nil
}
```

`%w` preserva a cadeia de erro (acessível via `errors.Is`/`errors.As`).

### Middleware HTTP (net/http)

```go
type errorHandler func(http.ResponseWriter, *http.Request) error

func wrap(h errorHandler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            handleError(w, r, err)
        }
    })
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {
    var appErr *apperror.AppError
    if errors.As(err, &appErr) {
        writeJSON(w, appErr.Status, errorResponse{
            Error:   appErr.Code,
            Message: appErr.Message,
        })
        return
    }

    log.ErrorContext(r.Context(), "unhandled error", "err", err)
    writeJSON(w, http.StatusInternalServerError, errorResponse{
        Error:   "INTERNAL_ERROR",
        Message: "erro interno do servidor",
    })
}

type errorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}
```

### Middleware Gin

```go
func ErrorMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        if len(c.Errors) == 0 {
            return
        }

        err := c.Errors.Last().Err
        var appErr *apperror.AppError
        if errors.As(err, &appErr) {
            c.JSON(appErr.Status, gin.H{"error": appErr.Code, "message": appErr.Message})
            return
        }

        slog.ErrorContext(c.Request.Context(), "unhandled error", "err", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "INTERNAL_ERROR", "message": "erro interno",
        })
    }
}
```

### Result-like (raro em Go)

Go não tem `Result` idiomático — `(T, error)` é o padrão. Quando o erro NÃO é exceção (ex.: lookup que pode legitimamente vir vazio), retorne `(*T, error)` e use `nil` para "não existe":

```go
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
    var u User
    err := r.db.QueryRowContext(ctx, `...`).Scan(&u.ID, &u.Email)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil  // não é erro: usuário não existe
    }
    if err != nil {
        return nil, fmt.Errorf("find by email: %w", err)
    }
    return &u, nil
}
```

### Logging estruturado (slog)

```go
import "log/slog"

slog.ErrorContext(ctx, "operation failed",
    "err", err,
    "user_id", userID,
    "operation", "transferCredits",
)
```

`slog` (stdlib desde Go 1.21) integra com OpenTelemetry e respeita `context`.

### Timeouts em chamadas externas

```go
func fetchData(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("new request: %w", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("http %d", resp.StatusCode)
    }
    return io.ReadAll(resp.Body)
}
```

`context.WithTimeout` propaga cancelamento para client, queries e goroutines filhas.

### Anti-patterns (Go)

```go
// Ignorar erro
data, _ := json.Marshal(input)

// Wrapping vazio (sem contexto)
return fmt.Errorf("%w", err)  // desnecessário, basta `return err`

// panic em código de aplicação
if err != nil { panic(err) }  // panic é para programming errors, não runtime

// recover() para mascarar bug
defer func() { _ = recover() }()  // esconde bugs reais

// Comparar erro por string
if err.Error() == "not found" {}  // frágil: usar errors.Is
```

### Checklist (Go)

- [ ] Funções que falham retornam `(T, error)`.
- [ ] Erros propagam com `fmt.Errorf("%w", err)` adicionando contexto.
- [ ] Sentinelas (`errors.Is`) para identidade; tipos customizados (`errors.As`) para detalhes.
- [ ] Middleware/handler central converte `*AppError` em resposta HTTP.
- [ ] `errcheck` linter ativo (não há erros silenciosamente ignorados).
- [ ] `context.Context` propagado para timeout/cancelamento.
- [ ] Logging estruturado (`slog`) sem dados sensíveis.
- [ ] Sem `panic`/`recover` para fluxo de controle.
