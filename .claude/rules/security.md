---
paths:
  - "**/*.ts"
  - "**/*.go"
---

# Segurança

## Autenticação

- Manter o token de sessão fora do alcance do JavaScript do cliente (por exemplo, cookie `HttpOnly`) é um default seguro para aplicações web.
- Validar o token em camada centralizada (middleware ou guard), não em cada endpoint.

A implementação concreta depende do framework (middleware Express, guard NestJS, etc.) e do mecanismo de auth escolhido.

## Autorização

- Use um modelo explícito de permissões (por exemplo, RBAC ou ABAC), aplicado no backend.
- Não confie em verificações feitas apenas no frontend.

## Validação de input

- Valide na entrada (controller/route); não confie em dados do cliente.
- Ferramentas comuns: Zod, Valibot, class-validator. Use a adotada pelo projeto.
- Não construa queries SQL por concatenação de strings.

## Proteção contra ataques comuns

- **SQL injection**: queries parametrizadas.
- **Rate limiting**: mecanismo adequado em rotas públicas e endpoints sensíveis.
- **CORS**: origens permitidas explícitas; evite `*` em produção.

## Variáveis de ambiente

- Acesse variáveis de ambiente por um módulo de configuração centralizado em vez de ler `process.env` em vários pontos. Isso facilita validação e mocks em testes.
- Não commite `.env`, credenciais ou segredos.
- Segredos ficam em variáveis de ambiente do runtime (ou secret manager), não no código.

## Cabeçalhos de segurança

- Configure cabeçalhos HTTP de segurança (por exemplo, via `helmet` em Express ou middleware equivalente em outros frameworks).

---

## 🔹 Go: segurança

Os princípios são os mesmos; o que muda são as bibliotecas e algumas garantias da toolchain.

### Autenticação

- Cookies `HttpOnly` + `Secure` + `SameSite=Lax/Strict` continuam o default.
- JWT: `golang-jwt/jwt`. Sessões server-side: `gorilla/sessions` ou Redis-backed customizado.
- Validação centralizada em middleware de cada framework (`gin`, `echo`, `chi`, `fiber`).
- `bcrypt`/`argon2` via `golang.org/x/crypto/bcrypt` ou `golang.org/x/crypto/argon2` para hash de senha.

### Autorização

- RBAC/ABAC sem padrão dominante; libs como `casbin` cobrem ambos.
- Decisão sempre no backend; não confie em headers ou cookies de roles vindos do cliente.

### Validação de input

- `go-playground/validator` (tags em struct), `ozzo-validation` (DSL) ou validação manual no construtor da entidade.
- SQL injection: nunca concatene strings em queries — use placeholders (`db.QueryContext(ctx, "... WHERE id = $1", id)`). `database/sql` recusa multistatement por default em vários drivers.
- Para SQL dinâmico, use builders (`squirrel`, `goqu`) ou `sqlc` (gera código tipado).

### Proteção contra ataques comuns

- **SQL injection**: parametrização nativa do `database/sql`.
- **Rate limiting**: `golang.org/x/time/rate` (token bucket oficial), `httprate`, ou middleware do framework. Em produção, Redis-backed para distribuído.
- **CORS**: `rs/cors` ou middleware nativo do framework. Origens explícitas; `*` é proibitivo com credentials.
- **CSRF**: `gorilla/csrf` quando há cookies de sessão.
- **Path traversal**: `filepath.Clean` + verificação de prefixo antes de abrir arquivos vindos de input.
- **SSRF**: validar URLs de outbound; bloquear ranges privados (RFC 1918, link-local).

### Variáveis de ambiente

- Padrão: pacote `config` que lê env no startup, valida e expõe um struct tipado.
- `kelseyhightower/envconfig` ou `spf13/viper` são populares.
- Falhar rápido (`log.Fatal`) se config obrigatória estiver ausente.
- Não logue config; ou logue versão sanitizada.

### Cabeçalhos de segurança

- Sem equivalente exato a `helmet`. Aplicar manualmente via middleware:
  - `Strict-Transport-Security`
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Content-Security-Policy` (configurar por contexto)
  - `Referrer-Policy: no-referrer`
- `unrolled/secure` agrupa esses cabeçalhos em um middleware.

### Dependências

- `govulncheck` (oficial) é executado em CI. Detecta CVEs apenas em código *efetivamente alcançado*, não só na lista do `go.sum`.
- `dependabot`/`renovate` configurados no repositório para updates.
