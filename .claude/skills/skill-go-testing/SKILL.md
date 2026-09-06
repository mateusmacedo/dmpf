---
name: skill-go-testing
description: >
  Use esta skill quando o usuário pedir para "configurar testes Go", "table-driven tests",
  "mock de interface", "teste de use case Go", "teste de handler Go", "testcontainers-go",
  ou mencionar testing stdlib, testify, gomock, mockery, httptest, build tags,
  t.Helper, t.Cleanup, race detector, coverage Go.
  Cobre setup do toolchain de teste em backend Go (testing stdlib, testify, gotestsum),
  table-driven tests, testes de use cases, handlers e repositórios (testcontainers-go),
  mock patterns (gomock, mockery, fakes manuais) e async/concurrent testing.
  Para padrões agnósticos, ver skill-unit-integration-testing.
model: sonnet
---

# Go testing (backend)

## Objetivo

Testes em Go backend: setup do toolchain, table-driven idiomático, testes de use cases/handlers/repositórios e padrões de mock.

## Quando usar

- Ao configurar a stack de testes em projeto Go.
- Ao testar use cases, handlers HTTP, repositórios ou workers.
- Ao criar mocks/fakes para dependências (repos, serviços, filas).

## Setup

### testing stdlib + testify

A `testing` da stdlib basta para a maioria dos casos. `testify/require`/`assert` reduz boilerplate.

```bash
go get github.com/stretchr/testify
```

### Estrutura de arquivos

Testes vivem ao lado do código testado, com sufixo `_test.go`:

```
internal/user/
├── usecase.go
├── usecase_test.go         # mesmo pacote — acesso a unexported
├── usecase_external_test.go # pacote user_test — só API pública
```

Convenções:
- Mesmo pacote (`package user`) → testa também detalhes internos.
- Pacote `_test` (`package user_test`) → testa só a API pública (preferível para validar contrato).

### Build tags para integração

```go
//go:build integration

package postgres_test
```

Roda só com `go test -tags=integration ./...`. Mantém testes lentos fora do CI rápido.

### gotestsum (output melhor)

```bash
go install gotest.tools/gotestsum@latest
gotestsum --format pkgname -- -race -coverprofile=cov.out ./...
```

---

## Comandos

```bash
go test ./...                              # roda todos
go test -run TestCreateUser ./internal/user # filtra por nome
go test -race ./...                        # detector de race
go test -cover -coverprofile=cov.out ./... # cobertura
go tool cover -html=cov.out                # HTML de cobertura
go test -count=1 ./...                     # ignora cache
go test -v -run TestX/subname              # subtest específico
go test -timeout 30s ./...                 # timeout global
```

---

## Estrutura de teste

### Padrão básico

```go
func TestCreateUser_validData(t *testing.T) {
    repo := &fakeUserRepo{}
    hasher := &fakeHasher{result: "hashed"}
    sut := user.NewService(repo, hasher)

    out, err := sut.Create(context.Background(), user.CreateInput{
        Name:  "John",
        Email: "john@test.com",
    })

    require.NoError(t, err)
    require.Equal(t, "John", out.Name)
}
```

`require.X` falha o teste imediatamente; `assert.X` registra a falha mas continua.

### Table-driven (idiomático)

```go
func TestParseEmail(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr error
    }{
        {"simple", "john@test.com", "john@test.com", nil},
        {"upper", "JOHN@TEST.COM", "john@test.com", nil},
        {"empty", "", "", ErrEmptyEmail},
        {"invalid", "not-email", "", ErrInvalidEmail},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseEmail(tt.input)
            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tt.want, got)
        })
    }
}
```

Cada caso vira um subtest via `t.Run`. Vantagens:
- `go test -run TestParseEmail/empty` roda só 1.
- Output mostra qual caso falhou.
- Adicionar caso = adicionar linha.

### t.Helper e t.Cleanup

```go
func makeUser(t *testing.T, opts ...UserOpt) *user.User {
    t.Helper() // stack trace pula esta função
    u := &user.User{ID: "u1", Email: "test@x.com"}
    for _, opt := range opts {
        opt(u)
    }
    return u
}

func setupDB(t *testing.T) *sql.DB {
    t.Helper()
    db := openTestDB(t)
    t.Cleanup(func() { db.Close() })
    return db
}
```

`t.Cleanup` é o equivalente Go ao `afterEach` — registra função que roda no fim do teste (mesmo em falha).

### Setup compartilhado (TestMain)

```go
var testDB *sql.DB

func TestMain(m *testing.M) {
    var err error
    testDB, err = openTestDB()
    if err != nil {
        log.Fatal(err)
    }
    code := m.Run()
    testDB.Close()
    os.Exit(code)
}
```

Use com moderação — `TestMain` esconde dependências. Prefira `t.Cleanup` por teste.

---

## Mocks e fakes

### Fake manual (preferido em Go)

```go
type fakeUserRepo struct {
    users        map[string]*user.User
    findByIDErr  error
    findByIDCalls int
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id string) (*user.User, error) {
    f.findByIDCalls++
    if f.findByIDErr != nil {
        return nil, f.findByIDErr
    }
    u, ok := f.users[id]
    if !ok {
        return nil, user.ErrNotFound
    }
    return u, nil
}

func (f *fakeUserRepo) Save(ctx context.Context, u *user.User) (*user.User, error) {
    if f.users == nil {
        f.users = map[string]*user.User{}
    }
    f.users[u.ID] = u
    return u, nil
}
```

Fakes manuais são idiomáticos em Go — explícitos, sem reflection, fáceis de debugar.

### gomock (geração via mockgen)

```bash
go install go.uber.org/mock/mockgen@latest
mockgen -source=internal/user/repository.go -destination=internal/user/mocks/repository_mock.go -package=mocks
```

```go
//go:generate mockgen -source=repository.go -destination=mocks/repository_mock.go -package=mocks

func TestCreateUser(t *testing.T) {
    ctrl := gomock.NewController(t)
    repo := mocks.NewMockRepository(ctrl)

    repo.EXPECT().
        FindByEmail(gomock.Any(), "john@test.com").
        Return(nil, user.ErrNotFound)
    repo.EXPECT().
        Save(gomock.Any(), gomock.Any()).
        Return(makeUser(t), nil)

    sut := user.NewService(repo)
    _, err := sut.Create(context.Background(), in)
    require.NoError(t, err)
}
```

Use gomock para interfaces grandes ou quando assertions de chamada são valiosas.

### mockery (alternativa)

```bash
go install github.com/vektra/mockery/v2@latest
mockery --name=Repository --dir=internal/user --output=internal/user/mocks
```

API similar a `testify/mock`:

```go
repo := mocks.NewRepository(t)
repo.On("FindByID", mock.Anything, "u1").Return(makeUser(t), nil)
```

`mockery` integra com `testify`; `gomock` tem API própria. Escolha uma e mantenha.

### Quando usar mock vs fake

| | Fake manual | gomock/mockery |
|---|---|---|
| Boilerplate | médio (escreve uma vez) | baixo (gerado) |
| Verificar chamadas | manual (contadores) | EXPECT() builtin |
| Comportamento complexo | ✅ natural | ⚠️ verboso |
| Refactor da interface | precisa atualizar | regenera |
| Stack trace de falha | claro | aceitável |

Para interfaces pequenas e estáveis, fake manual ganha. Para interfaces grandes ou que mudam muito, mock gerado vence.

---

## Testes de use case / service

```go
func TestCreateUser(t *testing.T) {
    tests := []struct {
        name      string
        setupRepo func(*fakeUserRepo)
        input     createuser.Input
        wantErr   error
    }{
        {
            name: "creates with valid data",
            setupRepo: func(r *fakeUserRepo) {
                r.users = map[string]*user.User{}
            },
            input:   createuser.Input{Name: "John", Email: "john@test.com"},
            wantErr: nil,
        },
        {
            name: "fails when email exists",
            setupRepo: func(r *fakeUserRepo) {
                r.users = map[string]*user.User{
                    "u1": {Email: "john@test.com"},
                }
            },
            input:   createuser.Input{Name: "John", Email: "john@test.com"},
            wantErr: createuser.ErrEmailExists,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &fakeUserRepo{}
            if tt.setupRepo != nil {
                tt.setupRepo(repo)
            }
            sut := createuser.New(repo, &fakeHasher{}, fakeClock{})

            _, err := sut.Execute(context.Background(), tt.input)
            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
        })
    }
}
```

---

## Testes de handler HTTP (httptest)

```go
func TestUserHandler_Create(t *testing.T) {
    uc := &fakeCreateUserUseCase{
        result: &createuser.Output{ID: "u1", Name: "John"},
    }
    h := userhttp.NewHandler(uc, slog.Default())

    body := strings.NewReader(`{"name":"John","email":"john@test.com"}`)
    req := httptest.NewRequest(http.MethodPost, "/users", body)
    rec := httptest.NewRecorder()

    h.Create(rec, req)

    require.Equal(t, http.StatusCreated, rec.Code)

    var resp userhttp.UserResponse
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
    require.Equal(t, "u1", resp.ID)
}
```

`httptest.NewRequest` cria request sem subir servidor. `httptest.NewRecorder` captura a response.

### Servidor de teste

```go
srv := httptest.NewServer(handler)
defer srv.Close()

resp, err := http.Get(srv.URL + "/users/u1")
```

Use quando precisa testar com client HTTP real ou middleware end-to-end.

### Validação de erros

```go
func TestUserHandler_Create_ValidationError(t *testing.T) {
    h := userhttp.NewHandler(&fakeCreateUserUseCase{}, slog.Default())

    req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
    rec := httptest.NewRecorder()

    h.Create(rec, req)

    require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
    require.Contains(t, rec.Body.String(), "validation_failed")
}
```

---

## Testes de repositório (testcontainers-go)

### Setup com Postgres real

```go
//go:build integration

package postgres_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupDB(t *testing.T) *sqlx.DB {
    t.Helper()
    ctx := context.Background()

    // testcontainers-go >= v0.32: a imagem é argumento posicional de Run;
    // RunContainer + WithImage estão deprecated (modules/postgres/postgres.go:139).
    pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
        tcpostgres.WithDatabase("test"),
        tcpostgres.WithUsername("test"),
        tcpostgres.WithPassword("test"),
        tcpostgres.BasicWaitStrategies(),
    )
    testcontainers.CleanupContainer(t, pg) // registra o Terminate mesmo quando err != nil
    require.NoError(t, err)

    dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    db, err := sqlx.Connect("postgres", dsn)
    require.NoError(t, err)
    t.Cleanup(func() { _ = db.Close() })

    runMigrations(t, db)
    return db
}
```

### Teste do repositório

```go
func TestUserRepository_FindByEmail(t *testing.T) {
    db := setupDB(t)
    sut := postgres.NewUserRepository(db)

    err := sut.Save(context.Background(), &user.User{
        ID: "u1", Email: "test@example.com", Name: "Test",
    })
    require.NoError(t, err)

    found, err := sut.FindByEmail(context.Background(), "test@example.com")
    require.NoError(t, err)
    require.Equal(t, "u1", found.ID)
}

func TestUserRepository_NotFound(t *testing.T) {
    db := setupDB(t)
    sut := postgres.NewUserRepository(db)

    _, err := sut.FindByEmail(context.Background(), "missing@x.com")
    require.ErrorIs(t, err, user.ErrNotFound)
}
```

`testcontainers-go` sobe um container Postgres por teste (ou compartilhado via `TestMain`). Cobre detalhes que mock de DB perde (constraints, índices, tipos).

### Alternativa: dockertest

`ory/dockertest` é mais leve. testcontainers-go é mais ergonômico e cross-driver.

### sqlmock (último recurso)

`DATA-DOG/go-sqlmock` simula `database/sql`. Use só quando container não é possível — cobre menos casos reais.

---

## Testes dependentes de tempo

### Clock injetável

```go
type Clock interface {
    Now() time.Time
}

type SystemClock struct{}
func (SystemClock) Now() time.Time { return time.Now() }

type fakeClock struct{ now time.Time }
func (c fakeClock) Now() time.Time { return c.now }
```

```go
clock := fakeClock{now: time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)}
sut := token.NewService(clock)
out := sut.CreateToken()
require.Equal(t, time.Date(2024, 7, 15, 10, 0, 0, 0, time.UTC), out.ExpiresAt)
```

Não use `time.Now()` direto em código testável — injete clock.

### synctest (Go 1.24+, experimental)

```go
import "testing/synctest"

func TestRetry(t *testing.T) {
    synctest.Run(func() {
        // tempo virtual — time.Sleep avança instantaneamente
        retry(3, time.Second, fn)
    })
}
```

Útil para testar lógica de timeouts sem `time.Sleep` real.

---

## Concurrent testing

### Race detector

```bash
go test -race ./...
```

Sempre rodar em CI. Detecta data races em runtime.

### Goroutines em testes

```go
func TestWorker_concurrent(t *testing.T) {
    sut := worker.New()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            sut.Process(i)
        }(i)
    }
    wg.Wait()

    require.Equal(t, 100, sut.Count())
}
```

### Goleak (vazamento de goroutine)

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

Falha se goroutines ficaram vivas após o teste — protege contra goroutine leaks.

---

## Factories (test helpers)

### Padrão make + opts

```go
type UserOpt func(*user.User)

func WithEmail(e string) UserOpt { return func(u *user.User) { u.Email = e } }
func WithRole(r user.Role) UserOpt { return func(u *user.User) { u.Role = r } }

func makeUser(t *testing.T, opts ...UserOpt) *user.User {
    t.Helper()
    u := &user.User{
        ID:        "u1",
        Email:     "john@test.com",
        Name:      "John Doe",
        Role:      user.RoleUser,
        CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
    }
    for _, opt := range opts {
        opt(u)
    }
    return u
}

// uso
u := makeUser(t, WithEmail("admin@x.com"), WithRole(user.RoleAdmin))
```

Functional options dão flexibilidade sem `Partial<T>`.

### Builder alternativo

```go
type UserBuilder struct{ user user.User }

func NewUserBuilder() *UserBuilder { return &UserBuilder{user: user.User{ID: "u1"}} }
func (b *UserBuilder) WithEmail(e string) *UserBuilder { b.user.Email = e; return b }
func (b *UserBuilder) Build() *user.User { return &b.user }
```

Mais verboso; útil quando há muitas combinações.

---

## Async / job testing

```go
func TestSendEmailWorker(t *testing.T) {
    mailer := &fakeMailer{}
    sut := workers.NewSendEmail(mailer)

    payload, _ := json.Marshal(workers.EmailSendJob{
        To: "user@test.com", Subject: "Hello",
    })
    task := asynq.NewTask(workers.TaskEmailSend, payload)

    err := sut.Handle(context.Background(), task)

    require.NoError(t, err)
    require.Len(t, mailer.sent, 1)
    require.Equal(t, "user@test.com", mailer.sent[0].To)
}

func TestSendEmailWorker_invalidPayload(t *testing.T) {
    sut := workers.NewSendEmail(&fakeMailer{})
    task := asynq.NewTask(workers.TaskEmailSend, []byte(`{"to":""}`))
    err := sut.Handle(context.Background(), task)
    require.ErrorIs(t, err, workers.ErrInvalidPayload)
}
```

---

## Coverage

```bash
go test -coverprofile=cov.out ./...
go tool cover -func=cov.out          # por função
go tool cover -html=cov.out          # HTML interativo
go test -coverpkg=./... -coverprofile=cov.out ./...  # cross-package
```

Sem `-coverpkg`, só conta cobertura do pacote sob teste. `-coverpkg=./...` conta o impacto global.

---

## Nomenclatura sugerida

| Tipo | Função | Exemplo |
|------|--------|---------|
| Use case | `Test[UseCase]_[scenario]` | `TestCreateUser_validData` |
| Handler | `Test[Handler]_[Action]` | `TestUserHandler_Create` |
| Repositório | `Test[Repo]_[Method]` | `TestUserRepository_FindByEmail` |
| Worker | `Test[Worker]_[scenario]` | `TestSendEmailWorker_invalidPayload` |
| Util | `Test[Func]` | `TestParseEmail` |

Subtests com nomes descritivos minúsculos (`t.Run("empty input", ...)`).

---

## Checklist

- [ ] `go test ./...` passa sem erros.
- [ ] `go test -race ./...` passa em CI.
- [ ] Cobertura medida e acompanhada (`go tool cover`).
- [ ] Testes table-driven com `t.Run` para subtests.
- [ ] `t.Helper()` em factories e helpers.
- [ ] `t.Cleanup` para teardown (não defer dentro do teste).
- [ ] Fakes manuais para interfaces pequenas; mocks gerados para grandes.
- [ ] `httptest` para handlers HTTP.
- [ ] `testcontainers-go` para repositórios de integração (build tag `integration`).
- [ ] Clock injetável para código dependente de tempo.
- [ ] Sem chamadas `time.Sleep` em testes (use clock fake / synctest).
- [ ] `goleak` ou inspeção manual contra goroutine leaks.
- [ ] Casos felizes E erros esperados cobertos.
