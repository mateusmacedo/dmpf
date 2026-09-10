# Inventário AS-IS — Guia resumido

> **Fonte:** [`docs/dmpf/inventario-as-is.md`](../inventario-as-is.md) | **Status:** Guia derivado (não normativo) | **Linhas:** ~1.000 → ~180
>
> **Propósito:** Visão panorâmica do estado da plataforma de mensageria, transação e observabilidade em 10 repositórios (2026-08-13).

---

## Cobertura

| Item | Valor |
|--|--|
| **Repositórios únicos** | 10: backoffice-procap-services, golibs, rendafacil-bff, rendafacil-services, telesena-monorepo, shared-ro-sync-services, shared-titulos-services, telesena-ativavel-services, telesena-live-services, telesena-titulos-services |
| **Cortes** | 13 (alguns repos com dois cortes: apps+libs, apps, shared) |
| **Levantamento** | 2026-08-13 |
| **Formato** | Convenção: `Fato` (observado) / `Inferência` / `Lacuna` (ausência) |

---

## Mensageria: estado observado

- **SQS-cêntrico:** 5/9 serviços implantáveis usam SQS runtime; Kafka **ausente** em todas as versões versionadas
- **SNS → SQS:** FilterPolicy observado (`shared-ro-sync-services`)
- **Outbox/inbox nomeados:** só 1/10 (`shared-titulos-services` com `backoffice_outbox` e `inbound_event`)
- **Kernel Watermill:** `golibs` (AMQP/SQS/SQL/in-memory); dois caminhos SQS paralelos em alguns repos

---

## Contratos e wire

- **OpenAPI/Swagger:** HTTP em 8/10 repos (padrão observado)
- **Protobuf / Buf / Schema Registry / AsyncAPI:** 0/10 — lacuna total
- **Contract tests:** exceção em `telesena-monorepo`
- **Envelope assíncrono:** JSON ad hoc (não schema-first)

---

## Transação local

- **Drivers:** GORM, pgx, Prisma, TypeORM, MSSQL (cada repo seu)
- **TX + outbox:** só `shared-titulos-services`; demais sem padrão
- **Idempotência pontual:** observada em 3 repos (guards, unique constraints)

---

## Observabilidade

- **OpenTelemetry:** 3 repos (`golibs`, procap, shared-ro-sync libs)
- **New Relic / Elastic APM:** padrão em 6+ repos Nest/Go de produto
- **Log:** Pino/nestjs-pino ou go-logger
- **Métricas:** Prometheus `/metrics` em 2 repos (falta padronização OTel)

---

## Ownership formal

- **CODEOWNERS:** 0/10 (ausência em todos)
- **Fallback:** author em `package.json` ou README
- **Convenção deste documento:** sem owner nomeado → `owner não identificado`

---

## Restrições organizacionais e técnicas

- **Cloud:** AWS predominante (SQS/SNS/Kinesis), MSSQL legado
- **Falta de Kafka versionado:** zero clientes em produção
- **Prática de idempotência:** presente mas heterogênea
- **Sem baseline formalizado** de padrão de transação assíncrona

---

## Lacunas e próximos passos

| Lacuna | Impacto |
|--|--|
| Protobuf / Schema Registry / Buf | RFC (ANC-03) exige wire schema-first; parque usa JSON ad hoc |
| Padrão de outbox/inbox | RFC (ANC-02) exige; só 1/10 implementa |
| Kafka versionado | RFC (ANC-04) fixa Kafka como alvo; ausente no parque |
| Ownership formalizado | Sem CODEOWNERS; RFC (ANC-09, `BOM`) exige metadado de responsável |
| OTel padronizado | Fragmentado entre New Relic / Elastic / sem OTel em alguns | 

**Nota:** Este inventário é **baseline candidato** — aprovação Plataforma/Arquitetura ainda pendente.

Consulte a norma em [`RFC §1.5`](../rfc-dmpf-foundation-v0.1.md) para regras de lastragem e consequências de mudança do baseline.
