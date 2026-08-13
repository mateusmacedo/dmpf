# Go — padrões de backend

HTTP handlers, repositórios, middleware, dependency injection e configuração. Os exemplos usam `net/http` da stdlib e `chi`/`gin` como referências; princípios são transferíveis para Echo, Fiber, etc.

## Entity patterns

### Entity como struct

```go
type User struct {
    ID        string
    Email     string
    Name      string
    Role      Role
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Sem decorators. Persistência fica no repositório, não na entity.

### Mapeamento coluna ↔ Go (database/sql)

| SQL | Go | Nullable |
|-----|-----|---------|
| `varchar`, `text` | `string` | `sql.NullString` |
| `int`, `bigint` | `int64` | `sql.NullInt64` |
| `timestamp`, `date` | `time.Time` | `sql.NullTime` |
| `boolean` | `bool` | `sql.NullBool` |
| `jsonb`, `json` | `[]byte`, struct customizada | — |
| `uuid` | `string`, `uuid.UUID` | `*uuid.UUID` |

### Relações explícitas

```go
type Petition struct {
    ID     string
    Title  string
    UserID string  // FK
    User   *User   // carregado via JOIN ou query separada
}
```

Não há lazy loading idiomático — JOIN explícito ou query adicional.

### Enum via tipo string + iota

```go
type DocumentStatus string

const (
    StatusDraft     DocumentStatus = "draft"
    StatusPublished DocumentStatus = "published"
    StatusArchived  DocumentStatus = "archived"
)

func (s DocumentStatus) Valid() bool {
    switch s {
    case StatusDraft, StatusPublished, StatusArchived:
        return true
    }
    return false
}
```

Para enums numéricos, `iota`:

```go
type Priority int

const (
    PriorityLow Priority = iota
    PriorityMedium
    PriorityHigh
)
```

---

## Repositórios

### Interface no consumidor

```go
// internal/user/repository.go
package user

type Repository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, u *User) (*User, error)
    Delete(ctx context.Context, id string) error
}
```

Interface fica no pacote do domínio. Implementação fica em `infra/`.

### Implementação concreta retornando struct

```go
// internal/infra/postgres/user_repository.go
package postgres

type UserRepository struct {
    db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*user.User, error) {
    var row userRow
    err := r.db.GetContext(ctx, &row, `SELECT id, email, name, role, created_at, updated_at FROM users WHERE id = $1`, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, user.ErrNotFound
        }
        return nil, fmt.Errorf("find user %s: %w", id, err)
    }
    return row.toDomain(), nil
}
```

### Linha do DB ≠ entity de domínio

```go
type userRow struct {
    ID        string    `db:"id"`
    Email     string    `db:"email"`
    Name      string    `db:"name"`
    Role      string    `db:"role"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}

func (r userRow) toDomain() *user.User {
    return &user.User{
        ID:        r.ID,
        Email:     r.Email,
        Name:      r.Name,
        Role:      user.Role(r.Role),
        CreatedAt: r.CreatedAt,
        UpdatedAt: r.UpdatedAt,
    }
}
```

Separação evita vazar detalhes de persistência no domínio.

### Repositório genérico (Go 1.18+)

```go
type Identifiable interface {
    GetID() string
}

type BaseRepository[T Identifiable] interface {
    FindByID(ctx context.Context, id string) (T, error)
    Save(ctx context.Context, entity T) (T, error)
    Delete(ctx context.Context, id string) error
}
```

Use com moderação — repositórios geralmente têm queries específicas que não cabem em CRUD genérico.

---

## DTOs

### DTO com tags

```go
type CreateUserDto struct {
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Email string `json:"email" validate:"required,email"`
    Role  Role   `json:"role"  validate:"omitempty,oneof=admin user"`
}
```

### Decode + validate em handler

```go
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var dto CreateUserDto
    if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
        respondError(w, http.StatusBadRequest, "invalid_json", err)
        return
    }
    if err := validate.Struct(dto); err != nil {
        respondError(w, http.StatusUnprocessableEntity, "validation_failed", err)
        return
    }
    user, err := h.useCase.Execute(r.Context(), dto.toInput())
    if err != nil {
        respondAppError(w, err)
        return
    }
    respondJSON(w, http.StatusCreated, toResponse(user))
}
```

### Métodos de mapping no DTO

```go
func (d CreateUserDto) toInput() createuser.Input {
    return createuser.Input{
        Name:  d.Name,
        Email: d.Email,
        Role:  user.Role(d.Role),
    }
}
```

DTO é o boundary. Use case recebe Input do domínio.

---

## HTTP handlers

### net/http puro

```go
type Handler struct {
    useCase *createuser.UseCase
    log     *slog.Logger
}

func NewHandler(uc *createuser.UseCase, log *slog.Logger) *Handler {
    return &Handler{useCase: uc, log: log}
}

func (h *Handler) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", h.Create)
    mux.HandleFunc("GET /users/{id}", h.FindByID)
    return mux
}

func (h *Handler) FindByID(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    // ...
}
```

`http.ServeMux` da stdlib (Go 1.22+) suporta path values e métodos.

### Chi router

```go
import "github.com/go-chi/chi/v5"

r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(middleware.Logger)

r.Route("/users", func(r chi.Router) {
    r.Post("/", h.Create)
    r.Get("/{id}", h.FindByID)
    r.Group(func(r chi.Router) {
        r.Use(authMiddleware)
        r.Patch("/{id}", h.Update)
        r.Delete("/{id}", h.Delete)
    })
})
```

### Gin

```go
import "github.com/gin-gonic/gin"

r := gin.New()
r.Use(gin.Recovery(), loggerMiddleware)

users := r.Group("/users")
{
    users.POST("/", h.Create)
    users.GET("/:id", h.FindByID)
}
```

### Helpers de resposta

```go
func respondJSON(w http.ResponseWriter, status int, payload any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(payload); err != nil {
        slog.Error("encode response", "err", err)
    }
}

type errorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func respondError(w http.ResponseWriter, status int, code string, err error) {
    respondJSON(w, status, errorResponse{Code: code, Message: err.Error()})
}

func respondAppError(w http.ResponseWriter, err error) {
    var ae *AppError
    if errors.As(err, &ae) {
        respondJSON(w, ae.StatusCode, errorResponse{Code: string(ae.Code), Message: ae.Message})
        return
    }
    respondJSON(w, http.StatusInternalServerError, errorResponse{Code: "internal_error", Message: "internal error"})
}
```

### Path/Query params

```go
// net/http (1.22+)
id := r.PathValue("id")

// query
page, _ := strconv.Atoi(r.URL.Query().Get("page"))
limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

// chi
id := chi.URLParam(r, "id")

// gin
id := c.Param("id")
page := c.DefaultQuery("page", "1")
```

---

## Middleware

### Função middleware (net/http)

```go
type Middleware func(http.Handler) http.Handler

func loggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        slog.Info("request",
            "method", r.Method,
            "path", r.URL.Path,
            "duration", time.Since(start),
        )
    })
}
```

### Composição

```go
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}

handler := Chain(mux,
    requestIDMiddleware,
    loggerMiddleware,
    authMiddleware,
    rateLimitMiddleware,
)
```

### Auth middleware com context

```go
type ctxKey string
const userKey ctxKey = "user"

type AuthenticatedUser struct {
    UserID string
    Role   Role
    Email  string
}

func authMiddleware(verify TokenVerifier) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)
            user, err := verify(r.Context(), token)
            if err != nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), userKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func userFromContext(ctx context.Context) (*AuthenticatedUser, bool) {
    u, ok := ctx.Value(userKey).(*AuthenticatedUser)
    return u, ok
}
```

### Middleware com role

```go
func requireRole(roles ...Role) Middleware {
    allowed := make(map[Role]bool, len(roles))
    for _, r := range roles {
        allowed[r] = true
    }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            u, ok := userFromContext(r.Context())
            if !ok || !allowed[u.Role] {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// uso
mux.Handle("DELETE /users/{id}", Chain(deleteHandler, requireRole(RoleAdmin)))
```

### Recover middleware

```go
func recoverMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                slog.Error("panic recovered",
                    "panic", rec,
                    "stack", string(debug.Stack()),
                )
                http.Error(w, "internal error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

---

## Dependency injection

### Constructor function (preferido)

```go
// internal/user/service.go
type Service struct {
    repo   Repository
    hasher Hasher
    clock  Clock
}

func NewService(repo Repository, hasher Hasher, clock Clock) *Service {
    return &Service{repo: repo, hasher: hasher, clock: clock}
}
```

DI é literalmente passar dependências no construtor. Sem container, sem reflection.

### Composition root (main.go)

```go
// cmd/api/main.go
func main() {
    cfg := config.MustLoad()

    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
    defer rdb.Close()

    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Wire dependencies
    userRepo  := postgres.NewUserRepository(db)
    hasher    := bcryptHasher.New(cfg.BcryptCost)
    clock     := clock.System{}
    userSvc   := user.NewService(userRepo, hasher, clock)
    userHandler := userhttp.NewHandler(userSvc, log)

    // Server
    mux := http.NewServeMux()
    mux.Handle("/users", userHandler.Routes())

    srv := &http.Server{
        Addr:              ":" + strconv.Itoa(cfg.Port),
        Handler:           Chain(mux, recoverMiddleware, loggerMiddleware),
        ReadHeaderTimeout: 5 * time.Second,
    }

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Error("server", "err", err)
        }
    }()

    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    srv.Shutdown(shutdownCtx)
}
```

### Wire (Google) — DI gerada em compile time

```go
// wire.go
//+build wireinject

func InitializeAPI(cfg Config) (*API, error) {
    wire.Build(
        postgres.NewUserRepository,
        bcryptHasher.New,
        clock.NewSystem,
        user.NewService,
        userhttp.NewHandler,
        NewAPI,
    )
    return nil, nil
}
```

`wire` gera o código de wiring. Útil em projetos grandes com muitos componentes.

### Fx (Uber) — DI runtime

```go
fx.New(
    fx.Provide(
        config.Load,
        db.Connect,
        postgres.NewUserRepository,
        user.NewService,
        userhttp.NewHandler,
    ),
    fx.Invoke(StartServer),
).Run()
```

Útil para apps complexas com lifecycle. Adiciona reflection — trade-off vs explicitness.

---

## Configuração

### Carregar via env (caarlos0/env)

```go
import "github.com/caarlos0/env/v11"

type Config struct {
    Port        int    `env:"PORT" envDefault:"8080"`
    DatabaseURL string `env:"DATABASE_URL,required"`
    RedisURL    string `env:"REDIS_URL,required"`
    JWTSecret   string `env:"JWT_SECRET,required"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
    Env         string `env:"ENV" envDefault:"development"`
}

func MustLoad() *Config {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        log.Fatal(err)
    }
    return &cfg
}
```

### Validação em startup

```go
func (c *Config) Validate() error {
    if c.JWTSecret == "" || len(c.JWTSecret) < 32 {
        return errors.New("JWT_SECRET must be at least 32 chars")
    }
    if c.Env != "development" && c.Env != "production" && c.Env != "test" {
        return fmt.Errorf("invalid ENV: %s", c.Env)
    }
    return nil
}
```

Falhe no boot, não em runtime.

---

## Graceful shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Error("server", "err", err)
    }
}()

<-ctx.Done()
log.Info("shutdown signal received")

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Error("server shutdown", "err", err)
}

// fechar workers, conexões, drenar filas
worker.Stop()
db.Close()
rdb.Close()
```

Ordem:
1. Parar de aceitar requisições novas (`srv.Shutdown`).
2. Aguardar requisições em voo terminarem (até timeout).
3. Drenar workers/queues.
4. Fechar conexões de DB/cache.

---

## Worker / job patterns

### Worker com graceful shutdown (asynq)

```go
import "github.com/hibiken/asynq"

func main() {
    redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisURL}
    srv := asynq.NewServer(redisOpt, asynq.Config{
        Concurrency: 10,
        Queues:      map[string]int{"critical": 6, "default": 3, "low": 1},
    })

    mux := asynq.NewServeMux()
    mux.HandleFunc(TaskDocumentGenerate, HandleDocumentGenerate)
    mux.HandleFunc(TaskEmailSend, HandleEmailSend)

    if err := srv.Run(mux); err != nil {
        log.Fatal(err)
    }
}
```

`asynq` cuida do graceful shutdown (sinais, drain de jobs em voo).

### Job idempotency

```go
func HandleEmailSend(ctx context.Context, t *asynq.Task) error {
    var p EmailSendJob
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return fmt.Errorf("unmarshal: %w", err)
    }

    sent, err := store.AlreadySent(ctx, p.IdempotencyKey)
    if err != nil {
        return fmt.Errorf("check idempotency: %w", err)
    }
    if sent {
        return nil // sucesso silencioso
    }

    if err := mailer.Send(ctx, p.To, p.Subject, p.Body); err != nil {
        return fmt.Errorf("send mail: %w", err) // retry automático
    }

    return store.MarkSent(ctx, p.IdempotencyKey)
}
```

### Retry e backoff

`asynq` suporta retry exponencial via `asynq.MaxRetry`/`asynq.ProcessIn`. Erros não retornados (panic recuperado) também rotacionam.
