# .claude/

Recursos de apoio a assistentes de código versionados neste template: regras de
conduta, skills de domínio e agentes especializados. Quem clona o template recebe
tudo isso pronto, sem depender de configuração pessoal na máquina.

Este índice é gerado a partir do conteúdo real do diretório. Ao adicionar ou
remover arquivos, atualize a tabela correspondente.

| Diretório | Itens | Conteúdo |
| --- | --- | --- |
| `rules/` | 12 | Regras de conduta e de qualidade |
| `skills/` | 26 | Skills de domínio (mais 11 arquivos em `references/`) |
| `agents/` | 49 | Agentes especializados |
| `commands/` | 8 | Comandos de workspace (Nx e utilitários) |

Convenções gerais do repositório ficam em `AGENTS.md`, na raiz. Skills de
workspace específicas de Nx ficam em `.agents/skills/`.

---

## Rules

Carregadas como contexto de conduta. Descrevem o que é esperado em cada área.

| Regra | Arquivo | Descrição |
| --- | --- | --- |
| CI e qualidade | `rules/ci-quality.md` | Ferramentas de CI, comandos de validação e pipeline de qualidade. |
| Padrões de código | `rules/code-patterns.md` | Sintaxe, naming, imports e convenções recomendadas. |
| Disciplina de comentários | `rules/comment-discipline.md` | Comente o porquê e não o quê; evita comentários redundantes, narração e código comentado. |
| Referências efêmeras | `rules/ephemeral-refs.md` | Cite apenas artefatos duráveis; evita apontar para planos e estados de pipeline não versionados. |
| Tratamento de erros | `rules/error-handling.md` | Hierarquia de erros, tratamento centralizado, logging e mensagens. |
| Organização de arquivos | `rules/file-organization.md` | Colocalização e separação em camadas. |
| Limites de tamanho | `rules/file-size-limits.md` | Heurísticas de tamanho para arquivos e funções. |
| Segurança Git | `rules/git-safety.md` | Hooks, force push seguro, revisão pré-commit e proteção das branches compartilhadas. |
| Performance | `rules/performance.md` | Banco de dados, cache, filas, N+1 e otimizações comuns. |
| Processo e qualidade | `rules/process-enforcement.md` | Quando exigir especificação e plano antes de implementar. |
| Segurança | `rules/security.md` | Autenticação, autorização, sanitização e variáveis sensíveis. |
| Convenções de teste | `rules/testing-conventions.md` | Estrutura, nomenclatura e cobertura dos testes. |

---

## Skills

Cada skill vive em `skills/<nome>/SKILL.md`. As skills de `skill-typescript` e
`skill-go` têm material complementar em `references/` (4 arquivos cada), e a
`c4-architecture` traz mais 3 arquivos em `references/` além de um `README.md`
próprio — é a única skill do índice com README interno. As skills
`c4-architecture` e `fill-pr-template-from-diff` não usam o prefixo `skill-` das
demais; ambas seguem a convenção do repositório de origem.

### Arquitetura

| Skill | Diretório | Descrição |
| --- | --- | --- |
| C4 Architecture | `skills/c4-architecture/` | Documentação de arquitetura com diagramas C4 (contexto, container, componente, deployment) em Mermaid. |
| Architecture Patterns | `skills/skill-architecture-patterns/` | Estrutura de pastas, separação de responsabilidades e trade-offs. |
| DDD | `skills/skill-ddd/` | Conceitos de Domain-Driven Design quando couberem. |
| Evolutionary Architecture | `skills/skill-evolutionary-architecture/` | Refatoração incremental, fitness functions e migração de código. |
| Change Impact Analysis | `skills/skill-change-impact-analysis/` | Análise de impacto antes de alterar código que cruza camadas. |

### Código e qualidade

| Skill | Diretório | Descrição |
| --- | --- | --- |
| Clean Code | `skills/skill-clean-code/` | Naming, funções pequenas, SOLID e code smells. |
| Code Standards | `skills/skill-code-standards/` | Convenções de TypeScript, formatação, imports e DTOs. |
| Code Review | `skills/skill-code-review/` | Checklist de review e comunicação de feedback. |
| Quality Checklist | `skills/skill-quality-checklist/` | Verificação pré-release de segurança, robustez e performance. |
| Bug Fix | `skills/skill-bug-fix/` | Isolamento de escopo, correção mínima e teste de regressão. |

### TypeScript e backend Node

| Skill | Diretório | Descrição |
| --- | --- | --- |
| TypeScript | `skills/skill-typescript/` | Convenções de tipagem de backend, com 4 referências complementares. |
| NestJS Patterns | `skills/skill-nestjs-patterns/` | Modules, DI, decorators, pipes, guards e exception filters. |
| TypeORM Patterns | `skills/skill-typeorm-patterns/` | Entities, repositories, migrations e query builder. |
| BullMQ Patterns | `skills/skill-bullmq-patterns/` | Queues, workers, jobs, retry e error handling. |
| Validation | `skills/skill-validation/` | Schemas, inferência de tipos e erros de validação. |

### Go

| Skill | Diretório | Descrição |
| --- | --- | --- |
| Go | `skills/skill-go/` | Convenções idiomáticas de backend Go, com 4 referências complementares. |
| Go DB Patterns | `skills/skill-go-db-patterns/` | `database/sql`, sqlx, sqlc, GORM, ent e migrations. |
| Go HTTP Patterns | `skills/skill-go-http-patterns/` | `net/http`, chi, gin, echo, fiber, middleware e DI. |
| Go Queue Patterns | `skills/skill-go-queue-patterns/` | asynq, river, machinery e watermill. |
| Go Testing | `skills/skill-go-testing/` | Table-driven tests, mocks e testcontainers-go. |

### Testes

| Skill | Diretório | Descrição |
| --- | --- | --- |
| Jest | `skills/skill-jest/` | Setup, matchers, mocks e testes assíncronos. |
| Unit e Integration Testing | `skills/skill-unit-integration-testing/` | Estrutura e escolha entre tipos de teste, agnóstico de framework. |

### Transversais

| Skill | Diretório | Descrição |
| --- | --- | --- |
| Error Handling | `skills/skill-error-handling/` | Erros customizados, middleware, filters e Result pattern. |
| Performance | `skills/skill-performance/` | Queries, cache, filas, pooling e event loop. |
| Security Patterns | `skills/skill-security-patterns/` | OWASP Top 10, validação de input e headers de segurança. |

### Workflow

| Skill | Diretório | Descrição |
| --- | --- | --- |
| Fill PR Template | `skills/fill-pr-template-from-diff/` | Preenche a descrição do PR a partir do diff da branch. |

> As skills de TypeORM e BullMQ cobrem stacks que este template **não** traz em
> `package.json`. Elas ficam disponíveis porque projetos derivados podem adotar
> essas tecnologias — trate-as como material de referência até que a stack exista
> de fato no seu projeto.

---

## Agents

Agentes especializados em `agents/<nome>.md`. Os nomes abaixo são os nomes reais
dos arquivos no disco.

### Revisão e diagnóstico

| Agente | Arquivo | Foco |
| --- | --- | --- |
| architect-reviewer | `agents/architect-reviewer.md` | Decisões de design, padrões arquiteturais e escolhas de tecnologia. |
| code-reviewer | `agents/code-reviewer.md` | Review abrangente de qualidade, segurança e boas práticas. |
| review | `agents/review.md` | Review de backend pós-implementação, focado em padrões do projeto. |
| qa-expert | `agents/qa-expert.md` | Estratégia de QA, plano de testes e métricas de qualidade. |
| refactoring-specialist | `agents/refactoring-specialist.md` | Transformação de código complexo ou duplicado preservando comportamento. |
| debugger | `agents/debugger.md` | Diagnóstico de bugs, causa raiz e análise de stack traces. |
| debug | `agents/debug.md` | Investigação de erros de runtime e comportamento inesperado. |
| change-impact-analysis | `agents/change-impact-analysis.md` | Impacto de alterações que cruzam camadas arquiteturais. |

### Backend, dados e APIs

| Agente | Arquivo | Foco |
| --- | --- | --- |
| backend-developer | `agents/backend-developer.md` | APIs server-side, microsserviços e arquitetura de backend. |
| node-specialist | `agents/node-specialist.md` | Aplicações, APIs e CLIs em Node.js. |
| typescript-pro | `agents/typescript-pro.md` | Type system avançado, generics e type-level programming. |
| golang-pro | `agents/golang-pro.md` | Concorrência, performance e padrões idiomáticos em Go. |
| sql-pro | `agents/sql-pro.md` | Otimização de queries, schemas e estratégias de índice. |
| database-administrator | `agents/database-administrator.md` | Performance, alta disponibilidade e disaster recovery. |
| api-designer | `agents/api-designer.md` | Design de REST/GraphQL, OpenAPI e versionamento. |
| api-documenter | `agents/api-documenter.md` | Documentação de API e portais interativos. |
| graphql-architect | `agents/graphql-architect.md` | Schemas, federação e performance de queries. |
| websocket-engineer | `agents/websocket-engineer.md` | Comunicação bidirecional em tempo real. |
| microservices-architect | `agents/microservices-architect.md` | Decomposição de monolitos e padrões de comunicação. |

### Infraestrutura, CI/CD e segurança

| Agente | Arquivo | Foco |
| --- | --- | --- |
| cloud-architect | `agents/cloud-architect.md` | Arquitetura de nuvem, migração e otimização de custo. |
| devops-engineer | `agents/devops-engineer.md` | Automação de infraestrutura, containers e deploy. |
| deployment-engineer | `agents/deployment-engineer.md` | Pipelines de CI/CD e automação de entrega. |
| docker-expert | `agents/docker-expert.md` | Build, otimização e segurança de imagens. |
| kubernetes-specialist | `agents/kubernetes-specialist.md` | Clusters, workloads e troubleshooting. |
| terraform-engineer | `agents/terraform-engineer.md` | Infrastructure as code e gestão de state. |
| platform-engineer | `agents/platform-engineer.md` | Plataformas internas de desenvolvimento e golden paths. |
| sre-engineer | `agents/sre-engineer.md` | SLO, error budget e redução de toil. |
| security-engineer | `agents/security-engineer.md` | Controles de segurança, threat modeling e compliance. |
| security | `agents/security.md` | Segurança de backend: validação, dependências e dados. |
| performance-engineer | `agents/performance-engineer.md` | Identificação e remoção de gargalos. |
| performance | `agents/performance.md` | Performance de backend Node.js: queries, cache e filas. |
| build-engineer | `agents/build-engineer.md` | Tempo de build e escala do build system. |
| tooling-engineer | `agents/tooling-engineer.md` | CLIs, geradores de código e extensões de IDE. |
| cli-developer | `agents/cli-developer.md` | Ferramentas de linha de comando e DX de terminal. |
| git-workflow-manager | `agents/git-workflow-manager.md` | Estratégias de branching e gestão de merge. |
| ci-monitor-subagent | `agents/ci-monitor-subagent.md` | Auxiliar do comando `monitor-ci`: status e detalhes de correção. |

### Testes e automação

| Agente | Arquivo | Foco |
| --- | --- | --- |
| test-automator | `agents/test-automator.md` | Frameworks de teste automatizado e integração com CI. |
| test | `agents/test.md` | Testes unitários, de integração e E2E de backend. |

### Documentação e texto

| Agente | Arquivo | Foco |
| --- | --- | --- |
| documentation-engineer | `agents/documentation-engineer.md` | Sistemas de documentação técnica e tutoriais. |
| readme-generator | `agents/readme-generator.md` | README construído a partir da realidade do repositório. |
| mermaid-diagram-specialist | `agents/mermaid-diagram-specialist.md` | Fluxogramas, diagramas de sequência e ERDs. |
| pt-reviewer | `agents/pt-reviewer.md` | Revisão de texto em português: acentuação, crase e confusáveis. |
| en-reviewer | `agents/en-reviewer.md` | Revisão de texto em inglês: ortografia e concordância. |
| design-bridge | `agents/design-bridge.md` | Tradução de um `DESIGN.md` em instruções de interface. |

### Orquestração e evolução

| Agente | Arquivo | Foco |
| --- | --- | --- |
| agent-organizer | `agents/agent-organizer.md` | Montagem de times de agentes e decomposição de tarefas. |
| multi-agent-coordinator | `agents/multi-agent-coordinator.md` | Coordenação de agentes concorrentes e estado distribuído. |
| context-manager | `agents/context-manager.md` | Estado compartilhado e sincronização de contexto. |
| knowledge-synthesizer | `agents/knowledge-synthesizer.md` | Extração de padrões a partir de interações anteriores. |
| legacy-modernizer | `agents/legacy-modernizer.md` | Migração incremental de sistemas legados. |

---

## Commands

| Comando | Arquivo |
| --- | --- |
| `fill-pr-template-from-diff` | `commands/fill-pr-template-from-diff.md` |
| `link-workspace-packages` | `commands/link-workspace-packages.md` |
| `monitor-ci` | `commands/monitor-ci.md` |
| `nx-generate` | `commands/nx-generate.md` |
| `nx-import` | `commands/nx-import.md` |
| `nx-plugins` | `commands/nx-plugins.md` |
| `nx-run-tasks` | `commands/nx-run-tasks.md` |
| `nx-workspace` | `commands/nx-workspace.md` |
