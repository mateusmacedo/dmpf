---
name: skill-code-review
description: |
  Use esta skill quando o usuário pedir para "revisar código", "code review",
  "revisar PR", "verificar qualidade", "analisar mudanças" ou mencionar feedback sobre código.
  Cobre checklist de review, categorias de feedback e comunicação construtiva.
  Para checklist pré-release, ver `skill-quality-checklist`.
argument-hint: "[escopo]"
model: opus
user-invocable: true
---

# Code Review

## Objetivo

Reunir boas práticas de code review: checklist, categorias de feedback e comunicação construtiva, com foco em bugs, segurança e manutenibilidade.

## Quando usar

- Ao revisar PRs, commits ou propostas de mudança.
- Para conduzir uma revisão estruturada.

---

## Workflow

### Revisão estruturada de um escopo

Escopo: **$ARGUMENTS**. Se não especificado, revise as alterações não commitadas (`git diff`).

#### Skills de referência

- `skill-code-standards` — regras de código.
- `skill-typeorm-patterns` — padrões de ORM.
- `skill-bullmq-patterns` — padrões de fila.
- `skill-nestjs-patterns` — padrões de framework.

#### Processo

**1. Identificar mudanças**

```bash
git diff              # alterações não commitadas
git diff --staged     # alterações staged
git diff HEAD~N       # últimos N commits
```

**2. Aplicar checklist**

**Código**

- [ ] Segue padrões do projeto.

**Contratos de API**

- [ ] DTOs de request/response definidos e validados.
- [ ] Códigos HTTP corretos para cada cenário.

**Repository Pattern**

- [ ] Repositórios seguem o contrato do domínio.
- [ ] Queries otimizadas (sem N+1, índices considerados).

**Hierarquia de erros**

- [ ] Erros de domínio lançados nos casos de uso.
- [ ] Tratamento centralizado (controller/middleware/handler global).
- [ ] Sem exposição de detalhes internos.

**Otimização de queries**

- [ ] Sem N+1.
- [ ] Índices para campos com filtro frequente.
- [ ] Joins e selects enxutos.

**Cobertura de testes**

- [ ] Suíte existente passa.
- [ ] Novos testes para funcionalidade nova.
- [ ] Lógica de negócio coberta.

**Autenticação e autorização**

- [ ] Guards/middlewares aplicados nas rotas.
- [ ] Permissões verificadas no nível adequado.

#### Formato de feedback

```markdown
### [arquivo:linha] Blocker — Título

**Problema:** descrição.

**Sugestão:**
\`\`\`ts
// código sugerido
\`\`\`
```

#### Severidades

- **Blocker** — deve corrigir antes do merge.
- **Warning** — seria melhor corrigir.
- **Sugestão** — opcional.

---

## Conhecimento base

### Filosofia

Objetivos do code review:

1. Detectar bugs antes da produção.
2. Garantir qualidade e manutenibilidade.
3. Compartilhar conhecimento no time.
4. Manter consistência no codebase.

Mindset:

- Revisar o código, não a pessoa.
- Ser específico e construtivo.
- Sugerir, não ordenar.
- Reconhecer o que está bem feito.

---

### Checklist de revisão

#### 1. Correção

- [ ] O código faz o que deveria?
- [ ] Edge cases tratados?
- [ ] Há bugs óbvios?
- [ ] Testes cobrem cenários importantes?
- [ ] Testes passando?

#### 2. Segurança

- [ ] Input de usuário validado?
- [ ] Queries parametrizadas?
- [ ] Output escapado quando cabível?
- [ ] Auth verificada?
- [ ] Dados sensíveis protegidos?
- [ ] Sem segredos no código?

#### 3. Performance

- [ ] Sem N+1?
- [ ] Loops desnecessários?
- [ ] Oportunidade de cache?
- [ ] Índices para filtros frequentes?

#### 4. Manutenibilidade

- [ ] Código legível?
- [ ] Nomes descritivos?
- [ ] Funções pequenas e focadas?
- [ ] Complexidade justificada?
- [ ] Sem duplicação desnecessária?

#### 5. Consistência

- [ ] Padrões do projeto respeitados?
- [ ] Estilo alinhado?
- [ ] Imports organizados?
- [ ] TypeScript strict?

#### 6. Documentação

- [ ] Código auto-explicativo onde possível?
- [ ] Comentários onde realmente ajudam?
- [ ] README atualizado se necessário?
- [ ] Breaking changes documentados?

---

### Categorias de feedback

#### Blocker (deve corrigir)

Issues que impedem o merge:

- Bugs que quebram funcionalidade.
- Vulnerabilidades.
- Regressões.
- Testes falhando.
- Violações graves de padrão.

Formato:

```
**Blocker**: [descrição]

[impacto]

Sugestão:
```código sugerido```
```

#### Should Fix (seria melhor corrigir)

- Code smells significativos.
- Problemas de performance potenciais.
- Violações menores.
- Testes faltando para casos importantes.

#### Sugestão

- Refatorações que melhoram legibilidade.
- Abordagens alternativas.
- Simplificações.

Formato:

```
**Sugestão**: [descrição]

[benefício]

Opcional — fique à vontade para manter a abordagem atual.
```

#### Pergunta

Dúvidas genuínas (entender decisões, clarificar intenção, aprender).

#### Elogio

Reconhecer soluções elegantes, testes bem estruturados, etc.

---

### Comunicação efetiva

Evitar:

- Comentários vagos ("isso está confuso").
- Comentários pessoais ("você sempre faz assim").
- Imperativos sem justificativa ("mude para X").
- Instruções sem explicação ("adicione um índice").

Preferir:

- Específico: "A função X está fazendo muitas coisas. Considere extrair a validação."
- Sobre o código: "Esse padrão pode causar N+1 no loop. Ver: [link]."
- Sugestão: "Que tal usar eager loading aqui?"
- Com justificativa: "Sugiro um índice composto em (user_id, status) para evitar full scan nessa query."

Frases úteis:

- Sugestões: "Que tal...?", "Considere...", "Uma alternativa seria...".
- Dúvidas: "Estou curioso sobre...", "Poderia explicar...?".
- Elogios: "Boa solução para...", "Abordagem elegante".

---

### O que deixar para linters/formatters

- Formatação e estilo.
- Ordenação de imports.
- Espaços e indentação.
- Aspas e trailing commas.

---

### Template de PR Description

```markdown
## O que mudou

[Descrição breve]

## Por quê

[Contexto e motivação]

## Como testar

1. [Passo 1]
2. [Passo 2]
3. [Resultado esperado]

## Checklist

- [ ] Testes adicionados/atualizados
- [ ] Documentação atualizada (se necessário)
- [ ] Breaking changes documentados
- [ ] Self-review realizado

## Issues relacionadas

Closes #123
```

---

### Métricas (referência)

Métricas frequentemente consideradas saudáveis:

- Tempo até primeira review: < 24h.
- Ciclos de review: 1 a 3.
- Tamanho do PR: idealmente pequeno e focado.
- Comentários por PR: alguns, suficientes para sinalizar qualidade.

Sinais de alerta:

- PRs muito grandes.
- Muitos ciclos de review.
- Reviews demorados.
- Aprovação sem leitura (rubber stamping).
- Só comentários negativos.

Os números exatos dependem do time.

---

### Anti-padrões

| Evitar | Preferir |
| -------- | ---------- |
| Rubber stamping | Review genuíno |
| Bloquear por estilo | Automatizar com linters |
| Comentários vagos | Feedback específico |
| Focar só no negativo | Reconhecer o positivo |
| PRs enormes | PRs pequenos e focados |
| Review demorado | Revisar em janela razoável |
| Discussões infinitas | Levar para conversa síncrona se necessário |

---

## 🔹 Go: code review

Os checklists e a comunicação são universais. Pontos específicos do ecossistema:

### Skills de referência (Go)

- `skill-go` — convenções de código.
- `skill-go-db-patterns` — padrões de banco.
- `skill-go-queue-patterns` — padrões de fila.
- `skill-go-http-patterns` — padrões HTTP.

### Checklist (acréscimos para Go)

#### Correção (Go)

- [ ] `errcheck` não acusa erros ignorados.
- [ ] `go vet ./...` sem warnings.
- [ ] `golangci-lint run` passa.
- [ ] `go test -race ./...` passa.
- [ ] Type assertions usam `, ok` quando o tipo não é garantido.
- [ ] Type switches com case `default` ou exhaustive.

#### Segurança (Go)

- [ ] Queries parametrizadas (`db.QueryContext` com `$1`, `$2`).
- [ ] Sem `crypto/md5` ou `crypto/sha1` para auth.
- [ ] `crypto/rand` (não `math/rand`) para tokens/IDs sensíveis.
- [ ] `bcrypt`/`argon2` para senhas; `subtle.ConstantTimeCompare` para comparações sensíveis.
- [ ] `govulncheck` sem CVEs alcançadas.

#### Performance (Go)

- [ ] Sem N+1 (preload em GORM, JOIN em SQL puro).
- [ ] `defer rows.Close()` em queries.
- [ ] `context.Context` propagado.
- [ ] Goroutines com `context` para cancelamento.
- [ ] `sync.Pool`, `strings.Builder` em hot paths quando relevante.

#### Manutenibilidade (Go)

- [ ] Nomes idiomáticos (`UserID`, não `UserId`; `ctx`, não `context`).
- [ ] Receivers consistentes em métodos do mesmo tipo.
- [ ] Interfaces no consumidor, não no implementador (quando faz sentido).
- [ ] Funções pequenas; `funlen`/`gocyclo` em limites.

#### Consistência (Go)

- [ ] `gofmt`/`goimports` aplicado.
- [ ] Tags de struct (`json`, `db`, `validate`) consistentes no projeto.
- [ ] Import groupings em três blocos (stdlib, externos, internos).

#### Documentação (Go)

- [ ] Funções/tipos exportados têm comentário godoc iniciando pelo nome.
- [ ] Pacote tem comentário em um arquivo `doc.go` ou no `package` statement.
- [ ] Examples (`func ExampleFoo`) quando ajudam.

### O que deixar para linters (Go)

- Formato (`gofmt`/`goimports`).
- Imports não usados (`go vet`).
- Erros descartados (`errcheck`).
- Funções/imports não usados (`unused`, `unparam`).
- Complexidade (`gocyclo`, `gocognit`, `funlen`).
- Race conditions (`go test -race`).

### Comandos para revisão

```bash
git diff                       # alterações não commitadas
git diff --staged              # staged
go vet ./...                   # análise estática
golangci-lint run              # agregador
go build ./...                 # typecheck implícito
go test ./... -race            # testes + race detector
govulncheck ./...              # CVEs alcançadas
```
