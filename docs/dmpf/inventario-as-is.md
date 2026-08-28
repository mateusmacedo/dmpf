<!-- ephemeral-refs-ok-file: baseline candidato FND-01 — cita evidências em docs/specs -->
# Inventário AS-IS — DMPF Foundation (FND-01)

> **Status:** promovido para revisão (**baseline candidato** — aprovação
> Plataforma/Arquitetura em PR ainda pendente)
> **Draft de origem:** rascunho local homônimo, fora do versionamento — não citável como fonte
> **Spec:** [SPEC-K9H204F1](../specs/SPEC-K9H204F1-dmpf-inventario-as-is.md) /
> [ARQ-438](https://lider-cap.atlassian.net/browse/ARQ-438)
> **Data da consolidação:** 2026-08-14
> **Responsável pela consolidação:** agente (Cursor) sob pipeline SPEC-K9H204F1
> **Convenção epistêmica:** `Fato` | `Inferência` | `Lacuna`
> **Defaults de rastreabilidade:** salvo coluna própria, **Owner** =
> `owner não identificado`; **Data** = data do levantamento do corte na
> matriz §0 (**2026-08-13**), salvo indicação em contrário.

## 0. Matriz canônica de evidências

Regra de dedupe: síntese agrega por **repositório** (`repo@commit`). Quando há
dois cortes no mesmo `repo@commit`, o consolidado cita ambos como fontes e
**não** conta o padrão duas vezes no denominador “repositórios com X”.

| # | Repositório | Commit | Escopo do corte | Fonte |
|---|-------------|--------|-----------------|-------|
| 1 | `backoffice-procap-services` | `fab7c8ec` | apps/api, apps/migration, libs | [inventario-as-is-backoffice-procap-services.md](../specs/SPEC-K9H204F1/inventario-as-is-backoffice-procap-services.md) |
| 2 | `golibs` | `9106941` | packages/* (kernels Go; sem app de produção) | [inventario-as-is-golibs.md](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md) |
| 3 | `rendafacil-bff` | `e345066` | src/ (BFF Nest) | [inventario-as-is-rendafacil-bff.md](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-bff.md) |
| 4 | `rendafacil-services` | `78886b1` | apps/*, pkg/* | [inventario-as-is-rendafacil-services.md](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-services.md) |
| 5 | `telesena-monorepo` | `4a9c659` | apps + libs Nest | [inventario-as-is-telesena-monorepo.md](../specs/SPEC-K9H204F1/inventario-as-is-telesena-monorepo.md) |
| 6a | `shared-ro-sync-services` | `82fae7f` | apps/** + libs/** (corte apps-libs; Node detalhado em 6b) | [inventario-shared-ro-sync-services-apps-libs.md](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-apps-libs.md) |
| 6b | `shared-ro-sync-services` | `82fae7f` | libs/node | [inventario-shared-ro-sync-services-libs-node.md](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-libs-node.md) |
| 7 | `shared-titulos-services` | `a11348ac` | apps + libs (híbrido) | [inventario-shared-titulos-services-apps-libs.md](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |
| 8a | `telesena-ativavel-services` | `d7d7f0f8` | apps | [inventario-telesena-ativavel-services-apps.md](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps.md) |
| 8b | `telesena-ativavel-services` | `d7d7f0f8` | apps + shared | [inventario-telesena-ativavel-services-apps-shared.md](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps-shared.md) |
| 9a | `telesena-live-services` | `dd50a92` | apps + libs | [inventario-telesena-live-services-apps-libs.md](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md) |
| 9b | `telesena-live-services` | `dd50a92` | libs | [inventario-telesena-live-services-libs.md](../specs/SPEC-K9H204F1/inventario-telesena-live-services-libs.md) |
| 10 | `telesena-titulos-services` | `0cbfdb4` | apps/api + libs/apm | [inventario-telesena-titulos-services-apps-libs.md](../specs/SPEC-K9H204F1/inventario-telesena-titulos-services-apps-libs.md) |

**Checagem:** 13 relatórios | 10 repositórios únicos | 3 repos com corte duplo
(`shared-ro-sync-services`, `telesena-ativavel-services`, `telesena-live-services`).

**Denominadores usados neste documento:**
- **D10** = 10 repositórios únicos da matriz.
- **D9serv** = 9 repositórios com serviço/app implantável (`golibs` excluído — só kernels).
- **SQS runtime (serviços)** = subset de D9serv com producer/consumer SQS no código de app.

Prompt de coleta: [prompt-inventario-repositorio.md](../specs/SPEC-K9H204F1/prompt-inventario-repositorio.md).

---

## 1. Padrões vigentes

> Mensageria, contratos, UoW/transação e observabilidade — síntese cross-cutting.
> Levantamento dos cortes: 2026-08-13. Owner default: `owner não identificado`.

### 1.1 Mensageria

| Padrão observado | Repos | Estado | Fontes |
|------------------|-------|--------|--------|
| AWS SQS em apps/serviços (runtime) | 5/9serv: `rendafacil-services`, `shared-ro-sync-services`, `shared-titulos-services`, `telesena-ativavel-services`, `telesena-live-services` | `Fato` | [rendafacil-services](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-services.md), [ro-sync](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-apps-libs.md), [titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md), [ativavel](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps.md), [live](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md) |
| Adapters SQS em biblioteca (sem topologia de produção) | `golibs` (`gocqrs` + `goservice`) | `Fato` | [golibs](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md) |
| SNS → SQS (fan-out com FilterPolicy) | `shared-ro-sync-services` | `Fato` | [ro-sync](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-apps-libs.md) |
| Kernel Watermill (AMQP/SQS/SQL/in-memory) | `golibs` (`gocqrs`) | `Fato` | [golibs](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md) |
| Pacote npm `@lidercap-apps/message-broker` | `telesena-ativavel-services`, `telesena-live-services` | `Fato` | [ativavel-shared](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps-shared.md), [live-libs](../specs/SPEC-K9H204F1/inventario-telesena-live-services-libs.md) |
| Sem broker no código do escopo | 4/9serv: `backoffice-procap-services`, `rendafacil-bff`, `telesena-monorepo`, `telesena-titulos-services` | `Fato` | [procap](../specs/SPEC-K9H204F1/inventario-as-is-backoffice-procap-services.md), [rendafacil-bff](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-bff.md), [telesena-monorepo](../specs/SPEC-K9H204F1/inventario-as-is-telesena-monorepo.md), [telesena-titulos](../specs/SPEC-K9H204F1/inventario-telesena-titulos-services-apps-libs.md) |
| Kafka client versionado | **0/D10** (só menção README em `golibs`) | `Lacuna` / `Fato` de ausência | [golibs](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md); ausência nos demais cortes da §0 |
| Outbox / inbox nomeados | **1/D10** (`shared-titulos-services`) | `Fato` | [titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |
| Publish dentro de callback de TX | `rendafacil-services` | `Fato` | [rendafacil-services](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-services.md) |

`Inferência` (alta): entre **serviços implantáveis**, a mensageria assíncrona observada é **SQS-cêntrica**; Kafka não é prática vigente nos repos inventariados.

### 1.2 Contratos (wire)

| Padrão observado | Cobertura | Estado | Fontes |
|------------------|-----------|--------|--------|
| HTTP via Swagger/OpenAPI | serviços HTTP: procap, rendafacil-bff, rendafacil-services, telesena-monorepo, ativavel, live, titulos-services, shared-titulos | `Fato` | [procap](../specs/SPEC-K9H204F1/inventario-as-is-backoffice-procap-services.md), [rendafacil-bff](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-bff.md), [rendafacil-services](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-services.md), [telesena-monorepo](../specs/SPEC-K9H204F1/inventario-as-is-telesena-monorepo.md), [ativavel](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps.md), [live](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md), [telesena-titulos](../specs/SPEC-K9H204F1/inventario-telesena-titulos-services-apps-libs.md), [shared-titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |
| Envelope JSON ad hoc para filas | repos com SQS runtime (§1.1) | `Fato` | mesmos links SQS runtime |
| `.proto` / Buf / Schema Registry / AsyncAPI | **0/D10** | `Lacuna` | ausência registrada nos 13 cortes da §0 |
| Contract tests (exceção) | `telesena-monorepo` soft-bff + `@telesena-monorepo/backend-contracts` | `Fato` | [telesena-monorepo](../specs/SPEC-K9H204F1/inventario-as-is-telesena-monorepo.md) |

`Inferência` (alta): contratos assíncronos **não** são schema-first; evolução observada = tipagem TS/Go + Swagger HTTP.

### 1.3 Unidade de trabalho / transação / outbox-inbox

| Padrão observado | Exemplos | Estado | Fontes |
|------------------|----------|--------|--------|
| TX local (GORM / pgx / Prisma / TypeORM / mssql) | procap, rendafacil-services, ativavel, live, telesena-monorepo auth-api, telesena-titulos | `Fato` | cortes respectivos na §0 |
| Transactional Outbox + inbox nomeados | `shared-titulos-services` (`backoffice_outbox`, `inbound_event`) | `Fato` | [titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |
| Sem outbox/inbox nomeados | 9/D10 (todos exceto shared-titulos) | `Lacuna` | cortes §0 ≠ titulos |
| Efeitos async via stored procedure na mesma TX | `telesena-monorepo` auth-api | `Fato` | [telesena-monorepo](../specs/SPEC-K9H204F1/inventario-as-is-telesena-monorepo.md) |
| Idempotência pontual (unique/guards/ADR) | live, ro-sync, shared-titulos | `Fato` | [live](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md), [ro-sync](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-apps-libs.md), [titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |

### 1.4 Observabilidade

| Stack observada | Repos | Estado | Fontes |
|-----------------|-------|--------|--------|
| OpenTelemetry | `golibs`, `backoffice-procap-services`, `shared-ro-sync` libs/node | `Fato` | [golibs](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md), [procap](../specs/SPEC-K9H204F1/inventario-as-is-backoffice-procap-services.md), [libs-node](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-libs-node.md) |
| New Relic e/ou Elastic APM | rendafacil-*, telesena-*, ativavel, live, shared-titulos, telesena-titulos | `Fato` | cortes Nest/Go de produto na §0 |
| Pino / nestjs-pino / gologger | procap, golibs, BFFs/APIs Nest inventariadas | `Fato` | cortes §0 com stack de log |
| Prometheus `/metrics` | `backoffice-procap-services`; worker entrega em `shared-titulos-services` | `Fato` | [procap](../specs/SPEC-K9H204F1/inventario-as-is-backoffice-procap-services.md), [titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |
| Trace headers em mensagem SQS | `telesena-live-services` | `Fato` | [live](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md) |

`Lacuna`: padronização única OTel vs NR/Elastic **não** aparece no universo inventariado.

---

## 2. Libs e componentes

### 2.1 Go (kernels / monorepos)

| Componente | Repo | Papel AS-IS | Owner | Estado |
|------------|------|-------------|-------|--------|
| `gocqrs` (Watermill) | `golibs` | adapters AMQP/SQS/SQL/in-memory | `owner não identificado` | `Fato` — [golibs](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md) |
| `goservice` SQS (SDK v1) | `golibs` | segundo caminho SQS, paralelo ao `gocqrs` | `owner não identificado` | `Fato` |
| `godata` / `gotelemetry` / `gologger` / `goresilience` | `golibs` | persistência, OTel+NR, logs, resiliência | `owner não identificado` | `Fato` |
| Workers + contracts Go | `shared-ro-sync-services` | sync RO Postgres→SNS/SQS→MSSQL | `owner não identificado` | `Fato` |
| Apps Go (pauta/estoque/etc.) | `shared-titulos-services` | domínio títulos/filantropia | `owner não identificado` | `Fato` |
| API Gin + libs telemetry | `backoffice-procap-services` | backoffice Procap | `owner não identificado` | `Fato` |
| API Fiber + `pkg/sqs` | `rendafacil-services` | afiliados Renda Fácil | `owner não identificado` (README lista e-mails; sem CODEOWNERS) | `Fato` |

### 2.2 TypeScript / Nest / Node

| Componente | Repo | Papel AS-IS | Owner | Estado |
|------------|------|-------------|-------|--------|
| `@lidercap-apps/message-broker` | consumido por ativavel/live | cliente SQS compartilhado | `owner não identificado` *neste inventário* (pacote fora do corte) | `Fato` de uso |
| `@lideranca-sites/node-*` (17 libs) | `shared-ro-sync-services` libs/node | ports/adapters, telemetry, logger | `owner não identificado` | `Fato` — [libs-node](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-libs-node.md) |
| `@telesena-monorepo/backend-*` | `telesena-monorepo` | auth, contracts, health, http-client, observability | `owner não identificado` | `Fato` |
| `shared/core` + `shared/infra` | `telesena-ativavel-services` | ports/queue + Prisma/TypeORM | `owner não identificado` (author em package.json) | `Fato` |
| libs Nest (`aws`, `apm`, `database`, …) | `telesena-live-services` | SQS, APM, TypeORM | `owner não identificado` | `Fato` |
| BFF Nest | `rendafacil-bff` | fachada HTTP | `owner não identificado` | `Fato` |
| API Nest + `libs/apm` | `telesena-titulos-services` | títulos HTTP/MSSQL | `owner não identificado` (author em package.json) | `Fato` |

### 2.3 Ownership formal

| Achado | Cobertura | Estado | Owner | Data |
|--------|-----------|--------|-------|------|
| Arquivo `CODEOWNERS` | **0/D10** | `Lacuna` / `Fato` de ausência | `owner não identificado` | 2026-08-13 |
| Owner substituto em manifesto | `package.json` author / README autores (pontual) | `Fato` | conforme arquivo | 2026-08-13 |
| Política deste documento | item sem dono nomeado → **`owner não identificado`** | convenção FND-01 | consolidação | 2026-08-14 |

`Fato`: CODEOWNERS **não** está disponível como fonte de ownership nos 10 repositórios inventariados.

---

## 3. NFRs e restrições

> Organizacionais e técnicas **observadas** — descritivo; sem prescrever TO-BE.

### 3.1 Transporte e cloud

| Restrição / prática | Evidência | Estado | Owner | Data |
|---------------------|-----------|--------|-------|------|
| AWS SQS é o broker dominante em serviços com mensageria | 5/9serv (§1.1) | `Fato` | `owner não identificado` | 2026-08-13 |
| SNS usado como fan-out para sync RO | `shared-ro-sync-services` | `Fato` | `owner não identificado` | 2026-08-13 |
| Kafka **não** é stack vigente nos cortes | 0 clients versionados / D10 | `Lacuna` / ausência | `owner não identificado` | 2026-08-13 |
| Dual DB Postgres + SQL Server legado | shared-titulos, ativavel, rendafacil integrador, ro-sync | `Fato` | `owner não identificado` | 2026-08-13 |
| HTTP síncrono cobre fatia relevante | procap, rendafacil-bff, telesena-monorepo, telesena-titulos | `Fato` | `owner não identificado` | 2026-08-13 |

### 3.2 Segurança e compliance (quando evidenciado)

| Achado | Cobertura | Estado | Owner | Data |
|--------|-----------|--------|-------|------|
| Semgrep / scanners em CI (parcial) | procap, telesena-titulos (workflows ativos limitados) | `Fato` pontual | `owner não identificado` | 2026-08-13 |
| Política org de segurança centralizada | **não medido** neste FND-01 | `Lacuna` | `owner não identificado` | 2026-08-13 |
| Secrets/IAM detalhados | fora do corte profundo da maioria dos relatórios | `Lacuna` | `owner não identificado` | 2026-08-13 |

### 3.3 Gaps vs. constraints DMPF (descritivo)

| Constraint DMPF (P0) | AS-IS observado | Estado | Owner | Data |
|----------------------|-----------------|--------|-------|------|
| Domínio sem I/O | Distâncias documentadas (ex.: ativavel `domain` importa Prisma/broker; live `domain` com DTOs Swagger) | `Fato` de distância | `owner não identificado` | 2026-08-13 |
| Protobuf só no wire | Protobuf **ausente**; wire = JSON/Swagger | `Fato` | `owner não identificado` | 2026-08-13 |
| At-least-once + efeitos idempotentes | **Heterogêneo** — ver matriz abaixo (não generalizar “via SQS”) | `Fato` / `Inferência` | `owner não identificado` | 2026-08-13 |
| Kernels Go/TS fora do épico normativo | `golibs` e `libs/node` existem como baseline de adapters | `Fato` | `owner não identificado` | 2026-08-13 |

#### 3.3.1 Semântica de entrega / idempotência (serviços com SQS)

| Repo | Transporte | Ack / delete observado | Idempotência de efeito | Estado |
|------|------------|------------------------|------------------------|--------|
| `shared-titulos-services` | SQS + outbox/inbox | worker lê outbox após commit | ADR/estratégias documentadas | `Fato` — [titulos](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md) |
| `shared-ro-sync-services` | SNS→SQS | consumers com upsert | idempotência por upsert | `Fato` — [ro-sync](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-apps-libs.md) |
| `telesena-live-services` | SQS | delete após sucesso; retry por visibility | unique index / guards parciais | `Fato` — [live](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md) |
| `telesena-ativavel-services` | SQS | poll→process→delete | evidência registra lacuna de idempotência no consumer | `Fato` / `Lacuna` — [ativavel](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps.md) |
| `rendafacil-services` | SQS | delete-on-error pode perder mensagem | publish-in-TX documentado; idempotência não garantida E2E | `Fato` — [rendafacil-services](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-services.md) |

---

## 4. Métricas atuais

### 4.1 Instrumentação existente

| Sinal | Onde | Estado | Owner | Data |
|-------|------|--------|-------|------|
| APM New Relic e/ou Elastic | serviços Nest/Go de produto na §0 | `Fato` | `owner não identificado` | 2026-08-13 |
| OpenTelemetry SDK | `golibs`, procap, libs/node ro-sync | `Fato` | `owner não identificado` | 2026-08-13 |
| Prometheus `/metrics` | procap; worker entrega em `shared-titulos` | `Fato` | `owner não identificado` | 2026-08-13 |
| Logs estruturados (Pino/Logrus) | cortes com stack de log na §1.4 | `Fato` | `owner não identificado` | 2026-08-13 |
| Trace headers em mensagem SQS | `telesena-live-services` | `Fato` | `owner não identificado` | 2026-08-13 |

### 4.2 Lacunas explícitas (`não medido`)

| Lacuna | Escopo | Estado | Owner | Data |
|--------|--------|--------|-------|------|
| Dashboards SRE/org nomeados e URLs | D10 | `não medido` neste levantamento | `owner não identificado` | 2026-08-13 |
| SLOs/SLIs formais por serviço | D10 | `não medido` | `owner não identificado` | 2026-08-13 |
| Métricas de lag/DLQ/consumer lag Kafka | N/A (Kafka ausente) | `não existe` no código inventariado | `owner não identificado` | 2026-08-13 |
| Métricas padronizadas de outbox (depth, age) | só titulos tem outbox; métricas dedicadas **não medidas** aqui | `Lacuna` | `owner não identificado` | 2026-08-13 |
| Funções APM presentes mas não chamadas | `telesena-titulos-services` | `Fato` | `owner não identificado` | 2026-08-13 |

---

## 5. Candidatos a piloto (rascunho)

> Rascunho para [SPEC-VVR1X71Q](../specs/SPEC-VVR1X71Q-dmpf-governanca-bom-pilotos.md) (FND-10).
> **Não** define charter nem métricas de sucesso. Owner default; data = 2026-08-13 (§10 dos cortes).

| Prioridade rascunho | Repositório | Motivo AS-IS | Ressalvas | Fonte §10 |
|---------------------|-------------|--------------|-----------|-----------|
| Forte | `shared-titulos-services` | SQS + outbox/inbox + dual DB + obs | complexidade / contratos SQS ad hoc | [titulos §10](../specs/SPEC-K9H204F1/inventario-shared-titulos-services-apps-libs.md#10-candidato-a-piloto) |
| Forte (eixo SQS) | `rendafacil-services` | producer/consumer SQS real + NR | domínio/Protobuf/outbox distantes; publish-in-TX | [rendafacil-services §10](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-services.md#10-candidato-a-piloto) |
| Sim c/ ressalvas | `shared-ro-sync-services` | SNS→SQS + envelope + upsert | ruído de libs; gaps outbox/protobuf | [ro-sync §10](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-apps-libs.md#10-candidato-a-piloto) |
| Sim c/ ressalvas | `telesena-ativavel-services` | SQS + ports; api-v2 + delivery-worker | domínio com I/O; sem outbox; idempotência frágil | [ativavel §10](../specs/SPEC-K9H204F1/inventario-telesena-ativavel-services-apps.md#10-candidato-a-piloto) |
| Parcial | `telesena-live-services` | SQS + idempotência parcial | sem outbox; domínio fraco | [live §10](../specs/SPEC-K9H204F1/inventario-telesena-live-services-apps-libs.md#10-candidato-a-piloto) |
| Parcial (domínio/OTel) | `backoffice-procap-services` | domínio sem I/O + OTel + Tx | sem mensageria real | [procap §10](../specs/SPEC-K9H204F1/inventario-as-is-backoffice-procap-services.md#10-candidato-a-piloto) |
| Kernel (não E2E) | `golibs` / libs/node | adapters/ports de referência | precisa consumidor | [golibs §10](../specs/SPEC-K9H204F1/inventario-as-is-golibs.md#10-candidato-a-piloto), [libs-node §10](../specs/SPEC-K9H204F1/inventario-shared-ro-sync-services-libs-node.md#10-candidato-a-piloto) |
| Não (mensageria) | `rendafacil-bff`, `telesena-monorepo`, `telesena-titulos-services` | HTTP/SQL sem broker | úteis só como fronteira/amostra de camadas | [bff §10](../specs/SPEC-K9H204F1/inventario-as-is-rendafacil-bff.md#10-candidato-a-piloto), [monorepo §10](../specs/SPEC-K9H204F1/inventario-as-is-telesena-monorepo.md#10-candidato-a-piloto), [telesena-titulos §10](../specs/SPEC-K9H204F1/inventario-telesena-titulos-services-apps-libs.md#10-candidato-a-piloto) |

---

## Apêndice A — Glossário mínimo deste inventário

| Termo | Uso aqui |
|-------|----------|
| `Fato` | Afirmação ancorada em arquivo/commit do relatório fonte |
| `Inferência` | Conclusão derivada; cita os fatos de apoio |
| `Lacuna` | Não encontrado no escopo inspecionado |
| `owner não identificado` | Ausência explícita de CODEOWNERS/equipe nomeada |
| D10 / D9serv | Denominadores da §0 |
