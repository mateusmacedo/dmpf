<!-- ephemeral-refs-ok-file: baseline candidato FND-01 — cita evidências em docs/specs -->
# Inventário AS-IS — DMPF Foundation (FND-01)

> **Status:** promovido para revisão (**baseline candidato** — aprovação
> Plataforma/Arquitetura em PR ainda pendente)
> **Draft de origem:** rascunho local homônimo, fora do versionamento — não citável como fonte
> **Spec:** a análise AS-IS de sistemas legados /
> ARQ-438
> **Data da consolidação:** 2026-08-14
> **Responsável pela consolidação:** agente (Cursor) sob pipeline a análise AS-IS de sistemas legados
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
| 1 | `legado-backoffice-services` | `fab7c8ec` | apps/api, apps/migration, libs | inventario-as-is-legado-backoffice-services.md |
| 2 | `legado-golibs` | `9106941` | packages/* (kernels Go; sem app de produção) | inventario-as-is-golibs.md |
| 3 | `legado-rendas-bff` | `e345066` | src/ (BFF Nest) | inventario-as-is-legado-rendas-bff.md |
| 4 | `legado-rendas-services` | `78886b1` | apps/*, pkg/* | inventario-as-is-legado-rendas-services.md |
| 5 | `legado-monorepo` | `4a9c659` | apps + libs Nest | inventario-as-is-legado-monorepo.md |
| 6a | `legado-sync-services` | `82fae7f` | apps/** + libs/** (corte apps-libs; Node detalhado em 6b) | inventario-legado-sync-services-apps-libs.md |
| 6b | `legado-sync-services` | `82fae7f` | libs/node | inventario-legado-sync-services-libs-node.md |
| 7 | `legado-titulos-shared-services` | `a11348ac` | apps + libs (híbrido) | inventario-legado-titulos-shared-services-apps-libs.md |
| 8a | `legado-ativavel-services` | `d7d7f0f8` | apps | inventario-legado-ativavel-services-apps.md |
| 8b | `legado-ativavel-services` | `d7d7f0f8` | apps + shared | inventario-legado-ativavel-services-apps-shared.md |
| 9a | `legado-live-services` | `dd50a92` | apps + libs | inventario-legado-live-services-apps-libs.md |
| 9b | `legado-live-services` | `dd50a92` | libs | inventario-legado-live-services-libs.md |
| 10 | `legado-titulos-services` | `0cbfdb4` | apps/api + libs/apm | inventario-legado-titulos-services-apps-libs.md |

**Checagem:** 13 relatórios | 10 repositórios únicos | 3 repos com corte duplo
(`legado-sync-services`, `legado-ativavel-services`, `legado-live-services`).

**Denominadores usados neste documento:**
- **D10** = 10 repositórios únicos da matriz.
- **D9serv** = 9 repositórios com serviço/app implantável (`legado-golibs` excluído — só kernels).
- **SQS runtime (serviços)** = subset de D9serv com producer/consumer SQS no código de app.

Prompt de coleta: prompt-inventario-repositorio.md.

---

## 1. Padrões vigentes

> Mensageria, contratos, UoW/transação e observabilidade — síntese cross-cutting.
> Levantamento dos cortes: 2026-08-13. Owner default: `owner não identificado`.

### 1.1 Mensageria

| Padrão observado | Repos | Estado | Fontes |
|------------------|-------|--------|--------|
| AWS SQS em apps/serviços (runtime) | 5/9serv: `legado-rendas-services`, `legado-sync-services`, `legado-titulos-shared-services`, `legado-ativavel-services`, `legado-live-services` | `Fato` | legado-rendas-services, ro-sync, titulos, ativavel, live |
| Adapters SQS em biblioteca (sem topologia de produção) | `legado-golibs` (`gocqrs` + `goservice`) | `Fato` | golibs |
| SNS → SQS (fan-out com FilterPolicy) | `legado-sync-services` | `Fato` | ro-sync |
| Kernel Watermill (AMQP/SQS/SQL/in-memory) | `legado-golibs` (`gocqrs`) | `Fato` | golibs |
| Pacote npm `@mateusmacedo/message-broker` | `legado-ativavel-services`, `legado-live-services` | `Fato` | ativavel-shared, live-libs |
| Sem broker no código do escopo | 4/9serv: `legado-backoffice-services`, `legado-rendas-bff`, `legado-monorepo`, `legado-titulos-services` | `Fato` | legado-backoffice, legado-rendas-bff, legado-monorepo, legado-titulos |
| Kafka client versionado | **0/D10** (só menção README em `legado-golibs`) | `Lacuna` / `Fato` de ausência | golibs; ausência nos demais cortes da §0 |
| Outbox / inbox nomeados | **1/D10** (`legado-titulos-shared-services`) | `Fato` | titulos |
| Publish dentro de callback de TX | `legado-rendas-services` | `Fato` | legado-rendas-services |

`Inferência` (alta): entre **serviços implantáveis**, a mensageria assíncrona observada é **SQS-cêntrica**; Kafka não é prática vigente nos repos inventariados.

### 1.2 Contratos (wire)

| Padrão observado | Cobertura | Estado | Fontes |
|------------------|-----------|--------|--------|
| HTTP via Swagger/OpenAPI | serviços HTTP: legado-backoffice, legado-rendas-bff, legado-rendas-services, legado-monorepo, ativavel, live, titulos-services, shared-titulos | `Fato` | legado-backoffice, legado-rendas-bff, legado-rendas-services, legado-monorepo, ativavel, live, legado-titulos, shared-titulos |
| Envelope JSON ad hoc para filas | repos com SQS runtime (§1.1) | `Fato` | mesmos links SQS runtime |
| `.proto` / Buf / Schema Registry / AsyncAPI | **0/D10** | `Lacuna` | ausência registrada nos 13 cortes da §0 |
| Contract tests (exceção) | `legado-monorepo` soft-bff + `@legado-monorepo/backend-contracts` | `Fato` | legado-monorepo |

`Inferência` (alta): contratos assíncronos **não** são schema-first; evolução observada = tipagem TS/Go + Swagger HTTP.

### 1.3 Unidade de trabalho / transação / outbox-inbox

| Padrão observado | Exemplos | Estado | Fontes |
|------------------|----------|--------|--------|
| TX local (GORM / pgx / Prisma / TypeORM / mssql) | legado-backoffice, legado-rendas-services, ativavel, live, legado-monorepo auth-api, legado-titulos | `Fato` | cortes respectivos na §0 |
| Transactional Outbox + inbox nomeados | `legado-titulos-shared-services` (`backoffice_outbox`, `inbound_event`) | `Fato` | titulos |
| Sem outbox/inbox nomeados | 9/D10 (todos exceto shared-titulos) | `Lacuna` | cortes §0 ≠ titulos |
| Efeitos async via stored procedure na mesma TX | `legado-monorepo` auth-api | `Fato` | legado-monorepo |
| Idempotência pontual (unique/guards/ADR) | live, ro-sync, shared-titulos | `Fato` | live, ro-sync, titulos |

### 1.4 Observabilidade

| Stack observada | Repos | Estado | Fontes |
|-----------------|-------|--------|--------|
| OpenTelemetry | `legado-golibs`, `legado-backoffice-services`, `legado-sync` libs/node | `Fato` | golibs, legado-backoffice, libs-node |
| New Relic e/ou Elastic APM | legado-rendas-*, legado-*, ativavel, live, shared-titulos, legado-titulos | `Fato` | cortes Nest/Go de produto na §0 |
| Pino / nestjs-pino / gologger | legado-backoffice, golibs, BFFs/APIs Nest inventariadas | `Fato` | cortes §0 com stack de log |
| Prometheus `/metrics` | `legado-backoffice-services`; worker entrega em `legado-titulos-shared-services` | `Fato` | legado-backoffice, titulos |
| Trace headers em mensagem SQS | `legado-live-services` | `Fato` | live |

`Lacuna`: padronização única OTel vs NR/Elastic **não** aparece no universo inventariado.

---

## 2. Libs e componentes

### 2.1 Go (kernels / monorepos)

| Componente | Repo | Papel AS-IS | Owner | Estado |
|------------|------|-------------|-------|--------|
| `gocqrs` (Watermill) | `legado-golibs` | adapters AMQP/SQS/SQL/in-memory | `owner não identificado` | `Fato` — golibs |
| `goservice` SQS (SDK v1) | `legado-golibs` | segundo caminho SQS, paralelo ao `gocqrs` | `owner não identificado` | `Fato` |
| `godata` / `gotelemetry` / `gologger` / `goresilience` | `legado-golibs` | persistência, OTel+NR, logs, resiliência | `owner não identificado` | `Fato` |
| Workers + contracts Go | `legado-sync-services` | sync RO Postgres→SNS/SQS→MSSQL | `owner não identificado` | `Fato` |
| Apps Go (pauta/estoque/etc.) | `legado-titulos-shared-services` | domínio títulos/filantropia | `owner não identificado` | `Fato` |
| API Gin + libs telemetry | `legado-backoffice-services` | backoffice legado | `owner não identificado` | `Fato` |
| API Fiber + `pkg/sqs` | `legado-rendas-services` | afiliados Renda Fácil | `owner não identificado` (README lista e-mails; sem CODEOWNERS) | `Fato` |

### 2.2 TypeScript / Nest / Node

| Componente | Repo | Papel AS-IS | Owner | Estado |
|------------|------|-------------|-------|--------|
| `@mateusmacedo/message-broker` | consumido por ativavel/live | cliente SQS compartilhado | `owner não identificado` *neste inventário* (pacote fora do corte) | `Fato` de uso |
| `@lideranca-sites/node-*` (17 libs) | `legado-sync-services` libs/node | ports/adapters, telemetry, logger | `owner não identificado` | `Fato` — libs-node |
| `@legado-monorepo/backend-*` | `legado-monorepo` | auth, contracts, health, http-client, observability | `owner não identificado` | `Fato` |
| `shared/core` + `shared/infra` | `legado-ativavel-services` | ports/queue + Prisma/TypeORM | `owner não identificado` (author em package.json) | `Fato` |
| libs Nest (`aws`, `apm`, `database`, …) | `legado-live-services` | SQS, APM, TypeORM | `owner não identificado` | `Fato` |
| BFF Nest | `legado-rendas-bff` | fachada HTTP | `owner não identificado` | `Fato` |
| API Nest + `libs/apm` | `legado-titulos-services` | títulos HTTP/MSSQL | `owner não identificado` (author em package.json) | `Fato` |

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
| SNS usado como fan-out para sync RO | `legado-sync-services` | `Fato` | `owner não identificado` | 2026-08-13 |
| Kafka **não** é stack vigente nos cortes | 0 clients versionados / D10 | `Lacuna` / ausência | `owner não identificado` | 2026-08-13 |
| Dual DB Postgres + SQL Server legado | shared-titulos, ativavel, legado-rendas integrador, ro-sync | `Fato` | `owner não identificado` | 2026-08-13 |
| HTTP síncrono cobre fatia relevante | legado-backoffice, legado-rendas-bff, legado-monorepo, legado-titulos | `Fato` | `owner não identificado` | 2026-08-13 |

### 3.2 Segurança e compliance (quando evidenciado)

| Achado | Cobertura | Estado | Owner | Data |
|--------|-----------|--------|-------|------|
| Semgrep / scanners em CI (parcial) | legado-backoffice, legado-titulos (workflows ativos limitados) | `Fato` pontual | `owner não identificado` | 2026-08-13 |
| Política org de segurança centralizada | **não medido** neste FND-01 | `Lacuna` | `owner não identificado` | 2026-08-13 |
| Secrets/IAM detalhados | fora do corte profundo da maioria dos relatórios | `Lacuna` | `owner não identificado` | 2026-08-13 |

### 3.3 Gaps vs. constraints DMPF (descritivo)

| Constraint DMPF (P0) | AS-IS observado | Estado | Owner | Data |
|----------------------|-----------------|--------|-------|------|
| Domínio sem I/O | Distâncias documentadas (ex.: ativavel `domain` importa Prisma/broker; live `domain` com DTOs Swagger) | `Fato` de distância | `owner não identificado` | 2026-08-13 |
| Protobuf só no wire | Protobuf **ausente**; wire = JSON/Swagger | `Fato` | `owner não identificado` | 2026-08-13 |
| At-least-once + efeitos idempotentes | **Heterogêneo** — ver matriz abaixo (não generalizar “via SQS”) | `Fato` / `Inferência` | `owner não identificado` | 2026-08-13 |
| Kernels Go/TS fora do épico normativo | `legado-golibs` e `libs/node` existem como baseline de adapters | `Fato` | `owner não identificado` | 2026-08-13 |

#### 3.3.1 Semântica de entrega / idempotência (serviços com SQS)

| Repo | Transporte | Ack / delete observado | Idempotência de efeito | Estado |
|------|------------|------------------------|------------------------|--------|
| `legado-titulos-shared-services` | SQS + outbox/inbox | worker lê outbox após commit | ADR/estratégias documentadas | `Fato` — titulos |
| `legado-sync-services` | SNS→SQS | consumers com upsert | idempotência por upsert | `Fato` — ro-sync |
| `legado-live-services` | SQS | delete após sucesso; retry por visibility | unique index / guards parciais | `Fato` — live |
| `legado-ativavel-services` | SQS | poll→process→delete | evidência registra lacuna de idempotência no consumer | `Fato` / `Lacuna` — ativavel |
| `legado-rendas-services` | SQS | delete-on-error pode perder mensagem | publish-in-TX documentado; idempotência não garantida E2E | `Fato` — legado-rendas-services |

---

## 4. Métricas atuais

### 4.1 Instrumentação existente

| Sinal | Onde | Estado | Owner | Data |
|-------|------|--------|-------|------|
| APM New Relic e/ou Elastic | serviços Nest/Go de produto na §0 | `Fato` | `owner não identificado` | 2026-08-13 |
| OpenTelemetry SDK | `legado-golibs`, legado-backoffice, libs/node ro-sync | `Fato` | `owner não identificado` | 2026-08-13 |
| Prometheus `/metrics` | legado-backoffice; worker entrega em `shared-titulos` | `Fato` | `owner não identificado` | 2026-08-13 |
| Logs estruturados (Pino/Logrus) | cortes com stack de log na §1.4 | `Fato` | `owner não identificado` | 2026-08-13 |
| Trace headers em mensagem SQS | `legado-live-services` | `Fato` | `owner não identificado` | 2026-08-13 |

### 4.2 Lacunas explícitas (`não medido`)

| Lacuna | Escopo | Estado | Owner | Data |
|--------|--------|--------|-------|------|
| Dashboards SRE/org nomeados e URLs | D10 | `não medido` neste levantamento | `owner não identificado` | 2026-08-13 |
| SLOs/SLIs formais por serviço | D10 | `não medido` | `owner não identificado` | 2026-08-13 |
| Métricas de lag/DLQ/consumer lag Kafka | N/A (Kafka ausente) | `não existe` no código inventariado | `owner não identificado` | 2026-08-13 |
| Métricas padronizadas de outbox (depth, age) | só titulos tem outbox; métricas dedicadas **não medidas** aqui | `Lacuna` | `owner não identificado` | 2026-08-13 |
| Funções APM presentes mas não chamadas | `legado-titulos-services` | `Fato` | `owner não identificado` | 2026-08-13 |

---

## 5. Candidatos a piloto (rascunho)

> Rascunho para [SPEC-VVR1X71Q](../specs/SPEC-VVR1X71Q-dmpf-governanca-bom-pilotos.md) (FND-10).
> **Não** define charter nem métricas de sucesso. Owner default; data = 2026-08-13 (§10 dos cortes).

| Prioridade rascunho | Repositório | Motivo AS-IS | Ressalvas | Fonte §10 |
|---------------------|-------------|--------------|-----------|-----------|
| Forte | `legado-titulos-shared-services` | SQS + outbox/inbox + dual DB + obs | complexidade / contratos SQS ad hoc | titulos §10 |
| Forte (eixo SQS) | `legado-rendas-services` | producer/consumer SQS real + NR | domínio/Protobuf/outbox distantes; publish-in-TX | legado-rendas-services §10 |
| Sim c/ ressalvas | `legado-sync-services` | SNS→SQS + envelope + upsert | ruído de libs; gaps outbox/protobuf | ro-sync §10 |
| Sim c/ ressalvas | `legado-ativavel-services` | SQS + ports; api-v2 + delivery-worker | domínio com I/O; sem outbox; idempotência frágil | ativavel §10 |
| Parcial | `legado-live-services` | SQS + idempotência parcial | sem outbox; domínio fraco | live §10 |
| Parcial (domínio/OTel) | `legado-backoffice-services` | domínio sem I/O + OTel + Tx | sem mensageria real | legado-backoffice §10 |
| Kernel (não E2E) | `legado-golibs` / libs/node | adapters/ports de referência | precisa consumidor | golibs §10, libs-node §10 |
| Não (mensageria) | `legado-rendas-bff`, `legado-monorepo`, `legado-titulos-services` | HTTP/SQL sem broker | úteis só como fronteira/amostra de camadas | bff §10, monorepo §10, legado-titulos §10 |

---

## Apêndice A — Glossário mínimo deste inventário

| Termo | Uso aqui |
|-------|----------|
| `Fato` | Afirmação ancorada em arquivo/commit do relatório fonte |
| `Inferência` | Conclusão derivada; cita os fatos de apoio |
| `Lacuna` | Não encontrado no escopo inspecionado |
| `owner não identificado` | Ausência explícita de CODEOWNERS/equipe nomeada |
| D10 / D9serv | Denominadores da §0 |
