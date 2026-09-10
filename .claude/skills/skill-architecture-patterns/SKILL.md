---
name: skill-architecture-patterns
description: |
  Use esta skill quando o usuário pedir para "organizar projeto", "definir estrutura de pastas",
  "separar responsabilidades", "definir camadas", "modularizar", "onde colocar esse código"
  ou mencionar arquitetura, dependências entre camadas, limites de módulos e trade-offs.
  Cobre estrutura de pastas, separação de responsabilidades, dependências e modularização.
  Para evolução de arquitetura existente, ver `skill-evolutionary-architecture`.
model: opus
---

# Architecture Patterns (Backend)

## Objetivo

Consolidar padrões de arquitetura backend: estrutura de pastas, separação de responsabilidades, dependências entre camadas e trade-offs frequentes.

## Quando usar

- Ao definir estrutura de pastas, camadas e módulos de um serviço backend.
- Ao refatorar dependências e limites entre contextos.
- Ao avaliar trade-offs (DI, modularização, padrões de dados).

## Estrutura de projeto (exemplo)

```
src/
  domain/         # Entidades, VOs, contratos, erros, serviços de domínio
  application/    # Casos de uso, serviços, controllers, DTOs
  infra/          # Repositórios, clients externos, consumers de fila
  main/           # Composition root, rotas, middlewares, config
  shared/         # Código compartilhado entre módulos
```

Variações por framework são comuns. Em Express, `main/` costuma ter factories de DI manual e setup de rotas. Em NestJS, cada módulo colocaliza `domain + application + infra` e `main/` equivale ao `AppModule` + `main.ts`.

---

## Clean Architecture — direção das dependências

```
┌──────────────────────────────────┐
│            domain/               │  núcleo puro
│  Entidades, VOs, contratos       │
└──────────────────────────────────┘
                ↑
┌──────────────────────────────────┐
│          application/            │  importa de domain/
│  Casos de uso, DTOs, serviços    │
└──────────────────────────────────┘
                ↑
┌──────────────────────────────────┐
│            infra/                │  importa de domain/ e application/
│  Repositórios, clients, consumers│  (implementa contratos)
└──────────────────────────────────┘
                ↑
┌──────────────────────────────────┐
│             main/                │  wiring
│  Composition root, rotas, config │
└──────────────────────────────────┘
```

---

## Regras de dependência

| Camada | Pode importar de | Evita importar de |
| -------- | ------------------- | ------------------- |
| `domain/` | tipos próprios | `application/`, `infra/`, `main/`, libs externas |
| `application/` | `domain/` | `infra/`, `main/` |
| `infra/` | `domain/`, `application/` | `main/` |
| `main/` | tudo (wiring) | — |

Princípio de ouro: o domínio não importa de camadas externas; contratos vivem em `domain/` e são implementados em `infra/`.

---

## Padrões de injeção de dependência

### DI manual via factories

```typescript
// main/factories/makeCreateUser.ts
export const makeCreateUser = () => {
  const userRepo = new TypeOrmUserRepo(dataSource)
  const hasher = new BcryptHasher()
  return new CreateUserUseCase({ userRepo, hasher })
}
```

- Wiring explícito.
- Cada caso de uso tem sua factory.
- Rastreabilidade alta, custo de boilerplate maior.

### DI via container IoC (ex.: NestJS)

```typescript
@Injectable()
export class CreateUserUseCase {
  constructor(
    @Inject('UserRepository') private readonly userRepo: UserRepository,
    private readonly hasher: BcryptHasher,
  ) {}
}

@Module({
  providers: [
    CreateUserUseCase,
    { provide: 'UserRepository', useClass: TypeOrmUserRepo },
  ],
})
export class UserModule {}
```

- O container resolve dependências automaticamente.
- Menos boilerplate e mais indireção.
- Testável via módulos de teste do framework.

---

## Módulos e coesão

### Alta coesão — um módulo por bounded context

```
src/modules/user/
  domain/          # User entity, contrato UserRepository
  application/     # CreateUserUseCase, UpdateUserUseCase, DTOs
  infra/           # TypeOrmUserRepo, BcryptHasher
```

### Colocalização vs. compartilhamento

| Código | Onde colocar |
| -------- | ------------- |
| Usado em apenas um módulo | Dentro do módulo |
| Usado por dois ou mais módulos | `shared/` |
| Contrato de domínio | `domain/` do módulo dono |
| Utilitário genérico (logger, data) | `shared/` |

---

## Separação de responsabilidades

### Controller vs. caso de uso

```typescript
// Controller com regra de negócio embutida
class UserController {
  async create(req: Request, res: Response) {
    const exists = await this.repo.findByEmail(req.body.email)
    if (exists) throw new ConflictError('Email já existe')
    const hashed = await bcrypt.hash(req.body.password, 10)
    const user = await this.repo.save({ ...req.body, password: hashed })
    res.status(201).json(user)
  }
}

// Controller fino delegando para caso de uso
class UserController {
  async create(req: Request, res: Response) {
    const result = await this.createUser.execute(req.body)
    res.status(201).json(result)
  }
}
```

Controllers cuidam de HTTP; casos de uso cuidam de regras de negócio.

---

## Trade-offs comuns

### Monolito modular vs. microserviços

| Aspecto | Monolito modular | Microserviços |
| --------- | ----------------- | --------------- |
| Complexidade operacional | Baixa | Alta (deploy, rede, observabilidade) |
| Comunicação | Chamadas in-process | HTTP/gRPC/filas |
| Consistência | Transações ACID fáceis | Consistência eventual |
| Quando considerar | Default para muitos casos | Quando há necessidade real de escala independente |

### DI manual vs. container IoC

| Aspecto | Manual | Container |
| --------- | -------- | ----------- |
| Rastreabilidade | Explícita | Indireta via decorators |
| Boilerplate | Maior | Menor |
| Testabilidade | Boa em ambos | Boa em ambos |

### Controller "gordo" vs. controller fino + caso de uso

| Aspecto | Controller gordo | Controller fino + caso de uso |
| --------- | ----------------- | ------------------------------- |
| Simplicidade | Tudo em um lugar | Separação clara |
| Testabilidade | Requer HTTP para testar | Caso de uso testável isolado |
| Reúso | Lógica presa ao HTTP | Reutilizável em filas, CLI, etc. |

### ORM: Active Record vs. Data Mapper

| Aspecto | Active Record | Data Mapper |
| --------- | -------------- | ------------- |
| Acoplamento | Entidade conhece o banco | Entidade pura; repositório à parte |
| Testabilidade | Mais difícil mockar | Mais fácil mockar repositório |
| Clean Architecture | Tende a violar | Compatível |

---

## Decisões de arquitetura

### Checklist

1. Qual o problema? (descrever claramente)
2. Quais as opções? (duas ou três abordagens)
3. Quais os trade-offs? (prós e contras de cada)
4. Qual a recomendação? (escolha justificada)
5. Como reverter se der errado?

### ADR (Architecture Decision Record) — exemplo

```markdown
# ADR-001: Adotar Data Mapper em vez de Active Record no ORM

## Status
Aceito

## Contexto
Precisamos de uma estratégia de persistência que:
- Mantenha entidades de domínio sem dependências de ORM
- Permita trocar a implementação de repositório em testes
- Respeite a direção de dependência da Clean Architecture

## Decisão
Adotar o padrão Data Mapper com repositórios dedicados.

## Consequências
### Positivas
- Entidades de domínio sem decorators de ORM
- Repositórios fáceis de mockar em testes unitários
- Compatível com Clean Architecture

### Negativas
- Mais boilerplate (entidade ORM + entidade de domínio + mapper)
- Curva de aprendizado

## Alternativas consideradas
- Active Record: descartado por acoplar domínio ao ORM
- Prisma: descartado no contexto específico por decisões de equipe
```

---

## Evolução incremental

### Refatoração segura

1. Adicionar o novo sem remover o existente.
2. Migrar consumidores gradualmente.
3. Remover o antigo quando não houver mais uso.

```typescript
// Passo 1: criar novo serviço ao lado do existente
// services/NotificationServiceV2.ts

// Passo 2: migrar consumidores para V2 um a um

// Passo 3: após migração completa, remover V1
```

### Sinais de que um módulo pode ser dividido

- Muitos arquivos na camada `application/` (sinal relativo; depende da complexidade).
- Casos de uso de contextos diferentes convivendo no mesmo módulo.
- Dependências circulares entre módulos.

---

## Checklist

- [ ] Estrutura de pastas reflete responsabilidades reais.
- [ ] Regras de dependência entre camadas respeitadas.
- [ ] Domain sem dependência de camadas externas.
- [ ] Código colocalizado quando usado por apenas um módulo.
- [ ] Mudanças arquiteturais incrementais e reversíveis.
- [ ] Trade-offs e decisões documentados quando relevante (ADR).

---

## 🔹 Go: padrões equivalentes

### Estrutura de projeto Go

```
projeto/
├── cmd/
│   └── api/main.go        # composition root
├── internal/              # código não importável fora do módulo
│   ├── <ctx>/             # bounded context (user, billing, doc)
│   │   ├── domain.go
│   │   ├── usecase.go
│   │   ├── repository.go
│   │   ├── http.go
│   │   └── *_test.go
│   ├── infra/             # adapters: db pool, redis, kafka client
│   ├── shared/            # cross-cutting (httputil, logger)
│   └── config/
├── pkg/                   # APIs públicas (raro em apps internos)
├── go.mod
└── go.sum
```

### Direção de dependências (Go)

A regra é a mesma: domain core sem dependência externa. **Diferencial Go**: imports cíclicos são *erro de compilação*, não warning. Estrutura arquitetural ruim quebra build cedo.

| Pacote | Pode importar | Não importa |
| -------- | --------------- | ------------- |
| `internal/<ctx>/domain.go` (entities/contracts) | stdlib, libs puras (uuid, time) | qualquer outro pacote interno |
| `internal/<ctx>/usecase.go` | `domain` do mesmo contexto | `infra`, `cmd` |
| `internal/<ctx>/*` (infra) | `domain`, `usecase` (interfaces) | `cmd` |
| `cmd/<binário>/main.go` | tudo | — |

### Dependency injection

Go não tem container dominante. Padrões comuns:

#### Constructor functions (idiomático)

```go
type CreateUser struct {
    repo   UserRepository
    hasher Hasher
}

func NewCreateUser(repo UserRepository, hasher Hasher) *CreateUser {
    return &CreateUser{repo: repo, hasher: hasher}
}

func (c *CreateUser) Execute(ctx context.Context, in Input) (User, error) {
    // ...
}
```

Wiring manual em `cmd/api/main.go`:

```go
func main() {
    db := setupDB()
    repo := postgres.NewUserRepository(db)
    hasher := bcrypt.NewHasher(10)
    createUser := user.NewCreateUser(repo, hasher)
    // ... wiring HTTP handlers, etc.
}
```

#### Wire (Google) — DI compile-time

Gera código de wiring a partir de "providers". Sem reflection, fail-fast em build.

#### Fx (Uber) — DI runtime

Lifecycle hooks, módulos. Mais "framework-like"; útil em apps grandes.

### Módulos e coesão

#### Bounded context = pacote

```
internal/user/      # bounded context user
├── domain.go       # User struct, UserRepository interface
├── usecase.go      # CreateUser, UpdateUser, etc.
├── repository.go   # PostgresUserRepository
└── http.go         # handlers HTTP
```

#### Visibilidade

- `PascalCase` = exportado (visível fora do pacote).
- `camelCase` = privado ao pacote.
- `internal/` no path = só importável pelo módulo dono. Garantia da toolchain.

#### Compartilhamento

| Código | Onde colocar |
| -------- | ------------- |
| Usado em apenas um pacote | Próprio pacote (privado) |
| Usado por dois ou mais pacotes do módulo | `internal/shared/<área>/` |
| API pública para outros módulos | `pkg/<área>/` |

### Separação de responsabilidades

#### Handler vs. usecase

```go
// Handler "gordo" (anti-pattern)
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    json.NewDecoder(r.Body).Decode(&req)
    if exists, _ := h.repo.FindByEmail(r.Context(), req.Email); exists != nil {
        http.Error(w, "email exists", http.StatusConflict)
        return
    }
    hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
    user, _ := h.repo.Save(r.Context(), User{...})
    json.NewEncoder(w).Encode(user)
}

// Handler fino + usecase (idiomático)
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, err)
        return
    }
    out, err := h.createUser.Execute(r.Context(), req.toInput())
    if err != nil {
        writeError(w, mapErrorToStatus(err), err)
        return
    }
    writeJSON(w, http.StatusCreated, out)
}
```

### Trade-offs (Go)

#### Monolito modular vs. microserviços

Igual ao TS. Em Go, monoliths modulares se beneficiam especialmente de `internal/<ctx>/` para isolamento.

#### Wire vs. Fx vs. wiring manual

| Aspecto | Manual | Wire | Fx |
| --------- | -------- | ------ | ----- |
| Boilerplate | Alto em apps grandes | Médio (gera código) | Baixo |
| Magia | Nenhuma | Geração explícita | Reflection runtime |
| Testabilidade | Alta | Alta | Alta |
| Quando usar | Apps pequenos/médios | Compile-time DI desejado | Apps com lifecycle complexo |

#### ORM vs. SQL puro vs. sqlc

| Aspecto | ORM (gorm, ent) | SQL puro (database/sql) | sqlc |
| --------- | ----------------- | ------------------------ | ------ |
| Boilerplate | Baixo | Alto | Médio |
| Type safety | Médio | Baixo (scan manual) | Alto (geração estática) |
| Performance | Variável | Alta | Alta |
| Clean Arch friendly | Médio | Alto | Alto |

### ADR em Go

A estrutura é a mesma. ADRs costumam ficar em `docs/adr/` no repositório, com `adr-tools` ou markdown manual.

### Evolução incremental (Go)

Mesma estratégia: criar novo ao lado, migrar consumidores, remover antigo.

```go
// Passo 1: criar novo construtor ao lado
func NewNotificationServiceV2(...) *NotificationServiceV2 { ... }

// Passo 2: migrar callers gradualmente

// Passo 3: remover V1 quando não houver imports
```

`go vet` e `golangci-lint` (com `unused`) ajudam a detectar código morto.
