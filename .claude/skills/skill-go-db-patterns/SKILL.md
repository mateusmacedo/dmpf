---
name: skill-go-db-patterns
description: |
  Use esta skill ao trabalhar com banco em Go: database/sql, sqlx, sqlc, GORM, ent.
  Cobre repository pattern, query builders, migrations (golang-migrate, atlas, goose),
  relations, transações, paginação, índices e otimização de queries.
model: opus
---

# Go DB — padrões

## Objetivo

Padronizar acesso a banco em Go: entities, repository pattern, queries, migrations e performance.

## Quando usar

- Ao criar ou modificar tabelas/entities.
- Ao implementar repositories na camada de infra.
- Ao escrever queries complexas.
- Ao criar ou revisar migrations.
- Ao investigar problemas de performance em DB.

## Escolha de driver / ORM

| Ferramenta | Quando usar | Trade-off |
| ------------ | ------------- | ----------- |
| `database/sql` (stdlib) | Controle total, sem mágica | Verbosidade |
| `sqlx` | SQL puro + struct scanning | Não mapeia relações |
| `sqlc` | SQL com geração de tipos | Setup adicional |
| GORM | CRUD rápido, convenção | Mágica, queries opacas |
| ent | Schema-first, code generation | Curva de aprendizado |
| `pgx` | Postgres nativo, performance | Específico de PG |

Idiomatic Go favorece SQL explícito (`sqlx` ou `sqlc`). GORM/ent são úteis em CRUDs simples ou com relacionamentos densos.

---

## Entities (database/sql + sqlx)

### Struct com tags

```go
type UserEntity struct {
    ID        string         `db:"id"`
    Name      string         `db:"name"`
    Email     string         `db:"email"`
    Role      string         `db:"role"`
    Metadata  []byte         `db:"metadata"`     // jsonb cru
    DeletedAt sql.NullTime   `db:"deleted_at"`   // nullable
    CreatedAt time.Time      `db:"created_at"`
    UpdatedAt time.Time      `db:"updated_at"`
}
```

### Mapeamento SQL → Go

| SQL | Go | Nullable |
| ----- | ----- | --------- |
| `varchar`, `text` | `string` | `sql.NullString` |
| `int`, `bigint` | `int64` | `sql.NullInt64` |
| `numeric` | `decimal.Decimal` (shopspring) | — |
| `timestamp`, `timestamptz` | `time.Time` | `sql.NullTime` |
| `boolean` | `bool` | `sql.NullBool` |
| `uuid` | `string`, `uuid.UUID` | `*uuid.UUID` |
| `jsonb`, `json` | `[]byte` ou struct + `Scan/Value` | — |
| `text[]` | `pq.StringArray` (lib/pq) | — |
| `enum` | tipo string customizado | — |

### JSONB tipado

```go
type Metadata struct {
    Source string            `json:"source"`
    Tags   []string          `json:"tags"`
    Extra  map[string]string `json:"extra"`
}

func (m Metadata) Value() (driver.Value, error) {
    return json.Marshal(m)
}

func (m *Metadata) Scan(src any) error {
    bytes, ok := src.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, m)
}

type UserEntity struct {
    ID       string   `db:"id"`
    Metadata Metadata `db:"metadata"`
}
```

### Relations explícitas

Sem decorators. Relações são FKs no schema; carregamento é JOIN explícito ou query separada.

```go
type DocumentEntity struct {
    ID             string `db:"id"`
    Title          string `db:"title"`
    OrganizationID string `db:"organization_id"`
    UserID         string `db:"user_id"`
}

// query separada
type DocumentWithUser struct {
    DocumentEntity
    UserName string `db:"user_name"`
}

const q = `
    SELECT d.id, d.title, d.organization_id, d.user_id, u.name AS user_name
    FROM documents d
    JOIN users u ON u.id = d.user_id
    WHERE d.id = $1
`
```

### Enum como string

```go
type DocumentStatus string

const (
    StatusDraft      DocumentStatus = "draft"
    StatusProcessing DocumentStatus = "processing"
    StatusCompleted  DocumentStatus = "completed"
    StatusFailed     DocumentStatus = "failed"
)

func (s DocumentStatus) Valid() bool {
    switch s {
    case StatusDraft, StatusProcessing, StatusCompleted, StatusFailed:
        return true
    }
    return false
}
```

---

## Repository pattern

### Interface no domínio

```go
// internal/user/repository.go
package user

type Repository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    Save(ctx context.Context, u *User) (*User, error)
    Delete(ctx context.Context, id string) error
}
```

### Implementação na infra

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
    err := r.db.GetContext(ctx, &row, `
        SELECT id, name, email, role, created_at, updated_at
        FROM users WHERE id = $1
    `, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, user.ErrNotFound
        }
        return nil, fmt.Errorf("find user %s: %w", id, err)
    }
    return row.toDomain(), nil
}

func (r *UserRepository) Save(ctx context.Context, u *user.User) (*user.User, error) {
    row := fromDomain(u)
    _, err := r.db.NamedExecContext(ctx, `
        INSERT INTO users (id, name, email, role, created_at, updated_at)
        VALUES (:id, :name, :email, :role, :created_at, :updated_at)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            email = EXCLUDED.email,
            role = EXCLUDED.role,
            updated_at = EXCLUDED.updated_at
    `, row)
    if err != nil {
        return nil, fmt.Errorf("save user: %w", err)
    }
    return u, nil
}
```

### Linha do DB ≠ entity de domínio

```go
type userRow struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    Role      string    `db:"role"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}

func (r userRow) toDomain() *user.User {
    return &user.User{
        ID:        r.ID,
        Name:      r.Name,
        Email:     r.Email,
        Role:      user.Role(r.Role),
        CreatedAt: r.CreatedAt,
        UpdatedAt: r.UpdatedAt,
    }
}

func fromDomain(u *user.User) userRow { /* ... */ }
```

Domínio não importa `database/sql` — repositório isola a persistência.

---

## SQL puro vs query builder

| Cenário | Usar |
| --------- | ------ |
| Busca por chave primária | `db.GetContext` com SQL inline |
| WHERE/ORDER BY simples | SQL inline |
| JOINs complexos | SQL inline (legibilidade) |
| Filtros dinâmicos | `squirrel` ou `goqu` |
| Migrations | SQL puro |
| CRUD repetitivo | `sqlc` (gera código) |

### Exemplo com sqlx

```go
const findActiveDocsQuery = `
    SELECT d.id, d.title, d.status, d.created_at, u.name AS user_name
    FROM documents d
    JOIN users u ON u.id = d.user_id
    WHERE d.status = $1
      AND d.created_at >= $2
    ORDER BY d.created_at DESC
    LIMIT $3 OFFSET $4
`

func (r *DocumentRepository) FindActive(ctx context.Context, since time.Time, limit, offset int) ([]*Document, error) {
    var rows []docWithUser
    err := r.db.SelectContext(ctx, &rows, findActiveDocsQuery, "completed", since, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("find active docs: %w", err)
    }
    docs := make([]*Document, len(rows))
    for i, row := range rows {
        docs[i] = row.toDomain()
    }
    return docs, nil
}
```

### Filtros dinâmicos com squirrel

```go
import sq "github.com/Masterminds/squirrel"

func (r *DocumentRepository) Find(ctx context.Context, f Filter) ([]*Document, error) {
    psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
    q := psql.Select("id", "title", "status").From("documents")

    if f.OrgID != "" {
        q = q.Where(sq.Eq{"organization_id": f.OrgID})
    }
    if f.Status != "" {
        q = q.Where(sq.Eq{"status": f.Status})
    }
    if !f.Since.IsZero() {
        q = q.Where(sq.GtOrEq{"created_at": f.Since})
    }
    q = q.OrderBy("created_at DESC").Limit(uint64(f.Limit))

    sqlStr, args, err := q.ToSql()
    if err != nil {
        return nil, fmt.Errorf("build query: %w", err)
    }

    var rows []docRow
    if err := r.db.SelectContext(ctx, &rows, sqlStr, args...); err != nil {
        return nil, fmt.Errorf("find docs: %w", err)
    }
    // ... map
}
```

`squirrel` substitui parâmetros para placeholders nomeados; sempre usa parametrização (sem SQL injection).

### sqlc — código gerado a partir de SQL

```sql
-- queries.sql
-- name: FindUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2;
```

```bash
sqlc generate
```

Gera funções Go tipadas a partir do SQL. Vantagens: queries estaticamente checadas contra schema, sem runtime parsing. Trade-off: setup adicional + recompilação.

---

## Transações

### Padrão básico

```go
func (r *DocumentRepository) MoveToOrg(ctx context.Context, docID, orgID string) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer func() {
        if cerr := tx.Rollback(); cerr != nil && !errors.Is(cerr, sql.ErrTxDone) {
            slog.Warn("rollback", "err", cerr)
        }
    }()

    if _, err := tx.ExecContext(ctx, `UPDATE documents SET organization_id = $1 WHERE id = $2`, orgID, docID); err != nil {
        return fmt.Errorf("update doc: %w", err)
    }
    if _, err := tx.ExecContext(ctx, `INSERT INTO audit_log (action, doc_id) VALUES ($1, $2)`, "moved", docID); err != nil {
        return fmt.Errorf("audit log: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }
    return nil
}
```

`defer tx.Rollback()` é seguro mesmo após `Commit` — `sql.ErrTxDone` é esperado.

### Unit of Work via interface

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(tx Tx) error) error
}

type Tx interface {
    UserRepository() user.Repository
    DocumentRepository() document.Repository
}
```

Use case usa `UnitOfWork` para coordenar repositórios em uma transação. Implementação concreta na infra.

### Isolamento

```go
tx, err := db.BeginTxx(ctx, &sql.TxOptions{
    Isolation: sql.LevelSerializable,
    ReadOnly:  false,
})
```

---

## Migrations

### Ferramentas comuns

| Ferramenta | Característica |
| ------------ | ---------------- |
| `golang-migrate/migrate` | Padrão da indústria, file-based, CLI rica |
| `pressly/goose` | SQL ou Go, migration por função |
| `ariga/atlas` | Schema-as-code, declarativo |
| GORM `AutoMigrate` | Conveniência, evitar em produção |

### golang-migrate

Estrutura:

```
migrations/
├── 0001_create_users.up.sql
├── 0001_create_users.down.sql
├── 0002_add_user_role.up.sql
└── 0002_add_user_role.down.sql
```

```bash
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" down 1
migrate create -ext sql -dir migrations -seq add_user_role
```

```sql
-- 0001_create_users.up.sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 0001_create_users.down.sql
DROP TABLE users;
```

### Aplicar no boot

```go
import "github.com/golang-migrate/migrate/v4"

m, err := migrate.New("file://migrations", cfg.DatabaseURL)
if err != nil {
    log.Fatal(err)
}
if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
    log.Fatal(err)
}
```

### Práticas seguras

| Recomendação | Motivo |
| -------------- | -------- |
| Evitar `AutoMigrate` em produção | Sem `down`, mudanças destrutivas implícitas |
| Toda migration tem `up` + `down` | Rollback seguro |
| Migrations idempotentes quando possível | `IF NOT EXISTS`, `ON CONFLICT` |
| Backfill em duas migrations | `ALTER ... NULL` → `UPDATE` → `ALTER ... NOT NULL` |
| Testar em staging antes de prod | Tempo de lock, dados |
| `CONCURRENTLY` para índices | Não bloquear escrita |

### Adicionar coluna NOT NULL com default

```sql
-- 0010_add_user_role.up.sql
-- Passo 1: adicionar nullable
ALTER TABLE users ADD COLUMN role VARCHAR(50);

-- Passo 2: backfill
UPDATE users SET role = 'user' WHERE role IS NULL;

-- Passo 3: tornar NOT NULL
ALTER TABLE users ALTER COLUMN role SET NOT NULL;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'user';
```

Em tabelas grandes, divida em múltiplas migrations e deploys para evitar lock prolongado.

### Remover coluna com segurança

1. Migration A — parar de escrever na coluna em código.
2. Deploy. Confirmar que nenhuma escrita ocorre.
3. Migration B — `DROP COLUMN`.

---

## Performance

### Selecionar colunas específicas

```go
// 🚫 SELECT * — traz JSONBs pesados
err := db.SelectContext(ctx, &users, `SELECT * FROM users WHERE org_id = $1`, orgID)

// ✅ Só o necessário
err := db.SelectContext(ctx, &users, `SELECT id, name, email FROM users WHERE org_id = $1`, orgID)
```

### Evitar N+1

```go
// 🚫 N+1
docs, _ := docRepo.FindByUser(ctx, userID)
for _, d := range docs {
    d.Template, _ = tmplRepo.FindByID(ctx, d.TemplateID)
}

// ✅ JOIN
const q = `
    SELECT d.id, d.title, t.id AS template_id, t.name AS template_name
    FROM documents d
    JOIN templates t ON t.id = d.template_id
    WHERE d.user_id = $1
`
```

Para batches grandes, considere `IN (...)` ou `ANY($1::uuid[])`:

```go
const q = `SELECT * FROM templates WHERE id = ANY($1::uuid[])`
err := db.SelectContext(ctx, &tmpls, q, pq.Array(templateIDs))
```

### Índices

Crie índices para colunas frequentemente usadas em `WHERE`/`ORDER BY`:

```sql
CREATE INDEX idx_documents_org_status ON documents (organization_id, status);
CREATE INDEX idx_documents_user_created ON documents (user_id, created_at DESC);
CREATE INDEX CONCURRENTLY idx_users_email_lower ON users (LOWER(email));
```

Use `EXPLAIN ANALYZE` para verificar uso.

### Paginação

Offset funciona para páginas iniciais; degrada em offsets grandes.

```go
// Offset (simples)
const q = `SELECT id, name FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`

// Keyset (escala)
const q = `
    SELECT id, name, created_at FROM users
    WHERE created_at < $1
    ORDER BY created_at DESC
    LIMIT $2
`
```

Keyset pagination escala melhor — passa o `created_at` da última linha em vez de offset.

### Connection pool

```go
db.SetMaxOpenConns(50)            // não abuse: PG default = 100 conexões totais
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(30 * time.Minute)
db.SetConnMaxIdleTime(5 * time.Minute)
```

Configure conforme infraestrutura (PgBouncer altera regras).

### Statement caching

`pgx` cacheia prepared statements automaticamente. Em `database/sql`, `db.PrepareContext` em queries muito chamadas.

### Sempre usar `Context`

```go
// 🚫 sem context — ignora cancelamento
db.Query(`...`)

// ✅ com context
db.QueryContext(ctx, `...`)
```

### Sempre fechar `rows`

```go
rows, err := db.QueryContext(ctx, q, args...)
if err != nil {
    return nil, err
}
defer rows.Close()  // ❗ obrigatório
```

Linter `sqlclosecheck` cobre.

---

## Anti-patterns

| Anti-pattern | Preferir |
| -------------- | ---------- |
| `AutoMigrate` em produção (GORM) | Migrations versionadas |
| String concat em SQL | Parametrização (`$1`, `?`) |
| `SELECT *` por reflexo | Colunas explícitas |
| ORM com lazy loading global | JOIN ou query separada |
| Entity como response DTO | Mapear para DTO |
| Esquecer `defer rows.Close()` | Sempre fechar |
| Conexão sem `Context` | Sempre `QueryContext` |
| `if err != nil { panic(err) }` | Wrap e propagar |
| `os.Exit` em código de lib | Retornar erro |

---

## Variações comuns

- **sqlc + pgx**: padrão moderno em apps que valorizam SQL puro com tipagem forte.
- **GORM**: aceitável em CRUDs simples, com cuidado para evitar N+1 e queries opacas.
- **ent**: schema-first, gera repositórios — encaixa bem em DDD.
- **Polyglot**: Postgres relacional + Mongo/Redis para casos específicos.

Adapte ao padrão do projeto.

---

## Checklist

- [ ] Entity em `internal/infra/<store>/` com tags `db:"..."`.
- [ ] Mapping linha-do-DB → entidade de domínio.
- [ ] Contrato do repositório no domínio; implementação na infra.
- [ ] Queries com parâmetros (`$1`, `$2`), sem concatenar strings.
- [ ] `defer rows.Close()` em queries que retornam linhas.
- [ ] `Context` propagado em todas as chamadas (`QueryContext`, `ExecContext`, etc.).
- [ ] Transações com `defer tx.Rollback()` antes de `Commit`.
- [ ] Migrations versionadas (`up` + `down`); sem `AutoMigrate` em prod.
- [ ] Índices para WHERE/ORDER BY frequentes.
- [ ] Paginação keyset em listas grandes.
- [ ] Connection pool configurado (`SetMaxOpenConns`, etc.).
- [ ] `sqlclosecheck` + `errcheck` no `golangci-lint`.
