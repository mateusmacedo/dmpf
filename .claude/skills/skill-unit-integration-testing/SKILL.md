---
name: skill-unit-integration-testing
description: |
  Orientação para testes unitários e de integração: estrutura, nomenclatura,
  escolha entre tipos, isolamento e boas práticas. Framework-agnóstica.
  Referências específicas de Jest/RTL ficam em skill-jest.
model: sonnet
---

# Testes Unitários e de Integração

## Objetivo

Conceitos gerais de testes unitários e de integração. Aborda estrutura, nomenclatura, boas práticas e critérios para escolher entre tipos de teste. Convenções concretas (nomes de describes, estrutura de pastas) são defaults sugeridos; cada projeto pode adaptar.

## Quando aplicar

- Ao definir estratégia entre testes unitários e de integração.
- Ao escrever testes com isolamento adequado.
- Ao decidir limites de uso de mocks e fakes.

## Tipos de teste

| Tipo | Escopo | Velocidade | Isolamento |
|------|--------|------------|------------|
| Unitário | Função/classe individual | Alta | Total (mocks/fakes) |
| Integração | Múltiplos módulos | Média | Parcial |

### Critérios para escolha

| Cenário | Tipo sugerido |
|---------|---------------|
| Função pura com lógica de negócio | Unitário |
| Transformação de dados | Unitário |
| Validação de inputs | Unitário |
| Service com dependências mockadas | Unitário |
| Use case isolado | Unitário |
| Controller com service real | Integração |
| Use case com repositório real | Integração |
| Fluxo entre múltiplos services | Integração |
| Repository com banco de dados | Integração |

## Nomenclatura

Convenção sugerida; projetos podem adaptar desde que a padronização seja consistente.

### Formato `describe`

```ts
describe('[categoria] nome', () => {})
describe('[categoria:subcategoria] nome', () => {})
```

Categorias comuns:

| Categoria | Uso |
|-----------|-----|
| `[utils]` | Funções utilitárias |
| `[service]` | Services de aplicação |
| `[use-case]` | Use cases |
| `[controller]` | Controllers |
| `[repository]` | Repositórios |
| `[domain]` | Regras de domínio |
| `[data:formatters]` | Formatadores de dados |
| `[infra]` | Integrações externas |

### Formato `it`/`test`

Descrições claras de comportamento. Prefixo `should` é um default comum:

```ts
it('should return true when condition is met', () => {})
it('should throw error when input is invalid', () => {})
it('should call callback with correct arguments', () => {})
```

O idioma das descrições é parâmetro do projeto.

## Estrutura AAA (Arrange, Act, Assert)

```ts
it('should calculate total with discount', () => {
  // Arrange
  const items = [{ price: 100 }, { price: 50 }]
  const discount = 0.1

  // Act
  const result = calculateTotal(items, discount)

  // Assert
  expect(result).toBe(135)
})
```

## Organização de arquivos

Uma opção comum é colocalizar testes em `__tests__/` dentro de cada módulo:

```
src/
├── feature/
│   ├── index.ts
│   ├── types.ts
│   └── __tests__/
│       └── test.ts
```

Alternativas igualmente válidas: diretório `tests/` paralelo, sufixo `.test.ts` ao lado do arquivo testado. O padrão deve ser consistente dentro do projeto.

### Helpers de teste (exemplo ilustrativo)

```
src/tests/
├── factories/         # makeUser(), makeDocument()
├── mocks/             # mock factories para repos/services
└── utils/
```

Adapte conforme necessidade.

## Boas práticas

### Testar comportamento observável

```ts
// Evitar — testa detalhes de implementação
it('should call repository.save once', () => {
  expect(repository.save).toHaveBeenCalledTimes(1)
})

// Preferir — testa resultado observável
it('should persist user and return created entity', async () => {
  const result = await createUserUseCase.execute(validInput)
  expect(result.id).toBeDefined()
  expect(result.email).toBe(validInput.email)
})
```

### Um conceito por teste

```ts
// Evitar — múltiplos conceitos
it('should validate, transform and save data', () => {})

// Preferir — conceitos separados
it('should reject invalid email format', () => {})
it('should normalize email to lowercase', () => {})
it('should persist valid user data', () => {})
```

### Determinismo

```ts
// Evitar — depende de data atual
it('should show current date', () => {
  expect(result).toContain(new Date().toISOString())
})

// Preferir — data controlada
it('should format date correctly', () => {
  expect(formatDate(new Date('2024-01-15'))).toBe('15/01/2024')
})
```

Usar utilitários de time do framework (ex.: fake timers) quando a lógica depende do relógio.

## Anti-patterns

```ts
// Acessar membros privados
expect(service['privateMethod']).toHaveBeenCalled()

// Esperas fixas em tempo
await new Promise(r => setTimeout(r, 1000))

// Silenciar erros
try { doSomething() } catch {}

// Uso amplo de any em mocks
const mock: any = {}
```

Alternativas:

```ts
// Resultado observável
const result = await service.execute(input)
expect(result).toEqual(expectedOutput)

// Async explícito
await expect(service.execute(badInput)).rejects.toThrow()

// Verificar tipo de erro
expect(() => validateInput(invalid)).toThrow(ValidationError)

// Mocks tipados (ver skill do framework em uso)
```

## Checklist

- [ ] Tipo de teste adequado ao escopo (unitário vs integração).
- [ ] Setup/teardown isolado e previsível.
- [ ] Asserts sobre resultado observável, não implementação.
- [ ] Mocks tipados e limitados ao necessário.
- [ ] Critério de cobertura definido pelo projeto e verificado.

---

## 🔹 Go: testes unitários e de integração

Conceitos (escopo, AAA, isolamento) são universais. Em Go o framework de testes é built-in (`testing`), e o idioma dominante é table-driven.

### Tipos de teste em Go

| Tipo | Convenção | Exemplo |
|------|-----------|---------|
| Unitário | `_test.go` no mesmo pacote | `service_test.go` |
| Integração | Build tag (`//go:build integration`) | `repository_integration_test.go` |
| Black-box | Sufixo `_test` no package | `package user_test` |
| Benchmark | Função `BenchmarkXxx(b *testing.B)` | `BenchmarkParse` |
| Example | Função `ExampleXxx()` | gera doc + valida output |

### Critérios para escolha

| Cenário | Tipo sugerido |
|---------|---------------|
| Função pura | Unitário |
| Use case com mocks dos repositórios | Unitário |
| Repositório com Postgres real (testcontainers) | Integração |
| Handler com router real, dependências fake | Unitário/integração leve |
| Fluxo end-to-end com banco e fila reais | Integração |

### Nomenclatura

#### Funções de teste

```go
func TestCalculateTotal(t *testing.T)              // ok
func TestCalculateTotal_WithDiscount(t *testing.T) // detalhe via _
func TestUserService_Create_RejectsInvalidEmail(t *testing.T)
```

`Test<Type>_<Method>_<Behavior>` é convenção comum, mas em table-driven o `t.Run("nome do caso", ...)` carrega a granularidade.

#### Subtests com `t.Run`

```go
func TestCalculateTotal(t *testing.T) {
    t.Run("aplica desconto percentual", func(t *testing.T) { /* ... */ })
    t.Run("rejeita itens inválidos", func(t *testing.T) { /* ... */ })
}
```

### Estrutura AAA em Go

```go
func TestCalculateTotal(t *testing.T) {
    // Arrange
    items := []Item{{Price: 100}, {Price: 50}}
    discount := 0.10

    // Act
    got := CalculateTotal(items, discount)

    // Assert
    if want := int64(135); got != want {
        t.Errorf("CalculateTotal = %d; want %d", got, want)
    }
}
```

Com `testify/assert`:

```go
import "github.com/stretchr/testify/assert"

func TestCalculateTotal(t *testing.T) {
    got := CalculateTotal([]Item{{Price: 100}, {Price: 50}}, 0.10)
    assert.Equal(t, int64(135), got)
}
```

### Table-driven (idiomático)

```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "user@example.com", false},
        {"missing @", "user.example.com", true},
        {"empty", "", true},
        {"too long", strings.Repeat("a", 256) + "@x.com", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail(%q) error = %v; wantErr %v",
                    tt.input, err, tt.wantErr)
            }
        })
    }
}
```

Cada caso vira subteste; `go test -run TestValidateEmail/empty` roda só um.

### Organização de arquivos (Go)

```
internal/
└── user/
    ├── user.go
    ├── user_test.go            # white-box (mesmo pacote)
    ├── service.go
    ├── service_test.go
    └── service_blackbox_test.go # opcional: package user_test
```

Não há diretório `__tests__/` — testes ficam ao lado do código (convenção da toolchain).

#### Test helpers

```go
// internal/testutil/factory.go
package testutil

func MakeUser(t *testing.T, opts ...func(*user.User)) *user.User {
    t.Helper()
    u := &user.User{ID: "u_1", Email: "x@y.com"}
    for _, opt := range opts {
        opt(u)
    }
    return u
}
```

`t.Helper()` faz o erro apontar para o teste, não o helper.

### Setup / teardown

#### Por subteste

```go
func TestUserRepo(t *testing.T) {
    db := setupDB(t)            // helper
    t.Cleanup(func() { db.Close() })

    t.Run("save", func(t *testing.T) { /* ... */ })
}
```

`t.Cleanup` substitui `defer` — roda mesmo em falha e respeita o subteste.

#### TestMain (setup global)

```go
func TestMain(m *testing.M) {
    db = setupSharedDB()
    code := m.Run()
    db.Close()
    os.Exit(code)
}
```

### Mocks

#### Interface manual + struct fake

```go
type fakeUserRepo struct {
    findByID func(ctx context.Context, id string) (*User, error)
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id string) (*User, error) {
    return f.findByID(ctx, id)
}

func TestUseCase(t *testing.T) {
    repo := &fakeUserRepo{
        findByID: func(_ context.Context, _ string) (*User, error) {
            return &User{ID: "u_1"}, nil
        },
    }
    uc := NewUseCase(repo)
    // ...
}
```

#### `gomock` ou `mockery`

Para projetos grandes com muitas interfaces, geração automática reduz boilerplate.

```bash
mockery --name UserRepository --output ./mocks
```

### Testes de integração com banco

#### testcontainers-go

```go
func TestUserRepo_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("integração desabilitada com -short")
    }

    ctx := context.Background()
    // testcontainers-go >= v0.32: a imagem é argumento posicional de Run;
    // RunContainer está deprecated (modules/postgres/postgres.go:139).
    // Alias tcpostgres: o package do projeto também se chama postgres.
    pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
        tcpostgres.WithDatabase("test"),
        tcpostgres.WithUsername("test"),
        tcpostgres.WithPassword("test"),
        tcpostgres.BasicWaitStrategies(),
    )
    testcontainers.CleanupContainer(t, pg) // no-op com pg == nil; Terminate mesmo se Run falhar
    if err != nil {
        t.Fatal(err)
    }

    dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
    db, _ := sql.Open("postgres", dsn)
    runMigrations(t, db)

    repo := postgres.NewUserRepository(db)
    // ...
}
```

#### Build tag para separar

```go
//go:build integration

package postgres

func TestUserRepo_Integration(t *testing.T) { /* ... */ }
```

Rodar: `go test -tags=integration ./...`

### Testes HTTP

```go
func TestCreateUser(t *testing.T) {
    h := NewHandler(fakeUseCase)
    req := httptest.NewRequest(http.MethodPost, "/users",
        strings.NewReader(`{"email":"x@y.com","name":"X"}`))
    rec := httptest.NewRecorder()

    h.ServeHTTP(rec, req)

    if rec.Code != http.StatusCreated {
        t.Errorf("status = %d; want 201", rec.Code)
    }
}
```

### Boas práticas (Go)

#### Testar comportamento, não implementação

```go
// Evitar — testa chamada interna
mockRepo.AssertCalled(t, "Save", mock.Anything)

// Preferir — testa resultado observável
got, err := uc.Execute(ctx, in)
assert.NoError(t, err)
assert.Equal(t, "u_1", got.ID)
```

#### Determinismo: clock injetado

```go
type Clock interface { Now() time.Time }

type fakeClock struct{ now time.Time }
func (f fakeClock) Now() time.Time { return f.now }
```

Não usar `time.Now()` direto em código testável.

#### Race detector

```bash
go test ./... -race
```

Roda em CI sempre. Detecta data races no nível de instrumentação.

### Anti-patterns (Go)

```go
// time.Sleep para esperar goroutine
time.Sleep(100 * time.Millisecond)  // flaky — usar channel/sync

// Acessar campo privado via reflection
reflect.ValueOf(svc).Elem().FieldByName("private")

// Globais mutáveis em testes (poluem outros tests)
var globalState = ...

// Erro silenciado
_, _ = svc.Execute(ctx, in)

// `interface{}` em mock — evitar; declare interface mínima
```

### Cobertura

```bash
go test ./... -cover                # cobertura por pacote
go test ./... -coverprofile=c.out   # detalhe
go tool cover -html=c.out           # navegador
```

Foco em pacotes de domínio/use case; infra cobre via integração.

### Checklist (Go)

- [ ] Tipo de teste adequado (unitário vs integração via build tag).
- [ ] Table-driven onde houver múltiplos casos.
- [ ] `t.Helper()` em helpers de teste.
- [ ] `t.Cleanup` em vez de `defer` para teardown.
- [ ] Mocks via interface mínima no consumidor.
- [ ] `go test -race` passa.
- [ ] Cobertura medida e revisada.
- [ ] Sem `time.Sleep` para sincronização.
- [ ] `testing.Short()` para pular testes pesados em modo rápido.
