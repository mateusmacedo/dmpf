---
name: skill-code-standards
description: |
  Use esta skill quando o usuário pedir para "padronizar código", "aplicar convenções",
  "formatar código", "organizar imports", "seguir padrões do projeto" ou mencionar
  padrões TypeScript, formatação, convenções backend e padrões internos.
  Cobre padrões TypeScript, formatação, imports, DTOs, decorators e convenções.
  Para princípios gerais (SOLID, code smells), ver `skill-clean-code`.
model: opus
---

# Padrões de código

## Objetivo

Descrever convenções de código recomendadas. Os defaults abaixo costumam funcionar bem em projetos TS/JS de backend; adeque ao padrão vigente do projeto.

## Quando usar

- Ao criar ou editar código.
- Ao revisar PRs para garantir consistência.
- Ao alinhar convenções do time.

## Escopo

- Regras gerais valem para todo o código TS/JS.
- Se o projeto já tem padrão diferente, preserve o padrão local.

## Formatação

- Configurações de ponto e vírgula, aspas e indentação ficam a cargo do formatter (por exemplo, Prettier ou Biome). Siga o padrão já aplicado.
- Prefira código auto-explicativo; reserve comentários para decisões não óbvias.

## TypeScript (defaults comuns)

- Prefira `type` a `interface` quando não houver extensão/merge:

  ```ts
  type CreateUserInput = { name: string; email: string }
  ```

- Arrow functions com `const` no nível de módulo:

  ```ts
  export const fetchUser = async (id: string) => {}
  ```

- Use `import type` para imports exclusivamente de tipos:

  ```ts
  import type { User } from './types'
  import { createUser } from './utils'
  ```

- Em geral, prefira union types ou `as const` a `enum` (menor pegada em runtime e melhor com tree-shaking):

  ```ts
  type Status = 'pending' | 'active' | 'done'
  const ROLES = ['admin', 'user'] as const
  ```

## Imports

- Quando o projeto define alias de path (por exemplo, `@/` ou `~/`), use os aliases para imports absolutos:

  ```ts
  import { UserRepository } from '@/infra/repositories/UserRepository'
  import type { User } from '@/domain/entities/User'
  ```

## Backend

### Nomenclatura de DTOs

Um padrão prático é `<Ação><Entidade>Dto`:

```ts
type CreateUserDto = {
  name: string
  email: string
}

type UpdatePetitionDto = {
  title?: string
  content?: string
}

type ListDocumentsQueryDto = {
  page: number
  limit: number
  status?: string
}
```

Se o projeto adota outra convenção (`CreateUserRequest`, `CreateUserSchema`, etc.), mantenha.

### Decorators (quando usar framework baseado em decorators)

```ts
@Injectable()
export class UserService {
  constructor(private readonly userRepository: UserRepository) {}
}

@Controller('users')
export class UserController {
  @Get(':id')
  @UseGuards(AuthGuard)
  findOne(@Param('id') id: string) {}
}
```

### Middleware (ex.: Express)

```ts
const authMiddleware = (req: Request, res: Response, next: NextFunction) => {
  const token = req.headers.authorization?.replace('Bearer ', '')
  if (!token) return res.status(401).json({ error: 'Token ausente' })
  next()
}

const errorMiddleware = (err: Error, req: Request, res: Response, next: NextFunction) => {
  console.error(err)
  res.status(500).json({ error: 'Erro interno' })
}
```

## Checklist

- [ ] Formatação segue o padrão do projeto.
- [ ] TypeScript com `type`, `import type` e sem `enum` quando conveniente.
- [ ] Imports respeitam os aliases definidos.
- [ ] DTOs seguem a convenção do projeto.
- [ ] Decorators aplicados corretamente (quando o framework usa).
- [ ] Middlewares seguem a assinatura padrão do framework.

---

## 🔹 Go: padrões de código

A maioria das decisões de estilo em Go é imposta pela toolchain (gofmt, go vet, naming exportado por capitalização). Sobra menos para o usuário decidir — o que existe está abaixo.

### Formatação

- `gofmt`/`goimports` é **obrigatório** — não é opcional como Prettier. CI deve falhar se houver diff.
- `gofumpt` (gofmt mais estrito) é opcional, mas comum.
- Tabs (não espaços) — imposto pelo gofmt.

### Tipos e funções

- `type T struct{...}` declara struct; `type T = X` declara alias.
- Não há genéricos com defaults; constraints via `interface { ~int | ~string }` ou `comparable`.
- Funções com `func` no nível de pacote; closures atribuídas a `var` existem mas não são idiomáticas em escopo top-level.
- Receivers com 1-2 letras (`u *User`).

### Visibilidade

- `PascalCase` exportado, `camelCase` privado ao pacote. Não há `export`.
- `internal/` no path = visibilidade restrita ao módulo (garantia da toolchain).

### Imports

- Caminhos são module paths definidos em `go.mod` (`github.com/empresa/projeto/internal/user`).
- `goimports` agrupa em três blocos: stdlib, externos, internos do módulo.
- Aliases possíveis (`alias "longo/path"`) mas raros — preferir nome do pacote direto.

### Backend Go

#### DTOs / structs de request

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Role  string `json:"role,omitempty"`
}

type UpdatePetitionRequest struct {
    Title   *string `json:"title,omitempty"`
    Content *string `json:"content,omitempty"`
}

type ListDocumentsQuery struct {
    Page   int    `form:"page,default=1"`
    Limit  int    `form:"limit,default=20"`
    Status string `form:"status,omitempty"`
}
```

Naming variations comuns: `CreateUserRequest`, `CreateUserInput`, `CreateUserCmd`. Mantenha consistência no projeto.

#### Handlers (Gin / Echo / Chi / net/http)

```go
// net/http puro
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    // ...
}

// Gin
func (h *UserHandler) Create(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // ...
}
```

#### Middleware

Em Go, middleware é uma função que recebe `http.Handler` e retorna `http.Handler`:

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if token == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Frameworks (Gin, Echo, Chi) têm assinaturas próprias mas o conceito é o mesmo.

### Checklist (Go)

- [ ] `gofmt`/`goimports` aplicado (CI sem diff).
- [ ] `go vet ./...` sem warnings.
- [ ] `golangci-lint run` passa.
- [ ] Naming idiomático (acrônimos em maiúsculas, receivers curtos).
- [ ] Imports agrupados por `goimports`.
- [ ] Tags `json`/`db`/`validate` consistentes.
- [ ] Erros tratados; `errcheck` sem ignorados não justificados.
- [ ] `context.Context` como primeiro parâmetro em handlers/usecases.
