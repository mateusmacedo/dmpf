# Go — tipos avançados

Detalhamento de generics, interfaces, type assertions/switches, embedding e padrões idiomáticos de tipagem.

## Generics (Go 1.18+)

### Sintaxe básica

```go
func Map[T, U any](s []T, f func(T) U) []U {
    out := make([]U, len(s))
    for i, v := range s {
        out[i] = f(v)
    }
    return out
}

names := Map([]User{u1, u2}, func(u User) string { return u.Name })
```

### Constraints

`any` aceita qualquer tipo. `comparable` aceita tipos comparáveis com `==`.

```go
func Contains[T comparable](slice []T, target T) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}
```

### Constraints customizadas

```go
type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](xs []T) T {
    var total T
    for _, x := range xs {
        total += x
    }
    return total
}
```

`~int` significa "qualquer tipo cujo underlying type é `int`" — inclui `type UserID int`.

### Constraints com métodos

```go
type Stringer interface {
    String() string
}

func JoinStrings[T Stringer](items []T, sep string) string {
    parts := make([]string, len(items))
    for i, item := range items {
        parts[i] = item.String()
    }
    return strings.Join(parts, sep)
}
```

### Tipo genérico

```go
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(v T) {
    s.items = append(s.items, v)
}

func (s *Stack[T]) Pop() (T, bool) {
    var zero T
    if len(s.items) == 0 {
        return zero, false
    }
    n := len(s.items) - 1
    v := s.items[n]
    s.items = s.items[:n]
    return v, true
}
```

### Inferência

```go
result := Map([]int{1, 2, 3}, func(n int) string { return strconv.Itoa(n) })
// inferido: Map[int, string]
```

Quando a inferência falha, especifique:

```go
result := Map[int, string]([]int{1, 2, 3}, conv)
```

### Quando NÃO usar generics

Para casos triviais ou com 1-2 chamadas, função concreta é mais clara:

```go
// Genérico desnecessário
func First[T any](s []T) T {
    return s[0]
}

// Concreto explícito
func firstUser(users []User) User {
    return users[0]
}
```

Generics são úteis para coleções (`Map`, `Filter`, `Reduce`), estruturas (`Stack`, `Set`, `Cache`) e operações realmente polimórficas. Para o resto, prefira código concreto.

---

## Interfaces

### Satisfação implícita

Em Go, não há `implements`. Um tipo satisfaz uma interface ao ter os métodos com a assinatura correta.

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type FileReader struct{ /* ... */ }

func (f *FileReader) Read(p []byte) (int, error) { /* ... */ }

// FileReader satisfaz Reader automaticamente
var r Reader = &FileReader{}
```

### Interface no consumidor (Accept interfaces, return structs)

Defina a interface onde ela é consumida, não onde é implementada:

```go
// internal/user/usecase.go (consumidor)
type UserStore interface {
    FindByID(ctx context.Context, id string) (*User, error)
}

type CreateUseCase struct {
    store UserStore
}

func NewCreateUseCase(store UserStore) *CreateUseCase {
    return &CreateUseCase{store: store}
}
```

```go
// internal/infra/postgres/user_repository.go (implementador)
type UserRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository { /* ... */ }

// retorna struct concreta, não interface
func (r *UserRepository) FindByID(ctx context.Context, id string) (*user.User, error) { /* ... */ }
```

Vantagens:
- Cada consumidor define só o que precisa (interface segregation).
- Implementador não precisa importar pacote do consumidor.
- Testes do consumidor não dependem de implementação real.

### Interfaces pequenas

Idiomatic Go favorece interfaces pequenas (1-3 métodos):

```go
// Stdlib
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type Closer interface { Close() error }

// Compostas
type ReadCloser interface {
    Reader
    Closer
}
```

### Empty interface vs `any`

`any` é alias para `interface{}` desde Go 1.18. Prefira `any`:

```go
func Print(v any) { /* ... */ }   // moderno
func Print(v interface{}) { /* ... */ } // pré-1.18
```

Use com moderação — `any` perde tipagem. Prefira generics quando o tipo é parametrizável.

### Nil interface gotcha

```go
type Error interface { Error() string }

func find() Error {
    var e *MyError = nil
    return e // interface NÃO é nil! contém (*MyError, nil)
}

if find() != nil {
    // entra aqui — bug clássico
}
```

Solução: retorne `nil` explicitamente, não um pointer nil tipado.

```go
func find() Error {
    if condition {
        return &MyError{}
    }
    return nil // interface nil de verdade
}
```

---

## Type assertions e type switches

### Type assertion

```go
var i any = "hello"

s := i.(string)         // panic se não for string
s, ok := i.(string)     // ok = false se não for string
```

Use a forma `, ok` exceto quando você tem certeza absoluta — o panic é difícil de depurar.

### Type switch

```go
func describe(i any) string {
    switch v := i.(type) {
    case int:
        return fmt.Sprintf("int: %d", v)
    case string:
        return fmt.Sprintf("string: %s", v)
    case []byte:
        return fmt.Sprintf("bytes: %d", len(v))
    case nil:
        return "nil"
    default:
        return fmt.Sprintf("unknown: %T", v)
    }
}
```

### Conversão entre interfaces

```go
type Reader interface { Read([]byte) (int, error) }
type Closer interface { Close() error }

func readAndClose(r Reader) error {
    if c, ok := r.(Closer); ok {
        defer c.Close()
    }
    // ...
}
```

### `errors.As` para erro tipado (preferido a type assertion)

```go
var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Println(ve.Field)
}
```

`errors.As` percorre a cadeia de wrapping (`%w`) automaticamente — type assertion crua não.

---

## Struct embedding

### Composição em vez de herança

```go
type Animal struct {
    Name string
}

func (a *Animal) Describe() string {
    return "I am " + a.Name
}

type Dog struct {
    Animal           // embedding (sem nome de campo)
    Breed string
}

d := Dog{
    Animal: Animal{Name: "Rex"},
    Breed:  "Labrador",
}

d.Name           // promovido — equivale a d.Animal.Name
d.Describe()     // método promovido — equivale a d.Animal.Describe()
```

### Embedding de interface

```go
type ReadWriter interface {
    Reader
    Writer
}

type LoggedDB struct {
    *sql.DB           // embed: promove métodos Query, Exec, etc.
    logger *slog.Logger
}

func (l *LoggedDB) Query(q string) (*sql.Rows, error) {
    l.logger.Info("query", "sql", q)
    return l.DB.Query(q) // chama método embedded explicitamente
}
```

### Override por shadowing

Se o tipo externo declara método com mesmo nome, ele "sombrea" o embedded:

```go
type Cat struct {
    Animal
}

func (c *Cat) Describe() string {
    return "Meow, I am " + c.Name
}

c.Describe()        // chama Cat.Describe
c.Animal.Describe() // chama Animal.Describe
```

### Ambiguidade

Se dois embeds têm o mesmo método, o uso direto é ambíguo:

```go
type A struct{}
func (a A) Hello() {}

type B struct{}
func (b B) Hello() {}

type C struct {
    A
    B
}

var c C
c.Hello()     // erro de compilação — ambiguity
c.A.Hello()   // ok
c.B.Hello()   // ok
```

### Use casos típicos

- `bytes.Buffer` embedded em writer customizado.
- `sync.Mutex` embedded em struct compartilhada (dá métodos `Lock`/`Unlock`).
- Test helpers embedded para reuso de setup.
- Decorator: tipo externo embed e adiciona logging/metrics.

---

## Pointer vs value receivers

### Pointer receiver

Use quando:
- Método modifica o receiver.
- Tipo é grande (cópia custosa).
- Tipo contém `sync.Mutex`, `sync.WaitGroup`, etc. (não copiar).

```go
func (u *User) SetEmail(email string) {
    u.email = email // modifica
}

func (b *bigStruct) Process() { /* evita cópia de muitos campos */ }
```

### Value receiver

Use quando:
- Tipo é pequeno e imutável.
- Quer semântica de valor (cada chamada vê uma cópia).

```go
type Money struct {
    Cents int64
    Currency string
}

func (m Money) Add(other Money) Money {
    return Money{Cents: m.Cents + other.Cents, Currency: m.Currency}
}
```

### Consistência

Mantenha consistência por tipo. Se um método usa `*T`, todos usam:

```go
type User struct{ /* ... */ }

func (u *User) Name() string  { return u.name }   // pointer
func (u *User) SetName(s string) { u.name = s }   // pointer (precisa)
// não misture com (u User) Foo() — confunde
```

### Interface satisfeita por método set

- Métodos com value receiver: satisfeitos por `T` E por `*T`.
- Métodos com pointer receiver: satisfeitos APENAS por `*T`.

```go
type Saver interface { Save() error }

type Repo struct{}
func (r *Repo) Save() error { return nil }

var s Saver = &Repo{} // ok
var s Saver = Repo{}  // erro: Repo não satisfaz Saver
```

---

## Padrões adicionais

### Type alias vs new type

```go
type UserID = string  // alias — UserID É string
type UserID string    // new type — UserID NÃO é string (precisa cast)
```

`type UserID string` é geralmente preferível: cria tipo distinto que evita misturar IDs.

```go
func FindUser(id UserID) {}

var s string = "abc"
FindUser(s)           // erro — precisa converter
FindUser(UserID(s))   // ok
```

### Stringer

```go
type Status int

const (
    StatusActive Status = iota
    StatusInactive
    StatusBanned
)

func (s Status) String() string {
    switch s {
    case StatusActive:   return "active"
    case StatusInactive: return "inactive"
    case StatusBanned:   return "banned"
    default:             return fmt.Sprintf("unknown(%d)", int(s))
    }
}

fmt.Println(StatusActive) // "active" — usa Stringer automaticamente
```

### iota para enums

```go
type Day int

const (
    Sunday Day = iota
    Monday
    Tuesday
    Wednesday
    Thursday
    Friday
    Saturday
)

const (
    _  = iota                 // ignora 0
    KB = 1 << (10 * iota)     // 1 << 10
    MB                        // 1 << 20
    GB                        // 1 << 30
)
```

### Struct tags

```go
type User struct {
    ID    string `json:"id" db:"id"`
    Email string `json:"email" db:"email" validate:"required,email"`
    Name  string `json:"name,omitempty" db:"name"`
    pwd   string `json:"-"`              // ignorado em JSON
}
```

Tags são metadados lidos por reflection (json.Marshal, validator, sqlx).

### Functional options

Padrão idiomático para construtores com muitas opções:

```go
type ServerOption func(*Server)

func WithPort(p int) ServerOption {
    return func(s *Server) { s.port = p }
}

func WithLogger(l *slog.Logger) ServerOption {
    return func(s *Server) { s.logger = l }
}

func NewServer(opts ...ServerOption) *Server {
    s := &Server{port: 8080}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

s := NewServer(WithPort(3000), WithLogger(logger))
```

Vantagens: defaults claros, evolução sem breaking changes, ordem irrelevante.

### Constraint interface vs regular interface

Constraint interfaces (em type parameters) podem usar união e `~`:

```go
// Constraint — só serve em [T constraint]
type Number interface {
    ~int | ~float64
}

// Regular interface — serve em variável/parâmetro
type Writer interface {
    Write([]byte) (int, error)
}
```

Não pode usar union/`~` em interface regular.

### Generic + type assertion

```go
func ToString[T any](v T) string {
    if s, ok := any(v).(fmt.Stringer); ok {
        return s.String()
    }
    return fmt.Sprintf("%v", v)
}
```

Cast `T` → `any` para então fazer assertion. Necessário porque `T any` não permite assertion direto.

### Ponteiros para opcionais

Em vez de zero value ambíguo, ponteiro distingue "não setado" de "zero":

```go
type UpdateUser struct {
    Email *string  // nil = não atualizar; não-nil = novo valor (mesmo "")
    Age   *int     // nil = não atualizar
}

if u.Email != nil {
    user.Email = *u.Email
}
```

Trade-off: mais verboso. Em DTOs JSON, considere `omitempty` + sentinelas em vez de pointer everywhere.
