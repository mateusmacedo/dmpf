---
name: skill-clean-code
description: |
  Use esta skill quando o usuário pedir para "melhorar código", "refatorar",
  "aplicar clean code", "renomear variáveis", "reduzir complexidade", "code smell"
  ou mencionar SOLID, tamanho de funções, naming ou legibilidade.
  Cobre naming, funções pequenas, SOLID, code smells e refatoração segura.
  Para convenções específicas de projeto, ver `skill-code-standards`.
model: sonnet
---

# Clean Code

## Objetivo
Reunir princípios gerais de Clean Code: naming, funções pequenas, SOLID, code smells e refatoração.

## Quando usar
- Ao melhorar legibilidade e coesão.
- Ao revisar naming e responsabilidades.
- Ao remover code smells com refatoração segura.

## Naming

### Variáveis

```typescript
// Menos claro
const d = new Date()
const arr = users.filter((u) => u.a)

// Mais revelador
const createdAt = new Date()
const activeUsers = users.filter((user) => user.isActive)
```

### Funções

```typescript
// Vago
const handle = () => {}
const process = (data) => {}

// Ação clara
const submitLoginForm = () => {}
const validateUserInput = (input) => {}
```

### Booleanos

```typescript
// Sem prefixo
const loading = true
const admin = user.role === 'admin'

// Prefixo is/has/can
const isLoading = true
const isAdmin = user.role === 'admin'
const hasPermission = checkPermission(user)
const canEdit = isAdmin && hasPermission
```

## Funções

### Pequenas e focadas

```typescript
// Faz muitas coisas
const processUser = (data) => {
  validate(data)
  const user = transform(data)
  save(user)
  sendEmail(user)
  log(user)
}

// Responsabilidade única
const createUser = (data) => {
  const validated = validateUserData(data)
  const user = buildUser(validated)
  return saveUser(user)
}
```

### Poucos parâmetros

```typescript
// Muitos parâmetros posicionais
const createUser = (name, email, age, role, dept, manager) => {}

// Objeto de configuração
type CreateUserInput = {
  name: string
  email: string
  role: UserRole
}

const createUser = (input: CreateUserInput) => {}
```

### Early return

```typescript
// Nesting profundo
const processOrder = (order) => {
  if (order) {
    if (order.isValid) {
      if (order.items.length > 0) {
        // lógica
      }
    }
  }
}

// Guard clauses
const processOrder = (order) => {
  if (!order) return
  if (!order.isValid) return
  if (order.items.length === 0) return

  // lógica
}
```

## SOLID

### Single Responsibility

```typescript
class UserService {
  createUser() {}
  sendEmail() {}
  generateReport() {}
  validateInput() {}
}

// Mais coeso
class UserService { createUser() {} }
class EmailService { send() {} }
class ReportService { generate() {} }
```

### Open/Closed

```typescript
type PaymentMethod = {
  process: (amount: number) => Promise<void>
}

const creditCard: PaymentMethod = {
  process: async (amount) => { /* ... */ }
}

const pix: PaymentMethod = {
  process: async (amount) => { /* ... */ }
}
```

### Dependency Inversion

```typescript
// Dependência concreta
const userService = new UserService(new PostgresDB())

// Dependência de abstração
type Database = {
  query: (sql: string) => Promise<unknown>
}

const createUserService = (db: Database) => ({
  findUser: (id) => db.query(`SELECT * FROM users WHERE id = ?`, [id])
})
```

## Code Smells

| Smell | Tendência de solução |
|-------|----------------------|
| Função longa | Extract function |
| Muitos parâmetros | Parameter object |
| Código duplicado | Extract function/module |
| Nesting profundo | Early return |
| Comentários que explicam "como" | Renomear/refatorar |
| Magic numbers | Constantes nomeadas |
| God class/module | Split |

Observação sobre limites numéricos: valores como "função ≤ 20 linhas" ou "arquivo ≤ 300 linhas" são heurísticas. Os projetos costumam definir seus próprios limites; ver `rules/file-size-limits.md`. O importante é manter o tamanho proporcional à complexidade real.

## Refatoração

### Quando refatorar

- Antes de adicionar feature em área confusa.
- Ao corrigir bug.
- Durante code review.
- Evite refatorar "por melhorar" sem critério, principalmente sem testes.

### Técnicas

```typescript
// Extract variable
if (user.role === 'admin' && user.permissions.includes('delete')) {}

const canDelete = user.role === 'admin' && user.permissions.includes('delete')
if (canDelete) {}

// Extract function
const price = basePrice * (1 - discount) * (1 + tax)

const calculateFinalPrice = (base, discount, tax) =>
  base * (1 - discount) * (1 + tax)
```

## Checklist

- [ ] Nomes revelam intenção.
- [ ] Funções com tamanho proporcional à complexidade.
- [ ] Poucos parâmetros ou parameter object.
- [ ] Uma responsabilidade por função.
- [ ] Sem nesting profundo.
- [ ] Sem magic numbers.
- [ ] Sem código duplicado sem justificativa.

---

## 🔹 Go: clean code idiomático

Os princípios de Clean Code são universais. Em Go, há convenções e idiomas próprios — alguns reforçam Clean Code, outros o ajustam.

### Naming (Go)

#### Variáveis

```go
// Menos claro
d := time.Now()
arr := filterUsers(users)

// Mais claro
createdAt := time.Now()
activeUsers := filterActive(users)
```

#### Convenções específicas

- **Receivers**: 1-2 letras derivadas do tipo (`u *User`, não `this`/`self`).
- **Variáveis curtas**: Go favorece nomes curtos em escopos curtos. `i`, `j`, `k` em loops; `r` para `Reader`; `w` para `Writer`. **Quanto maior o escopo, mais descritivo o nome.**
- **Nomes curtos exportados**: `errors.New`, `time.Now` — o pacote já dá contexto. Em Go, `User.Name()` é melhor que `user.GetUserName()`.
- **Sem prefixo `Get`**: getters em Go são `User.Name()`, não `User.GetName()`. Se precisar diferenciar de field, prefixe a function (raro).
- **Booleanos**: `IsActive`, `HasPermission`, `CanEdit` — mesmo padrão.
- **Acrônimos em maiúsculas**: `UserID`, `URLParser`, `HTTPClient` (não `UserId`, `UrlParser`, `HttpClient`).

### Funções

#### Pequenas e focadas

Mesmo princípio. Em Go, funções com erro retornam `(T, error)`:

```go
// Anti-padrão: faz muitas coisas
func processUser(data UserData) error {
    validate(data)
    user := transform(data)
    save(user)
    sendEmail(user)
    log(user)
    return nil
}

// Idiomático: cada step com erro propagado
func CreateUser(ctx context.Context, data UserData) (User, error) {
    validated, err := validateUser(data)
    if err != nil {
        return User{}, fmt.Errorf("validate: %w", err)
    }
    user := buildUser(validated)
    if err := saveUser(ctx, user); err != nil {
        return User{}, fmt.Errorf("save: %w", err)
    }
    return user, nil
}
```

#### Poucos parâmetros

Sem named parameters; opções:

```go
// Anti-padrão: muitos parâmetros posicionais
func CreateUser(name, email string, age int, role string, dept, manager string) {}

// Struct de opções
type CreateUserInput struct {
    Name  string
    Email string
    Role  Role
}
func CreateUser(in CreateUserInput) {}

// Functional options (libs públicas)
func NewServer(opts ...Option) *Server {}
```

#### Early return com erro

```go
// Idiomático: early return em cada erro
func processOrder(order Order) error {
    if order.ID == "" {
        return ErrInvalidOrder
    }
    if !order.IsValid() {
        return ErrOrderNotValid
    }
    if len(order.Items) == 0 {
        return ErrEmptyOrder
    }

    // lógica principal sem nesting
    return nil
}
```

### SOLID em Go

#### Single Responsibility

Mesma ideia. Em Go, um pacote coeso é a unidade primária — um pacote `user` foca em user; emails vão em `mailer`.

#### Open/Closed via interface

```go
type PaymentMethod interface {
    Process(ctx context.Context, amount Money) error
}

type CreditCard struct{ /* ... */ }
func (c *CreditCard) Process(ctx context.Context, amount Money) error { /* ... */ }

type PIX struct{ /* ... */ }
func (p *PIX) Process(ctx context.Context, amount Money) error { /* ... */ }
```

**Diferencial Go**: implementação de interface é **implícita** — não há `implements`. Qualquer tipo que tenha os métodos satisfaz a interface.

#### Dependency Inversion

```go
// Interface declarada onde é consumida (idiomático em Go)
type UserStore interface {
    FindByID(ctx context.Context, id string) (User, error)
    Save(ctx context.Context, u User) error
}

func NewUserService(store UserStore) *UserService {
    return &UserService{store: store}
}
```

**Idiomático em Go**: a interface vive no pacote do *consumidor*, não do *implementador*. O pacote `postgres` exporta `*UserRepository` (struct concreta); o pacote `user` declara `UserStore` interface mínima.

### Code Smells em Go

| Smell | Indicador / solução |
|-------|---------------------|
| Função longa | `funlen` linter; extract function |
| Muitos parâmetros | `gocyclo`/manual; struct de opções |
| Nesting profundo | Early return com erro |
| Magic numbers | `const` em bloco |
| Erro ignorado | `errcheck` linter |
| Goroutine sem context | `goroutineleak` (uber-go/goleak) em testes |
| Interface "fat" | Contraria "interface segregation"; quebrar |
| Abstrações prematuras | "Accept interfaces, return structs" — interfaces no consumidor |

### Refatoração segura (Go)

- `gopls` (LSP) e GoLand fazem rename refactor seguro.
- `go test ./...` antes e depois.
- Em diff grande, `go vet ./...` e `golangci-lint run` capturam regressões de estilo.

#### Extract function / variable

```go
// Antes
if user.Role == "admin" && contains(user.Permissions, "delete") {}

// Depois
canDelete := user.Role == "admin" && contains(user.Permissions, "delete")
if canDelete {}
```

### Idiomas específicos de Go

- **"Accept interfaces, return structs"** — funções recebem interfaces (flexibilidade do caller), retornam tipos concretos (não obriga a abstrair retorno).
- **"A little copying is better than a little dependency"** — duplicar 5 linhas pode ser melhor que importar um pacote externo.
- **"Make the zero value useful"** — structs devem funcionar com valor zero quando possível (`var b bytes.Buffer; b.Write(...)` funciona).
- **"Don't communicate by sharing memory; share memory by communicating"** — channels para coordenação entre goroutines, mutex para proteger estado pequeno.
