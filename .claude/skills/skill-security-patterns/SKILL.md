---
name: skill-security-patterns
description: |
  Use esta skill quando o usuário pedir para "verificar segurança", "OWASP",
  "validar input", "configurar CSP", "headers de segurança", "autenticação segura",
  ou mencionar vulnerabilidades, XSS, CSRF, SQL injection ou proteção de dados.
  Cobre OWASP Top 10, validação de input, autenticação, headers de segurança, CSP.
model: sonnet
---

# Padrões de segurança

## Objetivo
Padrões de segurança web: OWASP Top 10, validação de input, autenticação, headers, CSP e proteção de dados.

## Quando usar
- Ao aplicar práticas OWASP Top 10.
- Ao configurar headers de segurança e CSP.
- Ao revisar autenticação e autorização.

## OWASP Top 10 (2021)

| # | Vulnerabilidade | Prevenção |
|---|-----------------|-----------|
| 1 | Broken Access Control | Verificar autorização em cada request |
| 2 | Cryptographic Failures | Criptografar dados sensíveis, TLS |
| 3 | Injection | Parametrizar queries, validar input |
| 4 | Insecure Design | Threat modeling, secure by design |
| 5 | Security Misconfiguration | Hardening, remover defaults |
| 6 | Vulnerable Components | Atualizar dependências |
| 7 | Auth Failures | MFA, rate limiting, sessões seguras |
| 8 | Data Integrity Failures | Verificar atualizações, CI/CD seguro |
| 9 | Logging Failures | Logs de segurança, alertas |
| 10 | SSRF | Validar URLs, whitelist de destinos |

---

## Validação de input

### Princípios

```typescript
const email = validateEmail(req.body.email)
if (!email) throw new ValidationError('Invalid email')
```

### Checklist

- [ ] Validar tipo (string, number, boolean).
- [ ] Validar formato (regex, schema).
- [ ] Validar tamanho (min, max length).
- [ ] Sanitizar HTML (prevenir XSS).
- [ ] Escapar por contexto (SQL, HTML, URL).
- [ ] Validar no servidor, mesmo quando há validação no cliente.

### Zod — exemplo

```typescript
import { z } from 'zod'

const UserSchema = z.object({
  email: z.string().email().max(255),
  name: z.string().min(1).max(100),
  age: z.number().int().min(0).max(150).optional(),
})

type User = z.infer<typeof UserSchema>

const validate = (input: unknown): User => UserSchema.parse(input)
```

---

## Autenticação

### Senhas

```typescript
import bcrypt from 'bcrypt'

const SALT_ROUNDS = 12

const hashPassword = async (password: string): Promise<string> =>
  bcrypt.hash(password, SALT_ROUNDS)

const verifyPassword = async (password: string, hash: string): Promise<boolean> =>
  bcrypt.compare(password, hash)
```

bcrypt e argon2 são escolhas comuns.

### Tokens JWT

```typescript
const tokenConfig = {
  algorithm: 'RS256', // assimétrico costuma ser preferível
  expiresIn: '15m',   // access token com curta duração
  issuer: 'app-name',
  audience: 'app-users',
}
```

Refresh tokens costumam ter validade maior e ser armazenados em cookie `httpOnly` ou no banco.

### Rate limiting

```typescript
const loginLimiter = {
  windowMs: 15 * 60 * 1000,
  max: 5,
  message: 'Too many login attempts',
}

const apiLimiter = {
  windowMs: 60 * 1000,
  max: 100,
}
```

---

## Autorização

### Verificar em cada request

```typescript
app.get('/users/:id', authenticate, authorize('read:user'), getUser)

const authorize = (permission: string) => (req, res, next) => {
  if (!req.user.permissions.includes(permission)) {
    return res.status(403).json({ error: 'Forbidden' })
  }
  next()
}
```

### Prevenir IDOR

```typescript
app.get('/orders/:id', async (req, res) => {
  const order = await db.orders.findById(req.params.id)
  if (order.userId !== req.user.id) {
    return res.status(403).json({ error: 'Forbidden' })
  }
  res.json(order)
})
```

---

## Headers de segurança

```typescript
const securityHeaders = {
  'X-Frame-Options': 'DENY',
  'X-Content-Type-Options': 'nosniff',
  'X-XSS-Protection': '1; mode=block',
  'Referrer-Policy': 'strict-origin-when-cross-origin',
  'Strict-Transport-Security': 'max-age=31536000; includeSubDomains; preload',
  'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
  'Content-Security-Policy': "default-src 'self'; script-src 'self'",
}
```

---

## Content Security Policy (CSP)

### Configuração básica

```
default-src 'self';
script-src 'self' https://trusted-cdn.com;
style-src 'self' 'unsafe-inline';
img-src 'self' data: https:;
font-src 'self';
connect-src 'self' https://api.example.com;
frame-ancestors 'none';
base-uri 'self';
form-action 'self';
```

### Diretivas importantes

| Diretiva | Propósito |
|----------|-----------|
| `default-src` | Fallback para as demais |
| `script-src` | Fontes de JavaScript |
| `style-src` | Fontes de CSS |
| `img-src` | Fontes de imagens |
| `connect-src` | URLs para fetch/XHR |
| `frame-ancestors` | Quem pode embeddar (anti-clickjacking) |
| `base-uri` | Restringe `<base>` |
| `form-action` | Destinos de forms |

### Nonces para inline scripts

```typescript
const nonce = crypto.randomBytes(16).toString('base64')
// CSP: script-src 'self' 'nonce-${nonce}'
```

---

## Cookies seguros

```typescript
const cookieOptions = {
  httpOnly: true,
  secure: true,
  sameSite: 'strict',
  maxAge: 3600000,
  path: '/',
  domain: '.example.com',
}

res.cookie('session', token, cookieOptions)
```

---

## Proteção de dados

### Dados sensíveis

```typescript
console.log('User login:', { email, password: '[REDACTED]' })

return { id: user.id, email: user.email, name: user.name }
```

### Secrets

```typescript
const API_KEY = process.env.API_KEY

if (!process.env.API_KEY) {
  throw new Error('API_KEY is required')
}
```

---

## SQL Injection

Queries parametrizadas são a principal defesa.

```typescript
// TypeORM query builder com parâmetros
const user = await userRepository
  .createQueryBuilder('user')
  .where('user.id = :id', { id: userId })
  .getOne()

// find com opções tipadas
const user = await userRepository.findOne({ where: { id: userId } })

// Interpolação direta é vulnerável — evitar
// const result = await db.query(`SELECT * FROM users WHERE id = '${userId}'`)
```

---

## Prevenção de XSS

```typescript
// Sanitizar antes de armazenar conteúdo que será renderizado
import sanitizeHtml from 'sanitize-html'
const safeContent = sanitizeHtml(userInput, {
  allowedTags: ['b', 'i', 'em', 'strong'],
  allowedAttributes: {},
})

// Garantir Content-Type: application/json em respostas JSON
```

---

## CORS

```typescript
const corsOptions = {
  origin: ['https://app.example.com', 'https://admin.example.com'],
  methods: ['GET', 'POST', 'PUT', 'DELETE'],
  allowedHeaders: ['Content-Type', 'Authorization'],
  credentials: true,
  maxAge: 86400,
}
```

Evite `origin: '*'` combinado com `credentials: true`.

---

## Checklist de segurança

### Input/Output
- [ ] Todos os inputs validados no servidor.
- [ ] Queries parametrizadas (SQL injection).
- [ ] Output escapado por contexto (XSS).
- [ ] Uploads validados (tipo, tamanho).

### Autenticação
- [ ] Senhas com hash seguro (bcrypt/argon2).
- [ ] Tokens com expiração curta.
- [ ] Rate limiting em login.
- [ ] MFA disponível quando fizer sentido.

### Autorização
- [ ] Verificação em cada endpoint.
- [ ] Sem IDOR.
- [ ] Princípio do menor privilégio.

### Infraestrutura
- [ ] HTTPS obrigatório.
- [ ] Headers de segurança configurados.
- [ ] CSP implementado.
- [ ] Cookies com flags seguras.
- [ ] CORS restrito.

### Dados
- [ ] Dados sensíveis criptografados.
- [ ] Secrets em variáveis de ambiente.
- [ ] Logs sem dados sensíveis.
- [ ] Backups criptografados.

---

## 🔹 Go: padrões de segurança em Go

OWASP Top 10 é universal. As bibliotecas e idiomas mudam.

### Validação de input (Go)

```go
import "github.com/go-playground/validator/v10"

type UserRequest struct {
    Email string `json:"email" validate:"required,email,max=255"`
    Name  string `json:"name"  validate:"required,min=1,max=100"`
    Age   int    `json:"age"   validate:"omitempty,gte=0,lte=150"`
}

var validate = validator.New()

func parseUser(body io.Reader) (UserRequest, error) {
    var req UserRequest
    if err := json.NewDecoder(body).Decode(&req); err != nil {
        return req, fmt.Errorf("decode: %w", err)
    }
    if err := validate.Struct(req); err != nil {
        return req, fmt.Errorf("validate: %w", err)
    }
    return req, nil
}
```

Alternativas: `ozzo-validation` (programático), validação manual via funções.

### Senhas

```go
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    return string(hash), err
}

func VerifyPassword(password, hash string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

Alternativa: `golang.org/x/crypto/argon2` para argon2id.

### Comparação em tempo constante

```go
import "crypto/subtle"

func compareTokens(provided, expected string) bool {
    return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
```

Use em comparação de tokens, HMACs, secrets — `==` é vulnerável a timing attacks.

### Tokens JWT

```go
import "github.com/golang-jwt/jwt/v5"

func sign(userID string, key *rsa.PrivateKey) (string, error) {
    claims := jwt.MapClaims{
        "sub": userID,
        "exp": time.Now().Add(15 * time.Minute).Unix(),
        "iss": "app-name",
        "aud": "app-users",
    }
    return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
}

func parse(tokenStr string, pub *rsa.PublicKey) (*jwt.Token, error) {
    return jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
        if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return pub, nil
    })
}
```

Sempre verificar o algoritmo (proteção contra `alg: none` e confusão RS↔HS).

### Rate limiting

#### `golang.org/x/time/rate`

```go
import "golang.org/x/time/rate"

var loginLimiter = rate.NewLimiter(rate.Every(3*time.Minute), 5)

func loginHandler(w http.ResponseWriter, r *http.Request) {
    if !loginLimiter.Allow() {
        http.Error(w, "too many attempts", http.StatusTooManyRequests)
        return
    }
    // ...
}
```

#### Por IP (com mapa de limiters)

```go
type ipLimiter struct {
    mu       sync.Mutex
    limiters map[string]*rate.Limiter
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
    l.mu.Lock()
    defer l.mu.Unlock()
    if lim, ok := l.limiters[ip]; ok {
        return lim
    }
    lim := rate.NewLimiter(10, 20)
    l.limiters[ip] = lim
    return lim
}
```

Para escala, use Redis + algoritmo token bucket distribuído (`go-redis-rate`).

### Autorização

#### Middleware de auth (net/http)

```go
type ctxKey string
const userCtxKey ctxKey = "user"

func RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        user, err := verifyToken(token)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), userCtxKey, user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func RequirePermission(perm string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, _ := r.Context().Value(userCtxKey).(*User)
            if !user.HasPermission(perm) {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

#### IDOR

```go
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
    user, _ := r.Context().Value(userCtxKey).(*User)
    orderID := chi.URLParam(r, "id")

    order, err := h.repo.FindByID(r.Context(), orderID)
    if err != nil || order == nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }

    if order.UserID != user.ID {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    writeJSON(w, http.StatusOK, order)
}
```

#### RBAC declarativo (`casbin`)

Para regras complexas com policies em arquivo/DB.

### Headers de segurança

#### Manualmente

```go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        h := w.Header()
        h.Set("X-Frame-Options", "DENY")
        h.Set("X-Content-Type-Options", "nosniff")
        h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
        h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'")
        next.ServeHTTP(w, r)
    })
}
```

#### Via `unrolled/secure`

```go
import "github.com/unrolled/secure"

secureMiddleware := secure.New(secure.Options{
    FrameDeny:             true,
    ContentTypeNosniff:    true,
    BrowserXssFilter:      true,
    ContentSecurityPolicy: "default-src 'self'",
})

handler := secureMiddleware.Handler(yourHandler)
```

### Cookies seguros

```go
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    token,
    Path:     "/",
    Domain:   ".example.com",
    MaxAge:   3600,
    Secure:   true,
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
})
```

### CSRF

```go
import "github.com/gorilla/csrf"

csrfMW := csrf.Protect(
    []byte("32-byte-secret"),
    csrf.Secure(true),
    csrf.SameSite(csrf.SameSiteStrictMode),
)
```

### SQL Injection

`database/sql` usa parametrização nativa:

```go
// Seguro: parâmetros via placeholders
row := db.QueryRowContext(ctx, `SELECT id, name FROM users WHERE id = $1`, userID)

// Seguro: ExecContext
res, err := db.ExecContext(ctx, `UPDATE users SET name = $1 WHERE id = $2`, name, id)

// VULNERÁVEL: nunca interpolar input
// q := fmt.Sprintf(`SELECT * FROM users WHERE id = '%s'`, userID)
```

### XSS / sanitização HTML

```go
import "github.com/microcosm-cc/bluemonday"

p := bluemonday.UGCPolicy()
safe := p.Sanitize(userInput)
```

`html/template` (stdlib) escapa por padrão; `text/template` NÃO escapa — use `html/template` para HTML.

### CORS

```go
import "github.com/rs/cors"

c := cors.New(cors.Options{
    AllowedOrigins:   []string{"https://app.example.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
})

handler := c.Handler(router)
```

Evitar `AllowedOrigins: []string{"*"}` com `AllowCredentials: true`.

### Secrets e crypto/rand

```go
import (
    "crypto/rand"
    "encoding/hex"
)

func generateToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return hex.EncodeToString(b), nil
}
```

`math/rand` NUNCA para tokens, IDs sensíveis ou nonces — não é criptograficamente seguro.

### Auditoria de dependências

```bash
govulncheck ./...   # CVEs alcançadas (oficial, mais preciso)
go list -m -u all   # módulos com atualização disponível
nancy sleuth        # alternativa de auditoria
```

### Checklist de segurança (Go)

- [ ] Input validado via `validator` ou refinamento manual.
- [ ] Senhas com `bcrypt`/`argon2`; comparações sensíveis com `subtle.ConstantTimeCompare`.
- [ ] JWT com algoritmo verificado e `exp` curto.
- [ ] Queries parametrizadas (`$1`, `$2`); nunca `fmt.Sprintf`.
- [ ] `crypto/rand` para tokens; nunca `math/rand`.
- [ ] Middleware de headers de segurança aplicado.
- [ ] CSRF para forms autenticados.
- [ ] CORS restrito (não `*` com credentials).
- [ ] Cookies com `HttpOnly`, `Secure`, `SameSite`.
- [ ] Rate limiting em endpoints sensíveis.
- [ ] `govulncheck ./...` no CI.
- [ ] `slog` sem campos sensíveis (use `slog.Any` com mascaramento explícito).
