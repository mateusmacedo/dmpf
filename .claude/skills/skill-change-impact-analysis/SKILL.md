---
name: skill-change-impact-analysis
description: |
  Análise de impacto antes de alterar código que cruza camadas arquiteturais. Útil quando:
  - Alterar tipos/contratos compartilhados entre Controllers, Services, Domain e Infra
  - Modificar cache ou estado de jobs de fila consumidos por múltiplos workers
  - Mudar guards/middlewares de autenticação ou autorização
  - Refatorar código entre camadas (domain → infra, service → controller)
  - Alterar schemas de banco (entities de ORM, migrations)
  Faz sentido ser sugerida automaticamente quando for detectada alteração que afeta múltiplas camadas.
model: sonnet
user-invocable: true
argument-hint: <descrição da alteração planejada>
---

# Análise de impacto — cross-layer

## Objetivo

Analisar o impacto de alterações antes de executá-las, reduzindo o risco de quebras entre camadas arquiteturais:

- Controllers/Routes (endpoints HTTP, validação de request)
- Services/Use Cases (regras de negócio, orquestração)
- Domain (entidades, value objects, erros de domínio, contratos)
- Repositories/Infra (repositórios, clients externos)
- DTOs/Validation (schemas de request/response)
- Consumers de fila (workers, processors)
- Middlewares/Guards (auth, rate limiting, logging)
- Database schema (entities, migrations)

## Quando usar

Antes de:

- Alterar tipos exportados usados por outras camadas.
- Modificar cache (chaves, TTL, estrutura).
- Alterar estado ou payload de jobs.
- Mover lógica entre `domain/`, `application/` e `infra/`.
- Mudar guards de autenticação ou autorização.
- Alterar entidades de ORM (geralmente exige migration).
- Renomear ou mover services/repositories.
- Alterar contratos de API.
- Mudar schemas de validação usados por múltiplos endpoints.

## Checklist de análise

### 1. Camadas afetadas

| Camada | Diretórios típicos | Afetada? |
|--------|--------------------|----------|
| Controllers/Routes | `controllers/`, `routes/`, `*.controller.ts` | [ ] |
| Services/Use Cases | `services/`, `application/`, `use-cases/` | [ ] |
| Domain | `domain/entities/`, `domain/errors/`, `domain/contracts/` | [ ] |
| Repositories | `infra/repositories/`, `*.repository.ts` | [ ] |
| DTOs/Validation | `dtos/`, `*.dto.ts`, `validators/` | [ ] |
| Consumers de fila | `consumers/`, `workers/`, `processors/` | [ ] |
| Middlewares/Guards | `middlewares/`, `guards/`, `*.guard.ts` | [ ] |
| Database schema | `entities/`, `migrations/` | [ ] |

A nomenclatura varia por projeto (Clean Architecture manual, modular NestJS, pipeline/step engine, etc.). Ajuste aos diretórios reais.

### 2. Tipos e contratos compartilhados

```
[ ] Tipo adicionado/removido/modificado?
[ ] Importadores atualizados?
[ ] DTOs que usam o tipo atualizados?
[ ] Schemas de validação atualizados?
```

Locais a verificar: `types.ts`, `import type { ... } from ...`, DTOs que referenciam o tipo, contratos de domínio (interfaces de repositório).

### 3. Estado compartilhado

```
[ ] Cache modificado (chave, TTL, estrutura)?
[ ] Consumers que leem o cache atualizados?
[ ] Jobs com payload alterado?
[ ] Workers que processam os jobs atualizados?
[ ] Schema de banco alterado?
[ ] Migration criada para a alteração?
```

### 4. Contratos de API

```
[ ] DTO de request alterado?
[ ] DTO de response alterado?
[ ] Código HTTP de resposta mudou?
[ ] Novo campo obrigatório (breaking change)?
[ ] Clientes da API atualizados?
```

### 5. Autenticação e autorização

```
[ ] Guard/middleware de auth modificado?
[ ] Nova permissão necessária?
[ ] Roles alterados?
[ ] Rota pública virou privada (ou vice-versa)?
```

### 6. Database schema

```
[ ] Entity alterada?
[ ] Migration criada e testada (up + down)?
[ ] Índices adicionados/removidos?
[ ] Relacionamentos alterados (FK, cascade)?
[ ] Queries existentes compatíveis?
```

### 7. Filas e workers

```
[ ] Payload de job alterado?
[ ] Producers atualizados?
[ ] Consumers atualizados?
[ ] Retries e dead-letter configurados?
[ ] Concurrency impactada?
```

## Formato de saída

```markdown
## Análise de impacto

**Alteração**: [descrição breve]

### Camadas afetadas
- Controllers/Routes: [sim/não] — [motivo]
- Services/Use Cases: [sim/não] — [motivo]
- Domain: [sim/não] — [motivo]
- Repositories/Infra: [sim/não] — [motivo]
- DTOs/Validation: [sim/não] — [motivo]
- Consumers de fila: [sim/não] — [motivo]
- Middlewares/Guards: [sim/não] — [motivo]
- Database schema: [sim/não] — [motivo]

### Riscos identificados
1. [risco + mitigação]

### Arquivos a modificar
1. [arquivo] — [o que mudar]

### Verificações pós-implementação
- [ ] Lint + format
- [ ] Typecheck
- [ ] Testes
- [ ] Imports atualizados em todos os consumidores
- [ ] Migrations testadas (up + down)
- [ ] Jobs com payload correto
```

## Atalho

Quando a alteração é trivial e fica restrita a uma camada, registrar:

```
Impacto: baixo (alteração isolada em [camada], sem dependências cross-layer).
```

## Integração

Esta skill pode ser executada pelo agente `change-impact-analysis` ou invocada manualmente.

---

## 🔹 Go: análise de impacto

Os checklists são os mesmos. As ferramentas e os pontos de atenção em Go diferem.

### Ferramentas para mapear dependências (Go)

- `go list -deps ./...` — todas as dependências transitivas.
- `go list -f '{{.ImportPath}}: {{.Imports}}' ./...` — imports por pacote.
- `go mod why <pacote>` — explica presença no `go.sum`.
- IDE / `gopls` — find references, find implementations.
- `go vet ./...` — análise estática built-in.
- `golangci-lint run` — agregador.

### Camadas afetadas (Go)

| Camada | Diretórios típicos |
|--------|--------------------|
| Handlers HTTP | `internal/<ctx>/http.go`, `internal/<ctx>/handler.go` |
| Usecases | `internal/<ctx>/usecase.go` |
| Domain | `internal/<ctx>/domain.go` (entities, interfaces) |
| Repositories | `internal/<ctx>/repository.go`, `repository_<db>.go` |
| Request/Response structs | `internal/<ctx>/dto.go` ou inline em `http.go` |
| Consumers | `internal/<ctx>/consumer.go` |
| Middlewares | `internal/shared/httputil/middleware.go` |
| Migrations | `internal/db/migrations/` (golang-migrate, atlas, goose) |

### Pontos específicos de Go

#### Tipos e contratos

```
[ ] Struct exportada teve campos alterados? (afeta JSON, embedding, literals externos)
[ ] Tags `json`/`db`/`validate` mudaram? (não aparece como erro de build)
[ ] Interface exportada ganhou método? (quebra implementadores)
[ ] Função exportada teve assinatura alterada? (callers quebram)
[ ] Tipo concreto usado em outras camadas? (impacto explícito via imports)
```

#### Estado compartilhado

```
[ ] Cache key/struct em Redis alterada? (consumers que decodificam)
[ ] Payload de job (asynq, river) alterado? (workers e producers)
[ ] Schema do banco alterado? (migration up + down rodam?)
[ ] Tags `db:"..."` mudaram? (sqlx/sqlc afetados)
```

#### Contratos de API

```
[ ] Request/response struct alterado?
[ ] Tags `json` em campos afetam clientes?
[ ] Status code retornado mudou?
[ ] Novo campo obrigatório (validate:"required")?
```

#### Database schema

```
[ ] Migration criada via golang-migrate / atlas / goose?
[ ] Migration testada forward e backward?
[ ] Queries (em sqlc, queries.sql) regeneradas?
[ ] Índices alinhados a queries existentes?
```

#### Concorrência

```
[ ] Mudança em estrutura compartilhada? (precisa de mutex/channel?)
[ ] go test -race passa após mudança?
[ ] Goroutines novas têm context para cancelamento?
```

### Verificações pós-implementação (Go)

- [ ] `gofmt -l .` sem diff
- [ ] `go vet ./...` sem warnings
- [ ] `golangci-lint run` passa
- [ ] `go build ./...` passa
- [ ] `go test ./... -race` passa
- [ ] `go mod tidy` não introduziu mudanças não relacionadas
- [ ] Migrations testadas em ambiente de teste
- [ ] `govulncheck ./...` se dependências mudaram
