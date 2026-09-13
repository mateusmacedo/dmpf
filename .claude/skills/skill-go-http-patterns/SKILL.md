---
name: skill-go-http-patterns
description: |
  Use esta skill ao trabalhar com frameworks HTTP em Go: net/http, chi, gin, echo, fiber.
  Cobre roteamento, dependency injection (constructor + Wire/Fx), middleware, validação,
  autenticação JWT, autorização baseada em role, recover, structured errors, request lifecycle,
  graceful shutdown, structured logging com slog.
model: opus
---

# Go HTTP — padrões

## Objetivo

Reunir padrões de aplicações HTTP em Go: organização de handlers, dependency injection, middleware, autenticação, autorização e tratamento de erros padronizado.

## Quando usar

- Ao criar ou modificar handlers HTTP.
- Ao configurar DI no composition root.
- Ao implementar handlers com validação de request.
- Ao adicionar middleware de auth/authz.
- Ao criar interceptadores de logging ou recover.

## Estrutura de aplicação

### Layout idiomático

```
.
├── cmd/
│   └── api/
│       └── main.go              # composition root
├── internal/
│   ├── user/
│   │   ├── user.go              # entidade
│   │   ├── repository.go        # interface
│   │   ├── service.go           # use case
│   │   └── http/
│   │       ├── handler.go
│   │       ├── dto.go
│   │       └── routes.go
│   ├── auth/
│   │   ├── token.go
│   │   ├── middleware.go
│   │   └── claims.go
│   ├── infra/
│   │   ├── postgres/
│   │   └── redis/
│   └── shared/
│       ├── httpx/               # respond helpers, errors
│       └── middleware/
└── go.mod
```

### Pacote por bounded context

Cada feature vira pacote em `internal/`. Pacote `http` interno carrega handlers; `infra/postgres` carrega implementações de repositório.

---

## Roteamento

### net/http (Go 1.22+)

```go
mux := http.NewServeMux()
mux.HandleFunc("GET  /users/{id}",    h.FindByID)
mux.HandleFunc("POST /users",          h.Create)
mux.HandleFunc("PATCH /users/{id}",    h.Update)
mux.HandleFunc("DELETE /users/{id}",   h.Delete)
```

A stdlib (1.22+) suporta path values e métodos. Suficiente para muitos casos.

### chi (idiomático, leve)

```go
import "github.com/go-chi/chi/v5"

r := chi.NewRouter()
r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)

r.Route("/users", func(r chi.Router) {
    r.Get("/", h.List)
    r.Post("/", h.Create)
    r.Get("/{id}", h.FindByID)

    r.Group(func(r chi.Router) {
        r.Use(authMiddleware)
        r.Patch("/{id}", h.Update)
        r.Delete("/{id}", h.Delete)
    })
})
```

`chi` mantém a API próxima de `net/http` — handlers são `http.HandlerFunc`.

### gin / echo / fiber

Frameworks com mais features (binding, validation, response helpers) e API próprio.

```go
// gin
r := gin.New()
r.Use(gin.Recovery())
users := r.Group("/users")
users.POST("/", h.Create)
users.GET("/:id", h.FindByID)
```

Trade-off: ergonomia vs aderência ao stdlib. `chi` é o "meio do caminho".

---

## Dependency injection

### Constructor injection

```go
type UserHandler struct {
    create *createuser.UseCase
    find   *finduser.UseCase
    log    *slog.Logger
}

func NewUserHandler(
    create *createuser.UseCase,
    find   *finduser.UseCase,
    log    *slog.Logger,
) *UserHandler {
    return &UserHandler{create: create, find: find, log: log}
}
```

Sem container, sem reflection. Se o construtor cresce demais (>5 deps), o struct provavelmente acumula responsabilidades — divida.

### Composition root (main.go)

```go
func main() {
    cfg := config.MustLoad()

    db := mustConnectDB(cfg.DatabaseURL)
    rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Repositories
    userRepo := postgres.NewUserRepository(db)

    // Use cases
    createUser := createuser.New(userRepo, bcryptHasher.New(), clock.System{})
    findUser   := finduser.New(userRepo)

    // Handlers
    userHandler := userhttp.NewHandler(createUser, findUser, log)

    // Routes
    r := chi.NewRouter()
    r.Use(httpx.Recoverer(log), httpx.RequestID, httpx.AccessLog(log))
    userhttp.MountRoutes(r, userHandler, authMW)

    runServer(r, cfg.Port, log)
}
```

### Wire (Google) — DI gerada

```go
//go:build wireinject

func InitializeAPI(cfg config.Config) (*API, func(), error) {
    wire.Build(
        db.Connect,
        postgres.NewUserRepository,
        wire.Bind(new(user.Repository), new(*postgres.UserRepository)),
        bcryptHasher.New,
        createuser.New,
        finduser.New,
        userhttp.NewHandler,
        NewAPI,
    )
    return nil, nil, nil
}
```

`wire generate` produz código sem reflection. Útil em apps grandes.

### Fx (Uber) — DI runtime

```go
fx.New(
    fx.Provide(
        config.Load,
        db.Connect,
        postgres.NewUserRepository,
        createuser.New,
        userhttp.NewHandler,
    ),
    fx.Invoke(StartServer),
).Run()
```

Adiciona reflection e lifecycle hooks. Trade-off: magia vs explicitness.

---

## Handlers

### Estrutura básica (chi)

```go
type Handler struct {
    create *createuser.UseCase
    find   *finduser.UseCase
    log    *slog.Logger
}

func MountRoutes(r chi.Router, h *Handler, authMW func(http.Handler) http.Handler) {
    r.Route("/users", func(r chi.Router) {
        r.Post("/", h.Create)
        r.Get("/{id}", h.FindByID)
        r.With(authMW).Delete("/{id}", h.Delete)
    })
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var dto CreateUserDto
    if err := httpx.DecodeJSON(r, &dto); err != nil {
        httpx.Respond(w, http.StatusBadRequest, "invalid_json", err)
        return
    }
    if err := validate.Struct(dto); err != nil {
        httpx.Respond(w, http.StatusUnprocessableEntity, "validation_failed", err)
        return
    }
    out, err := h.create.Execute(r.Context(), dto.toInput())
    if err != nil {
        httpx.RespondAppError(w, h.log, err)
        return
    }
    httpx.JSON(w, http.StatusCreated, toResponse(out))
}
```

### Path / query params

```go
// chi
id := chi.URLParam(r, "id")

// net/http (1.22+)
id := r.PathValue("id")

// query com defaults
page, _ := strconv.Atoi(r.URL.Query().Get("page"))
if page < 1 {
    page = 1
}
```

### Sem retornar response — Go usa `ResponseWriter`

Diferente de NestJS, handler escreve direto em `http.ResponseWriter`. Padronize via helpers (`httpx.JSON`, `httpx.RespondError`).

---

## Validação de request

### go-playground/validator

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New(validator.WithRequiredStructEnabled())

type CreateUserDto struct {
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Email string `json:"email" validate:"required,email"`
    Role  string `json:"role"  validate:"required,oneof=admin user"`
}
```

### Decode + validate em helper

```go
package httpx

func DecodeJSON(r *http.Request, dst any) error {
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields() // equivalente a forbidNonWhitelisted
    if err := dec.Decode(dst); err != nil {
        return fmt.Errorf("decode body: %w", err)
    }
    if err := validate.Struct(dst); err != nil {
        return fmt.Errorf("validate: %w", err)
    }
    return nil
}
```

### Custom validator

```go
func init() {
    validate.RegisterValidation("uuid_v4", validateUUIDv4)
}

func validateUUIDv4(fl validator.FieldLevel) bool {
    _, err := uuid.Parse(fl.Field().String())
    return err == nil
}

type FindUserDto struct {
    ID string `json:"id" validate:"required,uuid_v4"`
}
```

---

## Middleware

### Padrão `func(http.Handler) http.Handler`

```go
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(ww, r)
            log.Info("request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", ww.status,
                "duration", time.Since(start),
            )
        })
    }
}

type statusRecorder struct {
    http.ResponseWriter
    status int
}
func (s *statusRecorder) WriteHeader(code int) {
    s.status = code
    s.ResponseWriter.WriteHeader(code)
}
```

### Composição

```go
r.Use(
    Recoverer(log),
    RequestID,
    AccessLog(log),
    CORS(corsConfig),
    RateLimit(redisLimiter),
)
```

Ordem importa. Recoverer primeiro garante captura de panic em qualquer middleware seguinte.

### Auth middleware (JWT)

```go
type ctxKey string
const userKey ctxKey = "user"

type AuthenticatedUser struct {
    ID    string
    Email string
    Role  Role
}

func AuthMiddleware(verify TokenVerifier) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)
            if token == "" {
                httpx.Respond(w, http.StatusUnauthorized, "missing_token", nil)
                return
            }
            user, err := verify(r.Context(), token)
            if err != nil {
                httpx.Respond(w, http.StatusUnauthorized, "invalid_token", err)
                return
            }
            ctx := context.WithValue(r.Context(), userKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func extractToken(r *http.Request) string {
    if c, err := r.Cookie("token"); err == nil {
        return c.Value
    }
    h := r.Header.Get("Authorization")
    return strings.TrimPrefix(h, "Bearer ")
}

func UserFromContext(ctx context.Context) (*AuthenticatedUser, bool) {
    u, ok := ctx.Value(userKey).(*AuthenticatedUser)
    return u, ok
}
```

### Role-based authorization

```go
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
    allowed := make(map[Role]bool, len(roles))
    for _, r := range roles {
        allowed[r] = true
    }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            u, ok := UserFromContext(r.Context())
            if !ok || !allowed[u.Role] {
                httpx.Respond(w, http.StatusForbidden, "forbidden", nil)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// uso
r.With(authMW, RequireRole(RoleAdmin)).Delete("/users/{id}", h.Delete)
```

### Recoverer

```go
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    log.Error("panic",
                        "panic", rec,
                        "stack", string(debug.Stack()),
                    )
                    httpx.Respond(w, http.StatusInternalServerError, "internal_error", nil)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Tratamento de erros

### AppError padronizado

```go
type AppError struct {
    Code       string
    Message    string
    StatusCode int
    Cause      error
}

func (e *AppError) Error() string  { return e.Message }
func (e *AppError) Unwrap() error  { return e.Cause }

var (
    ErrNotFound     = &AppError{Code: "not_found", Message: "resource not found", StatusCode: 404}
    ErrUnauthorized = &AppError{Code: "unauthorized", Message: "unauthorized", StatusCode: 401}
    ErrConflict     = &AppError{Code: "conflict", Message: "resource conflict", StatusCode: 409}
)
```

### Helper de resposta

```go
package httpx

type errorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func RespondAppError(w http.ResponseWriter, log *slog.Logger, err error) {
    var ae *AppError
    if errors.As(err, &ae) {
        JSON(w, ae.StatusCode, errorResponse{Code: ae.Code, Message: ae.Message})
        return
    }
    log.Error("unhandled", "err", err)
    JSON(w, http.StatusInternalServerError, errorResponse{
        Code: "internal_error", Message: "internal error",
    })
}

func JSON(w http.ResponseWriter, status int, payload any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(payload)
}
```

### Mapeamento status × código

| Status | Código sugerido | Quando |
| -------- | ---------------- | -------- |
| 400 | `invalid_request` | Body/query mal formado |
| 401 | `unauthorized` | Token inválido/ausente |
| 403 | `forbidden` | Sem permissão |
| 404 | `not_found` | Recurso ausente |
| 409 | `conflict` | Duplicidade |
| 422 | `validation_failed` | Regra de negócio violada |
| 429 | `too_many_requests` | Rate limit |
| 500 | `internal_error` | Inesperado |

---

## Logging com slog

```go
log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

log.Info("user created", "userID", u.ID, "email", u.Email)
log.Warn("retry", "attempt", n, "err", err)
log.Error("save user", "err", err, "userID", u.ID)
```

`slog` é stdlib (Go 1.21+). Para texto colorido em dev, use `lmittmann/tint`.

### Logger no contexto da request

```go
func WithLogger(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            reqID := middleware.GetReqID(r.Context())
            l := log.With("requestID", reqID, "path", r.URL.Path)
            ctx := context.WithValue(r.Context(), loggerKey, l)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

## Graceful shutdown

```go
func runServer(handler http.Handler, port int, log *slog.Logger) {
    srv := &http.Server{
        Addr:              ":" + strconv.Itoa(port),
        Handler:           handler,
        ReadHeaderTimeout: 5 * time.Second,
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    go func() {
        log.Info("server starting", "addr", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Error("server", "err", err)
            stop()
        }
    }()

    <-ctx.Done()
    log.Info("shutdown signal received")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Error("graceful shutdown", "err", err)
    }
}
```

Ordem ao encerrar:

1. Parar de aceitar requisições novas (`Shutdown` faz isso).
2. Aguardar requisições em voo até o timeout.
3. Fechar workers/queues, conexões DB e cache.

---

## Anti-patterns

| Anti-pattern | Preferir |
| -------------- | ---------- |
| Handler com lógica de negócio | Mover para use case/service |
| `panic` em fluxo de request | Retornar erro; `panic` só em programming errors |
| Token em query string | Header `Authorization` ou cookie HttpOnly |
| Middleware modificando body | Decodificar uma vez no handler |
| `interface{}` no contexto sem cast | Use chave tipada (`type ctxKey string`) |
| Goroutine sem `context` no handler | Sempre passar `r.Context()` |
| `http.Server` sem `ReadHeaderTimeout` | Definir timeout para evitar slowloris |
| Sem `defer Body.Close()` em client | Sempre fechar response.Body |

---

## Recursos comuns

| Pacote | Uso |
| -------- | ----- |
| `github.com/go-chi/chi/v5` | Router idiomático |
| `github.com/gin-gonic/gin` | Router com helpers |
| `github.com/go-playground/validator/v10` | Validação por struct tag |
| `github.com/golang-jwt/jwt/v5` | JWT |
| `golang.org/x/time/rate` | Rate limiting in-memory |
| `github.com/unrolled/secure` | Security headers |
| `github.com/rs/cors` | CORS |
| `github.com/swaggo/swag` | Swagger doc generator |
| `go.uber.org/fx` / `github.com/google/wire` | DI |
| `log/slog` (stdlib) | Logging estruturado |

---

## Checklist

- [ ] `internal/` para módulos privados; pacote por bounded context.
- [ ] Constructor injection — sem global state.
- [ ] Handler delega para use case (sem regra de negócio).
- [ ] DTOs validados com tags `validate` + helper `DecodeJSON`.
- [ ] Auth middleware coloca user no `context`; handlers leem via `UserFromContext`.
- [ ] Recoverer middleware captura panics.
- [ ] AppError + helper `RespondAppError` padronizam status/código.
- [ ] `slog` estruturado com `requestID`.
- [ ] `http.Server` com `ReadHeaderTimeout`.
- [ ] `signal.NotifyContext` + `srv.Shutdown` para graceful shutdown.
- [ ] Sem `panic` em fluxo de request.
- [ ] Sem dependência circular entre pacotes (Go falha em compilar).
