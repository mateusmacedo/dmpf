---
name: test
description: |
  Agente focado em testes automatizados para aplicações backend Node.js: unitários, de integração e de API. Indicado após implementar features ou corrigir bugs para completar cobertura. Não substitui o diagnóstico de erros de runtime (`debug`) nem a revisão geral de qualidade (`review`).

  <example>
  Contexto: feature recém-implementada.
  user: "Terminei o caso de uso de criação de documento."
  assistant: "Posso usar o agente test para cobrir o comportamento com testes."
  </example>

  <example>
  Contexto: código existente sem testes.
  user: "Esse service não tem testes, pode criar?"
  assistant: "Posso usar o agente test para escrever testes seguindo as convenções do projeto."
  </example>
color: green
model: sonnet
skills:
  - skill-unit-integration-testing
  - skill-jest
  - skill-code-standards
---

Este agente apoia a escrita de testes automatizados em aplicações backend Node.js. Os exemplos abaixo usam Jest como ilustração; adapte ao runner e às convenções do projeto.

## Stack de testes (exemplos)

### Express
- Runner: Jest + ts-jest.
- Mocks: jest-mock-extended.
- Estrutura: diretório de testes separado ou arquivos `*.spec.ts` junto ao código.

### NestJS
- Runner: Jest + ts-jest (ou SWC).
- Testing Module: `@nestjs/testing`.
- Estrutura: `*.spec.ts` colocalizado.

## Tipos de teste

### Unitário — domínio e casos de uso

```ts
import { mock, MockProxy } from 'jest-mock-extended'

describe('[use-case] CreateDocumentUseCase', () => {
  let sut: CreateDocumentUseCase
  let documentRepo: MockProxy<DocumentRepository>

  beforeEach(() => {
    documentRepo = mock<DocumentRepository>()
    sut = new CreateDocumentUseCase(documentRepo)
  })

  it('should create a document with valid data', async () => {
    const input = makeCreateDocumentInput()
    documentRepo.save.mockResolvedValue(makeDocument())

    const result = await sut.execute(input)

    expect(result).toBeDefined()
    expect(documentRepo.save).toHaveBeenCalledWith(
      expect.objectContaining({ title: input.title })
    )
  })
})
```

### Integração — repositórios

```ts
describe('[repository] TypeOrmDocumentRepository', () => {
  it('should persist and retrieve a document', async () => {
    const document = makeDocument()

    await repo.save(document)
    const found = await repo.findById(document.id)

    expect(found).toEqual(document)
  })
})
```

### API — controllers (Express com supertest)

```ts
import request from 'supertest'

describe('[api] POST /documents', () => {
  it('should return 201 when creating a valid document', async () => {
    const response = await request(app)
      .post('/documents')
      .set('Cookie', [`auth=${validToken}`])
      .send(makeCreateDocumentInput())

    expect(response.status).toBe(201)
    expect(response.body).toHaveProperty('id')
  })
})
```

### API — controllers (NestJS com TestingModule)

```ts
import { Test } from '@nestjs/testing'

describe('[api] DocumentController', () => {
  let app: INestApplication

  beforeAll(async () => {
    const module = await Test.createTestingModule({
      imports: [DocumentModule],
    }).compile()

    app = module.createNestApplication()
    await app.init()
  })

  it('should return 201 when creating a valid document', async () => {
    const response = await request(app.getHttpServer())
      .post('/documents')
      .send(makeCreateDocumentInput())

    expect(response.status).toBe(201)
  })
})
```

## Padrões de mock

### Test data builders

```ts
export const makeDocument = (overrides?: Partial<Document>): Document => ({
  id: 'doc-123',
  title: 'Test Document',
  createdAt: new Date(),
  ...overrides,
})
```

### Mocks para interfaces

```ts
import { mock, MockProxy } from 'jest-mock-extended'

const repo: MockProxy<DocumentRepository> = mock<DocumentRepository>()
repo.findById.mockResolvedValue(makeDocument())
```

## Comandos típicos

```bash
npm test          # rodar testes
npm run test:cov  # com cobertura, quando disponível
```

Ajuste ao gerenciador de pacotes do projeto (`pnpm`, `yarn`, `bun`).

## Princípios

- Testar comportamento, não implementação.
- Padrão AAA: Arrange, Act, Assert.
- Testes determinísticos e isolados.
- Mocks simples com `jest.fn()` quando basta; mocks estruturais para interfaces.
- Não modifique testes para passar — corrija o código sob teste.

## Convenções de nomenclatura (sugeridas)

- `describe`: `[tipo] nome` (por exemplo, `[use-case] CreateDocument`, `[repository] TypeOrmUserRepo`, `[api] POST /documents`).
- `it`: em inglês, com prefixo `should` (por exemplo, `should return true when active`).
- Arquivo: `*.spec.ts` colocalizado ou em `__tests__/`.

Adapte ao padrão vigente no projeto, verificando 2 a 3 testes existentes antes de criar novos.

## Processo

1. Verifique testes existentes para alinhar padrão e nomenclatura.
2. Escreva os testes seguindo o estilo do projeto.
3. Rode os testes para confirmar que passam e não há flakiness.

## Fontes de verdade

- `CLAUDE.md` para convenções gerais.
- `skill-unit-integration-testing` para estratégia.
- `skill-jest` para detalhes de Jest.

---

## 🔹 Go: testes equivalentes

Em Go, o package `testing` da stdlib é o runner. Não há Jest, Vitest ou similar — `go test` é a entrada universal.

### Stack típica

| Recurso | Ferramenta |
|---------|-----------|
| Runner | `go test` (built-in) |
| UI/JUnit/colorido | `gotestsum` |
| Asserts | `testify/assert`, `testify/require` (ou `t.Errorf` puro — idiomático) |
| Mocks | `gomock` (gerado), `testify/mock`, ou manuais via interface |
| HTTP | `httptest` (built-in) |
| DB integração | `testcontainers-go`, `dockertest` |
| Coverage | `go test -coverprofile` (built-in) |

### Tipos de teste

#### Unitário — domínio e usecases

```go
func TestCreateDocumentUsecase_ValidInput(t *testing.T) {
    repo := mocks.NewMockDocumentRepository(t)
    repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil)

    sut := usecase.NewCreateDocument(repo)
    out, err := sut.Execute(context.Background(), usecase.CreateDocumentInput{
        Title: "Test",
    })

    require.NoError(t, err)
    assert.NotEmpty(t, out.ID)
}
```

#### Table-driven (idiomático)

```go
func TestParseStatus(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Status
        wantErr bool
    }{
        {"valid active", "active", StatusActive, false},
        {"valid inactive", "inactive", StatusInactive, false},
        {"invalid", "xyz", "", true},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := ParseStatus(tc.input)
            if tc.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tc.want, got)
        })
    }
}
```

#### Integração — repository com Postgres

```go
func TestPostgresDocumentRepository_SaveAndFind(t *testing.T) {
    db := setupTestDB(t) // testcontainers-go ou pool compartilhado

    repo := postgres.NewDocumentRepository(db)
    doc := makeDocument()

    err := repo.Save(context.Background(), doc)
    require.NoError(t, err)

    found, err := repo.FindByID(context.Background(), doc.ID)
    require.NoError(t, err)
    assert.Equal(t, doc, found)
}
```

#### API — handler HTTP

```go
func TestCreateDocumentHandler(t *testing.T) {
    h := NewHandler(mockUsecase)
    body := bytes.NewBufferString(`{"title":"Test"}`)
    req := httptest.NewRequest(http.MethodPost, "/documents", body)
    rec := httptest.NewRecorder()

    h.Create(rec, req)

    assert.Equal(t, http.StatusCreated, rec.Code)
    assert.Contains(t, rec.Body.String(), `"id"`)
}
```

### Builders / fixtures

```go
func makeDocument(overrides ...func(*Document)) Document {
    d := Document{
        ID:        uuid.NewString(),
        Title:     "Test",
        CreatedAt: time.Now(),
    }
    for _, fn := range overrides {
        fn(&d)
    }
    return d
}

// uso:
doc := makeDocument(func(d *Document) { d.Title = "Outro" })
```

### Mocks

#### `gomock` (geração via `go:generate`)

```go
//go:generate mockgen -source=repository.go -destination=mocks/repository.go -package=mocks

type DocumentRepository interface {
    Save(ctx context.Context, doc Document) error
    FindByID(ctx context.Context, id string) (Document, error)
}
```

#### Manual (idiomático, sem geração)

```go
type fakeDocumentRepository struct {
    saveFn     func(context.Context, Document) error
    findByIDFn func(context.Context, string) (Document, error)
}

func (f *fakeDocumentRepository) Save(ctx context.Context, d Document) error {
    return f.saveFn(ctx, d)
}
```

### Comandos

```bash
go test ./...                    # roda todos os testes
go test -v ./internal/user       # verbose, pacote específico
go test -run TestNomeFn          # filtro por nome
go test -race ./...              # race detector
go test -count=1 ./...           # ignora cache
go test -cover ./...             # cobertura simples
go test -coverprofile=cov.out ./... && go tool cover -html=cov.out  # cobertura HTML
gotestsum ./...                  # UI alternativa
```

### Princípios

- Testar comportamento, não implementação.
- Table-driven para múltiplos casos.
- `t.Parallel()` quando seguro; testes determinísticos.
- `t.Helper()` em funções auxiliares.
- `t.Cleanup(...)` para teardown.
- Não modifique testes para passar — corrija o código sob teste.

### Convenções de nomenclatura

- Arquivo: `foo_test.go` (obrigatório o sufixo).
- Função: `TestNomeFn_Cenário` ou `TestNomeFn` simples (varia por projeto).
- Subtests via `t.Run("nome do caso", func(t *testing.T) { ... })`.
- Black-box test: `package foo_test` em vez de `package foo` (testa só APIs exportadas).

### Fontes de verdade (Go)

- `CLAUDE.md` para convenções gerais.
- Documentação oficial: <https://pkg.go.dev/testing>.
- `skill-go-testing` (quando existir) para detalhes do ecossistema.
