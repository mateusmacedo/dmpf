# Go — padrões de dados

Validação, structs JSON, services, DTOs, use cases e jobs em Go. Exemplos usam `go-playground/validator` como referência; os princípios se aplicam a qualquer biblioteca equivalente (`ozzo-validation`, etc.).

## Structs e validação

### Struct como fonte da verdade

Em Go, o struct é a fonte da verdade — tipos vêm dele, não de schemas externos.

```go
type CreateUserDto struct {
    Name  string `json:"name"  validate:"required,min=1"`
    Email string `json:"email" validate:"required,email"`
    Role  string `json:"role"  validate:"required,oneof=admin user"`
}
```

Tags `json` controlam serialização; tags `validate` controlam validação runtime.

### Validação runtime

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

func handleCreate(w http.ResponseWriter, r *http.Request) {
    var dto CreateUserDto
    if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
        http.Error(w, "invalid json", http.StatusBadRequest)
        return
    }
    if err := validate.Struct(dto); err != nil {
        // err é validator.ValidationErrors
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    // ...
}
```

### Validação em startup (config/env)

```go
type Config struct {
    DatabaseURL string `env:"DATABASE_URL,required"`
    RedisURL    string `env:"REDIS_URL,required"`
    JWTSecret   string `env:"JWT_SECRET,required"`
    Port        int    `env:"PORT" envDefault:"8080"`
}

func Load() (*Config, error) {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }
    return &cfg, nil
}

// main.go — fail fast
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}
```

### Constantes + tipo derivado

```go
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleUser   Role = "user"
    RoleViewer Role = "viewer"
)

func (r Role) Valid() bool {
    switch r {
    case RoleAdmin, RoleUser, RoleViewer:
        return true
    }
    return false
}
```

### Discriminated union (sum type via interface)

Go não tem união nativa. Padrão idiomático: interface + type switch.

```go
type JobResult interface {
    isJobResult()
}

type CompletedResult struct {
    FileURL   string
    PageCount int
}

func (CompletedResult) isJobResult() {}

type FailedResult struct {
    Reason string // "timeout", "invalid_template", ...
}

func (FailedResult) isJobResult() {}

func handle(r JobResult) {
    switch v := r.(type) {
    case CompletedResult:
        fmt.Println("done:", v.FileURL)
    case FailedResult:
        fmt.Println("failed:", v.Reason)
    }
}
```

`isJobResult()` não exportado restringe quem pode implementar — funciona como sealed type.

---

## Concorrência e async

### Context com timeout

```go
func loadUser(ctx context.Context, id string) (*User, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return repo.FindByID(ctx, id)
}
```

### errgroup para paralelismo

```go
import "golang.org/x/sync/errgroup"

func enrichUser(ctx context.Context, id string) (*EnrichedUser, error) {
    g, ctx := errgroup.WithContext(ctx)

    var user *User
    var subscription *Subscription
    var perms []Permission

    g.Go(func() error {
        u, err := userRepo.FindByID(ctx, id)
        user = u
        return err
    })
    g.Go(func() error {
        s, err := subRepo.FindByUserID(ctx, id)
        subscription = s
        return err
    })
    g.Go(func() error {
        p, err := permRepo.FindByUserID(ctx, id)
        perms = p
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, fmt.Errorf("enrich user %s: %w", id, err)
    }
    return &EnrichedUser{User: user, Subscription: subscription, Permissions: perms}, nil
}
```

`errgroup.WithContext` cancela siblings quando uma goroutine retorna erro.

### errgroup com limite

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10) // máx 10 goroutines simultâneas

for _, item := range items {
    item := item
    g.Go(func() error {
        return process(ctx, item)
    })
}
return g.Wait()
```

### Canal de timeout (alternativa a context)

```go
select {
case res := <-ch:
    return res, nil
case <-time.After(5 * time.Second):
    return nil, errors.New("timeout")
case <-ctx.Done():
    return nil, ctx.Err()
}
```

Prefira `context.WithTimeout` quando há cadeia de chamadas — propaga o cancelamento.

---

## Erros

### AppError com código

```go
type ErrorCode string

const (
    CodeNetworkError    ErrorCode = "network_error"
    CodeNotAuthenticated ErrorCode = "not_authenticated"
    CodeTimeout         ErrorCode = "timeout"
    CodeNotFound        ErrorCode = "not_found"
)

type AppError struct {
    Code       ErrorCode
    Message    string
    StatusCode int
    Cause      error
}

func (e *AppError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }
```

### Sentinelas vs tipo customizado

Sentinelas para casos canônicos:

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrConflict     = errors.New("conflict")
)

if errors.Is(err, ErrNotFound) {
    return http.StatusNotFound
}
```

Tipos para erros com dados:

```go
type ValidationError struct {
    Field  string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

var ve *ValidationError
if errors.As(err, &ve) {
    return http.StatusUnprocessableEntity, ve.Field
}
```

### Padrão Result via múltiplo retorno

Go não precisa de `Result<T, E>` — múltiplos retornos cobrem o caso:

```go
user, err := repo.FindByID(ctx, id)
if err != nil {
    return nil, fmt.Errorf("find user %s: %w", id, err)
}
```

### Catch silencioso (defer cleanup)

```go
defer func() {
    if cerr := tx.Rollback(); cerr != nil && !errors.Is(cerr, sql.ErrTxDone) {
        log.Warn("rollback failed", "err", cerr)
    }
}()
```

Sempre logue ou justifique antes de ignorar erros.

---

## Nullability

Go não tem `null`/`undefined`. Estratégias:

| Necessidade | Padrão idiomático |
|-------------|-------------------|
| Campo opcional | `*T` (ponteiro) |
| Distinguir "ausente" de "zero" | `*T` ou wrapper `sql.NullString` |
| Ausência em response | tag `json:",omitempty"` |
| Ausência em DB | `sql.Null*` (`NullString`, `NullInt64`, etc.) |

```go
type User struct {
    ID       string         `json:"id"`
    Email    string         `json:"email"`
    Name     *string        `json:"name,omitempty"`     // opcional
    DeletedAt sql.NullTime  `json:"-"`                   // nullable em DB
}
```

### Zero value útil

Projete tipos para que zero value seja válido:

```go
var b bytes.Buffer            // pronto para uso
var mu sync.Mutex             // pronto
var m map[string]int          // CUIDADO — nil map, não pode escrever
```

---

## Services — camada de dados

### Naming idiomático

| Tipo | Convenção | Exemplo |
|------|-----------|---------|
| Body request | `[Feature][Action]Request` | `SubscriptionCreateRequest` |
| Response | `[Feature][Action]Response` | `SubscriptionCreateResponse` |
| Function | `[Domain].[Action]` (método) | `subscriptions.Create` |

### Service struct + métodos

```go
package user

type Service struct {
    repo Repository
    log  *slog.Logger
}

func NewService(repo Repository, log *slog.Logger) *Service {
    return &Service{repo: repo, log: log}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*User, error) {
    u, err := s.repo.Save(ctx, req.toEntity())
    if err != nil {
        return nil, &AppError{
            Code:    CodeCreationFailed,
            Message: "failed to create user",
            Cause:   err,
        }
    }
    return u, nil
}
```

### Adapter — API externa → tipo de domínio

```go
package payment

// dto.go — schema cru da API externa
type gatewayResponse struct {
    TxnID       string `json:"txn_id"`
    TxnStatus   string `json:"txn_status"`
    AmountCents int64  `json:"amount_cents"`
}

// adapter.go — mapeia para tipo de domínio
func (r gatewayResponse) toDomain() *Payment {
    return &Payment{
        TransactionID: r.TxnID,
        Status:        Status(r.TxnStatus),
        Amount:        Money{Cents: r.AmountCents},
    }
}
```

DTOs são privados ao pacote (`camelCase`) — tipo de domínio é exportado.

---

## ORM Entity Typing

### Mapeamento coluna ↔ Go (database/sql + sqlx)

| SQL | Go | sql.Null* (nullable) |
|-----|-----|---------------------|
| `varchar`, `text` | `string` | `sql.NullString` |
| `int`, `bigint` | `int`, `int64` | `sql.NullInt64` |
| `timestamp`, `date` | `time.Time` | `sql.NullTime` |
| `boolean` | `bool` | `sql.NullBool` |
| `jsonb`, `json` | `[]byte` ou tipo customizado | `pgtype.JSONB` |
| `enum` | tipo string customizado | `sql.NullString` |
| `uuid` | `string` ou `uuid.UUID` | `*uuid.UUID` |

### Entity com tags

```go
type UserEntity struct {
    ID        string    `db:"id"         json:"id"`
    Email     string    `db:"email"      json:"email"`
    Name      string    `db:"name"       json:"name"`
    Role      Role      `db:"role"       json:"role"`
    Metadata  []byte    `db:"metadata"   json:"-"`        // jsonb cru
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}
```

### Relações (sem JOIN automático)

Em Go, JOINs são explícitos. Composição manual:

```go
type PetitionEntity struct {
    ID     string `db:"id"`
    Title  string `db:"title"`
    UserID string `db:"user_id"`
    User   *UserEntity `db:"-"` // carregado separadamente
}

func (r *Repo) FindWithUser(ctx context.Context, id string) (*PetitionEntity, error) {
    var p PetitionEntity
    if err := r.db.GetContext(ctx, &p, `SELECT ... FROM petitions WHERE id = $1`, id); err != nil {
        return nil, err
    }
    var u UserEntity
    if err := r.db.GetContext(ctx, &u, `SELECT ... FROM users WHERE id = $1`, p.UserID); err != nil {
        return nil, err
    }
    p.User = &u
    return &p, nil
}
```

`sqlc` e `ent` automatizam isso, mas com convenções próprias.

---

## DTO Typing

### DTO com tags de validação

```go
type CreateUserDto struct {
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Email string `json:"email" validate:"required,email"`
    Role  Role   `json:"role"  validate:"required,oneof=admin user"`
}

func (d CreateUserDto) Validate() error {
    return validate.Struct(d)
}

func (d CreateUserDto) ToEntity() *UserEntity {
    return &UserEntity{
        Name:  d.Name,
        Email: d.Email,
        Role:  d.Role,
    }
}
```

### Separar DTO de entity

```go
// internal/user/dto.go
type CreateUserDto struct { /* campos da request */ }
type UserResponse  struct { /* campos da response */ }

// internal/user/user.go
type User struct { /* campos do domínio */ }

// mapping
func toUserResponse(u *User) UserResponse { /* ... */ }
```

DTOs vivem na borda do sistema (HTTP, queue). Domínio não conhece DTOs.

---

## Use Case Typing

### Input/Output explícitos

```go
package createuser

type Input struct {
    Name  string
    Email string
    Role  user.Role
}

type Output struct {
    ID        string
    Name      string
    Email     string
    CreatedAt time.Time
}
```

### Use case como struct (constructor injection)

```go
type UseCase struct {
    repo   user.Repository
    hasher hasher.Service
    clock  clock.Clock
}

func New(repo user.Repository, hasher hasher.Service, clock clock.Clock) *UseCase {
    return &UseCase{repo: repo, hasher: hasher, clock: clock}
}

func (u *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
    // validação, regras, persistência
    entity := &user.User{
        Name:      in.Name,
        Email:     in.Email,
        Role:      in.Role,
        CreatedAt: u.clock.Now(),
    }
    saved, err := u.repo.Save(ctx, entity)
    if err != nil {
        return nil, fmt.Errorf("save user: %w", err)
    }
    return &Output{
        ID:        saved.ID,
        Name:      saved.Name,
        Email:     saved.Email,
        CreatedAt: saved.CreatedAt,
    }, nil
}
```

### Repositório por agregado (interface no consumidor)

```go
package user

// Repository é declarado aqui — quem precisa do User declara essa interface
type Repository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, u *User) (*User, error)
    Delete(ctx context.Context, id string) error
}
```

Implementação em `internal/infra/postgres/user_repository.go` retorna struct concreta.

---

## Job Typing (asynq, river, etc.)

### Payload tipado

```go
type DocumentGenerateJob struct {
    DocumentID   string `json:"documentId"`
    UserID       string `json:"userId"`
    TemplateType string `json:"templateType"`
}

type DocumentGenerateResult struct {
    FileURL   string `json:"fileUrl"`
    PageCount int    `json:"pageCount"`
}
```

### Handler tipado (asynq)

```go
import "github.com/hibiken/asynq"

const TaskDocumentGenerate = "document:generate"

func NewDocumentGenerateTask(p DocumentGenerateJob) (*asynq.Task, error) {
    payload, err := json.Marshal(p)
    if err != nil {
        return nil, fmt.Errorf("marshal payload: %w", err)
    }
    return asynq.NewTask(TaskDocumentGenerate, payload), nil
}

func HandleDocumentGenerate(ctx context.Context, t *asynq.Task) error {
    var p DocumentGenerateJob
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return fmt.Errorf("unmarshal: %w", err)
    }
    // processar
    return nil
}
```

### Queue map com generics

```go
type QueueName string

const (
    QueueDocumentGenerate    QueueName = "document:generate"
    QueueEmailSend           QueueName = "email:send"
    QueueSubscriptionExpire  QueueName = "subscription:expire"
)

type JobEnqueuer[T any] interface {
    Enqueue(ctx context.Context, queue QueueName, payload T) error
}

func enqueueDocumentGenerate(ctx context.Context, e JobEnqueuer[DocumentGenerateJob], p DocumentGenerateJob) error {
    return e.Enqueue(ctx, QueueDocumentGenerate, p)
}
```

### Idempotência

Inclua identificador determinístico no payload:

```go
type SendEmailJob struct {
    IdempotencyKey string `json:"idempotencyKey"` // hash de (userID, action, timestamp_minute)
    To             string `json:"to"`
    Template       string `json:"template"`
}
```

Worker checa cache/DB antes de processar — defesa contra retries.
