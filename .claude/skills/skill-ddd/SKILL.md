---
name: skill-ddd
description: |
  Use esta skill quando o usuário pedir para "modelar domínio", "criar entidade",
  "value object", "agregado", "bounded context", "linguagem ubíqua" ou mencionar
  Domain-Driven Design, modelagem de domínio ou regras de negócio complexas.
  Cobre entidades, value objects, agregados, repositórios, domain services, use cases e bounded contexts.
model: sonnet
---

# Domain-Driven Design (DDD)

## Objetivo
Fornecer vocabulário e padrões de DDD: entidades, value objects, agregados, repositórios, domain services, use cases e bounded contexts. DDD é uma abordagem útil em domínios com regras complexas; nem todo projeto precisa aplicar todos os building blocks.

## Quando usar
- Ao modelar domínios com regras complexas.
- Ao definir entidades, VOs, agregados e repositórios.
- Ao separar bounded contexts e linguagem ubíqua.
- Ao estruturar camadas `domain/`, `application/` e `infra/`.

## Conceitos

### Ubiquitous language

Mesma linguagem no código e no negócio:

```typescript
const item = cart.items[0]
item.qty = 5

// Alinhado com o domínio
const product = shoppingCart.products[0]
product.quantity = 5
```

### Bounded context

Cada contexto tem sua própria linguagem. Exemplo:

```
Petições
├── Documento = peça jurídica
├── Parte = autor ou réu
└── Prazo = deadline processual

Faturamento
├── Documento = nota fiscal
├── Cliente = pagador
└── Crédito = saldo disponível
```

## Building blocks

### Entity

Objeto com identidade única, regras de negócio e invariantes.

```typescript
class Permission {
  constructor(
    public readonly id: string,
    private resource: string,
    private action: 'read' | 'write' | 'delete',
    private active: boolean = true,
  ) {
    this.validateInvariants()
  }

  private validateInvariants() {
    if (!this.resource) throw new ValidationError('Resource é obrigatório')
    if (!this.action) throw new ValidationError('Action é obrigatória')
  }

  canAccess(targetResource: string, targetAction: string): boolean {
    return this.active && this.resource === targetResource && this.action === targetAction
  }

  deactivate() {
    this.active = false
  }

  isEqual(other: Permission): boolean {
    return this.id === other.id
  }
}
```

### Value Object

Objeto imutável, sem identidade, definido pelos atributos.

```typescript
class Email {
  public readonly value: string

  constructor(value: string) {
    if (!this.isValid(value)) {
      throw new ValidationError('Email inválido')
    }
    this.value = value.toLowerCase()
  }

  private isValid(email: string): boolean {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
  }

  isEqual(other: Email): boolean {
    return this.value === other.value
  }
}

class Money {
  constructor(
    public readonly amount: number,
    public readonly currency: string,
  ) {
    if (amount < 0) throw new ValidationError('Valor não pode ser negativo')
  }

  add(other: Money): Money {
    if (this.currency !== other.currency) {
      throw new ValidationError('Moedas diferentes')
    }
    return new Money(this.amount + other.amount, this.currency)
  }

  isEqual(other: Money): boolean {
    return this.amount === other.amount && this.currency === other.currency
  }
}
```

### Aggregate

Fronteira de consistência, acessada apenas pela raiz.

```typescript
class Document {
  private sections: DocumentSection[] = []

  constructor(
    public readonly id: string,
    private title: string,
    private authorId: string,
    private status: 'draft' | 'review' | 'published' = 'draft',
  ) {}

  addSection(title: string, content: string) {
    if (this.status === 'published') {
      throw new ConflictError('Não é possível editar documento publicado')
    }
    const section = new DocumentSection(
      crypto.randomUUID(),
      title,
      content,
      this.sections.length,
    )
    this.sections.push(section)
  }

  publish() {
    if (this.sections.length === 0) {
      throw new ValidationError('Documento sem seções não pode ser publicado')
    }
    this.status = 'published'
  }

  getSections(): ReadonlyArray<DocumentSection> {
    return [...this.sections]
  }
}

class DocumentSection {
  constructor(
    public readonly id: string,
    public title: string,
    public content: string,
    public order: number,
  ) {}
}
```

### Repository

Interface no domínio, implementação na infra.

```typescript
// domain/contracts/UserRepository.ts
type UserRepository = {
  findById(id: string): Promise<User | null>
  findByEmail(email: Email): Promise<User | null>
  save(user: User): Promise<void>
  delete(id: string): Promise<void>
}

// infra/repositories/TypeOrmUserRepository.ts
class TypeOrmUserRepository implements UserRepository {
  constructor(private readonly repository: Repository<UserEntity>) {}

  async findById(id: string): Promise<User | null> {
    const entity = await this.repository.findOne({ where: { id } })
    return entity ? this.toDomain(entity) : null
  }

  async save(user: User): Promise<void> {
    const entity = this.toEntity(user)
    await this.repository.save(entity)
  }

  private toDomain(entity: UserEntity): User {
    return new User(entity.id, new Email(entity.email), entity.name)
  }

  private toEntity(user: User): UserEntity {
    return { id: user.id, email: user.email.value, name: user.name }
  }
}
```

### Domain service

Lógica que não pertence a uma única entidade.

```typescript
class CreditTransferService {
  transfer(sender: User, receiver: User, amount: number) {
    if (sender.credits < amount) {
      throw new InsufficientCreditsError(amount, sender.credits)
    }

    sender.debitCredits(amount)
    receiver.creditCredits(amount)
  }
}
```

### Application service / use case

Orquestra objetos de domínio e define a fronteira de transação.

```typescript
class TransferCreditsUseCase {
  constructor(
    private readonly userRepository: UserRepository,
    private readonly creditTransferService: CreditTransferService,
  ) {}

  async execute(input: TransferCreditsDto): Promise<void> {
    const sender = await this.userRepository.findById(input.senderId)
    if (!sender) throw new NotFoundError('User', input.senderId)

    const receiver = await this.userRepository.findById(input.receiverId)
    if (!receiver) throw new NotFoundError('User', input.receiverId)

    this.creditTransferService.transfer(sender, receiver, input.amount)

    await this.userRepository.save(sender)
    await this.userRepository.save(receiver)
  }
}
```

## Mapeamento para estrutura backend

| Conceito DDD | Camada | Diretório típico |
|--------------|--------|------------------|
| Entity | Domain | `domain/entities/` |
| Value Object | Domain | `domain/value-objects/` |
| Domain Error | Domain | `domain/errors/` |
| Domain Service | Domain | `domain/services/` |
| Repository (contrato) | Domain | `domain/contracts/` |
| Use Case | Application | `application/use-cases/` |
| DTO | Application | `application/dtos/` |
| Repository (impl) | Infra | `infra/repositories/` |
| API Client | Infra | `infra/clients/` |

Projetos com frameworks modulares (ex.: NestJS) costumam agrupar por feature; engines baseadas em pipeline/step reorganizam conceitos em classes de passo. Adapte ao padrão do projeto.

## Anti-patterns

```typescript
// Modelo anêmico
type User = { name: string; email: string; credits: number }
const debitCredits = (user: User, amount: number) => {
  user.credits -= amount
}

// Modelo rico: lógica encapsulada
class User {
  constructor(
    public readonly id: string,
    public readonly email: Email,
    public name: string,
    private _credits: number = 0,
  ) {}

  get credits() { return this._credits }

  debitCredits(amount: number) {
    if (this._credits < amount) {
      throw new InsufficientCreditsError(amount, this._credits)
    }
    this._credits -= amount
  }

  creditCredits(amount: number) {
    this._credits += amount
  }
}

// Regra de negócio no controller (tende a vazar lógica)
const createPetitionController = async (req, res) => {
  const credits = user.plan.base + user.bonus - user.usage
  if (credits < 10) return res.status(402).json({ error: 'Sem créditos' })
}

// Melhor: lógica no domínio/caso de uso
const calculateRemainingCredits = (plan: Plan, bonus: number, usage: number) =>
  Math.max(0, plan.baseCredits + bonus - usage)

const execute = async (input: CreatePetitionDto) => {
  const remaining = calculateRemainingCredits(user.plan, user.bonus, user.usage)
  if (remaining < 10) throw new InsufficientCreditsError(10, remaining)
}
```

## Checklist

- [ ] Ubiquitous language definida para o contexto.
- [ ] Bounded contexts identificados.
- [ ] Entidades com identidade e regras encapsuladas.
- [ ] Value objects imutáveis com igualdade por valor.
- [ ] Agregados com fronteira de consistência clara.
- [ ] Repositórios: contrato em `domain/`, implementação em `infra/`.
- [ ] Domain services para lógica entre entidades.
- [ ] Use cases orquestrando na camada `application/`.
- [ ] Regras de negócio fora de controllers e infra.

---

## 🔹 Go: DDD em Go

Os conceitos são universais. Em Go, a ausência de classes e a preferência por composição ajustam a forma de expressar os building blocks.

### Entity

Struct com identidade e métodos com pointer receiver para mutação. Invariantes são verificadas no construtor (factory function) e em mutadores.

```go
package permission

import "errors"

type Action string

const (
    Read   Action = "read"
    Write  Action = "write"
    Delete Action = "delete"
)

type Permission struct {
    id       string
    resource string
    action   Action
    active   bool
}

func New(id, resource string, action Action) (*Permission, error) {
    if resource == "" {
        return nil, errors.New("resource é obrigatório")
    }
    if action == "" {
        return nil, errors.New("action é obrigatória")
    }
    return &Permission{
        id:       id,
        resource: resource,
        action:   action,
        active:   true,
    }, nil
}

func (p *Permission) ID() string { return p.id }

func (p *Permission) CanAccess(resource string, action Action) bool {
    return p.active && p.resource == resource && p.action == action
}

func (p *Permission) Deactivate() {
    p.active = false
}

func (p *Permission) Equals(other *Permission) bool {
    return other != nil && p.id == other.id
}
```

Convenções:
- Campos privados (lowercase) + getters quando necessário — encapsula invariantes.
- Construtor `New` (ou `NewX`) retorna `(*T, error)`.
- Pointer receiver para métodos que mutam; value receiver para os que apenas leem (mantenha consistência por tipo).

### Value Object

Struct com semântica de valor (imutável por convenção). Operações retornam novo VO.

```go
package money

import "errors"

type Money struct {
    amount   int64  // em centavos para evitar float
    currency string
}

func New(amount int64, currency string) (Money, error) {
    if amount < 0 {
        return Money{}, errors.New("valor não pode ser negativo")
    }
    if currency == "" {
        return Money{}, errors.New("moeda obrigatória")
    }
    return Money{amount: amount, currency: currency}, nil
}

func (m Money) Amount() int64    { return m.amount }
func (m Money) Currency() string { return m.currency }

func (m Money) Add(other Money) (Money, error) {
    if m.currency != other.currency {
        return Money{}, errors.New("moedas diferentes")
    }
    return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}

func (m Money) Equals(other Money) bool {
    return m.amount == other.amount && m.currency == other.currency
}
```

Convenções:
- Value receivers (sem `*`) — VO é copiado, não compartilhado.
- Sem mutadores; operações retornam novo valor.
- Construtor garante invariantes; campos privados impedem construção inválida fora do pacote.

### Aggregate

Raiz é struct com pointer receiver; entidades internas são acessadas via métodos da raiz.

```go
package document

import (
    "errors"

    "github.com/google/uuid"
)

type Status string

const (
    StatusDraft     Status = "draft"
    StatusReview    Status = "review"
    StatusPublished Status = "published"
)

type Document struct {
    id       string
    title    string
    authorID string
    status   Status
    sections []Section
}

type Section struct {
    ID      string
    Title   string
    Content string
    Order   int
}

func New(id, title, authorID string) *Document {
    return &Document{
        id:       id,
        title:    title,
        authorID: authorID,
        status:   StatusDraft,
    }
}

func (d *Document) AddSection(title, content string) error {
    if d.status == StatusPublished {
        return errors.New("documento publicado não pode receber seções")
    }
    d.sections = append(d.sections, Section{
        ID:      uuid.NewString(),
        Title:   title,
        Content: content,
        Order:   len(d.sections),
    })
    return nil
}

func (d *Document) Publish() error {
    if len(d.sections) == 0 {
        return errors.New("documento sem seções não pode ser publicado")
    }
    d.status = StatusPublished
    return nil
}

func (d *Document) Sections() []Section {
    out := make([]Section, len(d.sections))
    copy(out, d.sections)
    return out
}
```

A cópia em `Sections()` evita que o caller mute o slice interno (preserva a fronteira).

### Repository

Interface no pacote consumidor (domínio); implementação no pacote de infra. É idiomático em Go.

```go
// internal/user/user.go (domínio)
package user

import "context"

type Repository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    FindByEmail(ctx context.Context, email Email) (*User, error)
    Save(ctx context.Context, u *User) error
    Delete(ctx context.Context, id string) error
}
```

```go
// internal/infra/postgres/user_repository.go (infra)
package postgres

import (
    "context"
    "database/sql"

    "myapp/internal/user"
)

type UserRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*user.User, error) {
    var row userRow
    err := r.db.QueryRowContext(ctx, `SELECT id, email, name FROM users WHERE id = $1`, id).
        Scan(&row.id, &row.email, &row.name)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return toDomain(row)
}
```

Pontos:
- A interface vive onde é consumida (domínio), não onde é implementada (infra) — implementação é implícita em Go.
- A struct concreta (`*UserRepository`) é o que infra exporta.
- Mappers (`toDomain`/`toRow`) ficam no pacote de infra.

### Domain service

Função ou struct quando a operação não pertence a uma única entidade.

```go
package transfer

import "myapp/internal/user"

func TransferCredits(sender, receiver *user.User, amount int64) error {
    if err := sender.DebitCredits(amount); err != nil {
        return err
    }
    receiver.CreditCredits(amount)
    return nil
}
```

Para serviços que precisam de dependências (ex.: gerador de IDs, clock), use struct com construtor.

### Application service / Use case

Struct com construtor + método `Execute`. Orquestra repositórios e domain services dentro da fronteira de transação.

```go
package transfercredits

import (
    "context"

    "myapp/internal/user"
)

type Input struct {
    SenderID   string
    ReceiverID string
    Amount     int64
}

type UseCase struct {
    users user.Repository
    tx    TxManager
}

func New(users user.Repository, tx TxManager) *UseCase {
    return &UseCase{users: users, tx: tx}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) error {
    return uc.tx.Run(ctx, func(ctx context.Context) error {
        sender, err := uc.users.FindByID(ctx, in.SenderID)
        if err != nil || sender == nil {
            return user.ErrNotFound
        }

        receiver, err := uc.users.FindByID(ctx, in.ReceiverID)
        if err != nil || receiver == nil {
            return user.ErrNotFound
        }

        if err := transfer.TransferCredits(sender, receiver, in.Amount); err != nil {
            return err
        }

        if err := uc.users.Save(ctx, sender); err != nil {
            return err
        }
        return uc.users.Save(ctx, receiver)
    })
}
```

### Mapeamento para estrutura backend (Go)

| Conceito DDD | Pacote típico |
|--------------|---------------|
| Entity | `internal/<contexto>/<entidade>.go` |
| Value Object | `internal/<contexto>/<vo>.go` ou subpacote |
| Domain Error | `internal/<contexto>/errors.go` (sentinelas) |
| Domain Service | `internal/<contexto>/service.go` |
| Repository (interface) | `internal/<contexto>/repository.go` |
| Use Case | `internal/<contexto>/<acao>/usecase.go` |
| Repository (impl) | `internal/infra/<driver>/<entidade>_repository.go` |
| HTTP handler | `internal/<contexto>/http.go` |

Bounded context = pacote em `internal/<contexto>/`. A toolchain garante que pacotes externos não importem `internal/` fora do módulo.

### Anti-patterns (Go)

```go
// Modelo anêmico: struct exportada sem invariantes
type User struct {
    Name    string
    Email   string
    Credits int64
}

func DebitCredits(u *User, amount int64) {
    u.Credits -= amount  // qualquer caller pode violar invariante
}

// Modelo rico: estado privado + métodos com regras
type User struct {
    name    string
    email   Email
    credits int64
}

func (u *User) DebitCredits(amount int64) error {
    if u.credits < amount {
        return ErrInsufficientCredits
    }
    u.credits -= amount
    return nil
}

// Regra de negócio no handler (vaza lógica)
func (h *Handler) CreatePetition(w http.ResponseWriter, r *http.Request) {
    credits := user.Plan.Base + user.Bonus - user.Usage
    if credits < 10 {
        http.Error(w, "sem créditos", http.StatusPaymentRequired)
        return
    }
}

// Idiomático: regra no domínio/use case
func (u *User) RemainingCredits() int64 {
    return max(0, u.plan.Base+u.bonus-u.usage)
}
```

Outros anti-patterns específicos de Go:

- Compartilhar pointer para estado mutável entre goroutines sem `sync` ou channel.
- Exportar campos da entidade (rompe encapsulamento e invariantes).
- Interface "fat" no consumidor (declarar mais métodos do que usa).
- Mutar VO via pointer receiver — VO deve ser imutável.

### Checklist (Go)

- [ ] Bounded context = pacote em `internal/<ctx>/`.
- [ ] Entidades com construtor que valida invariantes.
- [ ] Value objects como structs com value receivers e campos privados.
- [ ] Agregados expõem métodos da raiz; estado interno encapsulado.
- [ ] Repositórios com interface no pacote de domínio, struct concreta na infra.
- [ ] Use cases como struct com `Execute(ctx, in)` retornando `(out, error)`.
- [ ] Mappers domínio↔persistência ficam no pacote de infra.
- [ ] Sem campos exportados que quebrem invariantes.
