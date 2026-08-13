---
paths:
  - "**/*.spec.ts"
  - "**/*_test.go"
  - "**/__tests__/**"
  - "**/tests/**"
  - "**/jest.config.*"
  - "jest.preset.js"
---

# Convenções de teste

## Stack (exemplos)

- **Runner**: Jest é comum em projetos Node/TypeScript; alternativas incluem Vitest.
- **Mocks tipados**: jest-mock-extended é uma opção prática.
- **API HTTP**: supertest para exercitar controllers/rotas.
- **Banco in-memory**: pg-mem (ou equivalente) para testes de integração sem container.

## Categorias de teste

| Categoria | Alvo | Ferramentas típicas |
|-----------|------|---------------------|
| Unitário | Domínio, serviços, casos de uso | Runner + mocks |
| Integração | Repositórios e banco, módulos maiores | Runner + banco in-memory ou banco dedicado de teste |
| API | Controllers/rotas HTTP | Runner + supertest |

## Localização dos arquivos

Dois padrões são comuns:

- Diretório `tests/` separado, espelhando `src/` (por exemplo, `tests/<caminho>/<nome>.spec.ts`).
- Testes colocalizados ao lado do código (por exemplo, `src/<caminho>/<nome>.spec.ts` ou `__tests__/<nome>.test.ts`).

Siga o padrão vigente no repositório.

## Cobertura

- Funções puras e casos de uso costumam ter testes.
- Controllers podem ter testes de API para rotas críticas.
- Repositórios com queries não triviais costumam ganhar testes de integração.

## Nomenclatura

Verifique 2 a 3 testes existentes antes de criar novos. Um padrão comum:

- `describe`: `[tipo] nome` (por exemplo, `[usecase] CreateUser`, `[repository] PetitionRepo`, `[controller] AuthController`).
- `it`: em inglês, com prefixo `should` (por exemplo, `should return user when valid id`).

## Padrão AAA

Organize os testes no formato Arrange-Act-Assert:

```ts
it('should return user when valid id', () => {
  // Arrange
  const userId = 'valid-id'

  // Act
  const result = sut.execute({ userId })

  // Assert
  expect(result).toBeDefined()
})
```

## Regra dura

Não modifique testes apenas para passá-los — corrija o código sob teste.

---

## 🔹 Go: convenções de teste

Em Go, testes são parte do `testing` package built-in. Não há runner externo, mas há ferramentas auxiliares.

### Stack típica

| Recurso | Ferramentas comuns |
|---------|--------------------|
| Runner | `go test` (built-in), `gotestsum` (UI/JUnit) |
| Asserts | `testify/assert`, `testify/require` (ou nada — `t.Errorf` puro é idiomático) |
| Mocks | `gomock` (geração via `mockgen`), `testify/mock`, mocks manuais (comum) |
| API HTTP | `httptest` (built-in: `httptest.NewServer`, `httptest.NewRecorder`) |
| Banco | Container real via `testcontainers-go`, `dockertest`, ou DB de teste compartilhado |
| Snapshots | `cupaloy`, `goldie` |
| Property-based | `gopter`, `rapid` |

### Categorias de teste

| Categoria | Convenção |
|-----------|-----------|
| Unitário | `_test.go` no mesmo pacote (acesso a símbolos privados) |
| Black-box | `_test.go` em pacote `xxx_test` (só APIs exportadas) |
| Integração | Tag de build `// +build integration` ou `_integration_test.go` |
| Benchmark | `func BenchmarkXxx(b *testing.B)` |
| Example | `func ExampleFoo()` — vai pra godoc + roda em `go test` |

### Localização

Sempre colocalizada: `foo.go` + `foo_test.go` no mesmo diretório. Pasta `tests/` separada existe para integração end-to-end, raramente para unit.

### Cobertura

- `go test -coverprofile=coverage.out ./...` + `go tool cover -html=coverage.out`.
- Cobertura de pacote, não de arquivo. Funções públicas e use cases devem ter cobertura significativa.

### Nomenclatura

- Arquivo: `foo_test.go` (obrigatório o sufixo).
- Funções: `func TestNomeDoSubject_Cenário(t *testing.T)` é convenção usual (ex: `TestCreateUser_DuplicateEmail`).
- Subtests para variações: `t.Run("nome do caso", func(t *testing.T) { ... })`.

### Padrão Table-Driven (idiomático)

Em vez de AAA por teste, Go favorece tabelas de casos:

```go
func TestParseDate(t *testing.T) {
    cases := []struct {
        name    string
        input   string
        want    time.Time
        wantErr bool
    }{
        {"valid ISO", "2025-01-01", time.Date(...), false},
        {"empty", "", time.Time{}, true},
        {"invalid format", "01/01/2025", time.Time{}, true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, err := ParseDate(tc.input)
            if (err != nil) != tc.wantErr {
                t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
            }
            if !got.Equal(tc.want) {
                t.Errorf("got %v, want %v", got, tc.want)
            }
        })
    }
}
```

AAA ainda funciona dentro do `t.Run`, só não é o padrão visual default.

### Helpers e fixtures

- `t.Helper()` em funções auxiliares para que o report aponte o caller.
- `t.Cleanup(func)` para teardown — substitui defer em vários casos.
- Fixtures em `testdata/` (a toolchain ignora esse diretório por convenção).

### Race e parallelism

- `go test -race ./...` em CI é praticamente obrigatório.
- `t.Parallel()` em testes lentos; cuidado com estado compartilhado.

### Regra dura

Igual: não modifique testes para fazê-los passar. Em Go, testes flaky por concorrência são frequentes — investigue ao invés de marcar `t.Skip`.
