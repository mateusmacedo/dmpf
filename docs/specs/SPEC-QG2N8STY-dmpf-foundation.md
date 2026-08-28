---
id: SPEC-QG2N8STY
slug: dmpf-foundation
title: DMPF Foundation — Golden Path orientado a domínio e mensagens
stage: building
priority: P0
depends_on: []
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-436
subtask_urls: [https://lider-cap.atlassian.net/browse/ARQ-438, https://lider-cap.atlassian.net/browse/ARQ-439, https://lider-cap.atlassian.net/browse/ARQ-440, https://lider-cap.atlassian.net/browse/ARQ-441, https://lider-cap.atlassian.net/browse/ARQ-442, https://lider-cap.atlassian.net/browse/ARQ-443, https://lider-cap.atlassian.net/browse/ARQ-444, https://lider-cap.atlassian.net/browse/ARQ-445, https://lider-cap.atlassian.net/browse/ARQ-446, https://lider-cap.atlassian.net/browse/ARQ-447, https://lider-cap.atlassian.net/browse/ARQ-448]
created: 2026-08-13
---
# SPEC-QG2N8STY: DMPF Foundation — Golden Path orientado a domínio e mensagens

## Resumo

Estabelecer a fundação normativa do **Domain Message Processing Framework
(DMPF)** — RFC, ADRs e políticas comuns para Go e TypeScript — convertendo a
proposta conceitual em especificação revisável e verificável, **sem**
implementar kernels, adapters ou providers de produção.

## Contexto

- **Problema**: squads resolvem domínio, contratos, mensageria, transações e
  observabilidade de forma independente → decisões incompatíveis, acoplamento
  a frameworks e falhas recorrentes em commit/ACK/redelivery.
- **Impacto**: golden path prescritivo nas fronteiras e flexível na modelagem
  de cada bounded context; kernels futuros assentam sobre limites estáveis.
- **Épico**: [ARQ-436](https://lider-cap.atlassian.net/browse/ARQ-436) — Em Desenvolvimento; escopo = fundação
  normativa (RFC/ADRs), sem kernels Go/TS.
- **Fontes**: `Parte-1`, na convenção que a RFC fixa em §1.3 — a referência
  versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md`; os anexos ficam em
  área local, fora do versionamento (índice no README).

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] RFC DMPF Foundation v0.1**: consolidar e versionar a RFC normativa
  a partir de Parte-1 e inventário AS-IS.
- [ ] **[P0] ADRs mínimos (001–015)**: registrar decisões obrigatórias com
  contexto, alternativas, trade-offs e consequências.
- [ ] **[P0] Limites e dependências**: definir blocos (domain, service, app,
  port, provider, contract) e matriz verificável em CI futuro.
- [ ] **[P0] UPR + Decision + mensagens**: especificar processamento síncrono
  sem I/O e taxonomia domínio/aplicação/wire.
- [ ] **[P0] UoW / inbox / outbox**: semântica transacional e at-least-once.
- [ ] **[P0] CloudEvents + Protobuf/Buf**: perfil organizacional e governança.
- [ ] **[P0] Políticas de transporte**: REST/OpenAPI, gRPC, Kafka, SNS/SQS, AsyncAPI.
- [ ] **[P0] Contexto, erros, segurança, resiliência, observabilidade**: baselines.
- [ ] **[P0] Testes e interop Go ↔ TS**: pirâmide, test kits e golden fixtures.
- [ ] **[P0] Governança, BOM e pilotos**: produto, escape hatches e 2 fluxos-piloto.
- [ ] **[P0] Prontidão**: épicos subsequentes (kernel Go/TS, contratos, golden
  path, piloto) refinados e Ready.

### Não-funcionais

- [ ] **[P0] Desacoplamento / coesão / interoperabilidade** conforme ARQ-436 §10.
- [ ] **[P0] Confiabilidade at-least-once** com fronteiras transacionais explícitas.
- [ ] **[P1] Extensibilidade** por composição; ES/CQRS/CDC permanecem opt-in.

## Camadas afetadas

Épico **normativo**: as camadas abaixo são as do DMPF que a fundação define,
não módulos de código a alterar. A coluna indica qual sub-spec normatiza cada
camada.

| Camada | Normatizada? | Sub-spec responsável |
|--------|--------------|----------------------|
| Domínio (UPR, Decision, eventos) | [x] | FND-03 (SPEC-8MNDEWDP), limites em FND-02 |
| Application service (UoW, orquestração) | [x] | FND-04 (SPEC-7PJ5WVCS) |
| Port / Provider (adapters, drivers) | [x] | FND-02 (SPEC-8YVF0RR5) |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | FND-05 (SPEC-7H08RZDG) |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | FND-06 (SPEC-YWFGNPG5) |
| Segurança e multi-tenancy | [x] | FND-07 (SPEC-XQWGGAXF) |
| Observabilidade e operação | [x] | FND-08 (SPEC-E15TBHCD) |
| Testes e interoperabilidade | [x] | FND-09 (SPEC-6RQBN98G) |
| Kernel Go / TypeScript (implementação) | [ ] | **Fora do épico** — ver Escopo fora |

## Localização de código

Fundação documental — **sem** código de kernel neste épico:

- SPECs: `docs/specs/SPEC-QG2N8STY-dmpf-foundation.md` + 11 sub-specs
- Drafts: área local, fora do versionamento (ver README)
- Promoção — artefatos normativos: `docs/dmpf/`
- Promoção — ADRs: `docs/adr/010-*.md` … `docs/adr/024-*.md`, indexados em `docs/adr/README.md`
- Pipeline: estado local do pipeline, fora do versionamento

## Design

### Sub-specs (decomposição)

| Issue | SPEC | Slug | depends_on |
| --- | --- | --- | --- |
| ARQ-438 | SPEC-K9H204F1 | dmpf-inventario-as-is | [] |
| ARQ-439 | SPEC-8YVF0RR5 | dmpf-rfc-limites-deps | [SPEC-K9H204F1] |
| ARQ-440 | SPEC-8MNDEWDP | dmpf-upr-decision-mensagens | [SPEC-8YVF0RR5] |
| ARQ-441 | SPEC-7PJ5WVCS | dmpf-uow-inbox-outbox | [SPEC-8MNDEWDP] |
| ARQ-442 | SPEC-7H08RZDG | dmpf-cloudevents-protobuf-buf | [SPEC-8MNDEWDP] |
| ARQ-443 | SPEC-YWFGNPG5 | dmpf-politicas-transporte | [SPEC-7H08RZDG] |
| ARQ-444 | SPEC-XQWGGAXF | dmpf-contexto-erros-seguranca | [SPEC-8YVF0RR5] |
| ARQ-445 | SPEC-E15TBHCD | dmpf-resiliencia-observabilidade | [SPEC-XQWGGAXF] |
| ARQ-446 | SPEC-6RQBN98G | dmpf-testes-interop | [SPEC-7PJ5WVCS, SPEC-7H08RZDG] |
| ARQ-447 | SPEC-VVR1X71Q | dmpf-governanca-bom-pilotos | [SPEC-K9H204F1] |
| ARQ-448 | SPEC-DBTRMM3X | dmpf-adrs-minimos | [SPEC-8YVF0RR5] |

### Grafo (resumo)

FND-01 → FND-02 → FND-03 → (FND-04 ∥ FND-05) → FND-06; FND-02 → FND-07 → FND-08;
FND-04+FND-05 → FND-09; FND-01 → FND-10 (fecha após demais); FND-02 → FND-11.

## Decisões técnicas

- Artefatos de trabalho nascem em área local (flat), **fora do versionamento**.
  Ao serem aprovados, promovem para área versionada: artefatos normativos para
  `docs/dmpf/`, ADRs para `docs/adr/` na faixa `010`–`024`, em sequência
  contínua aos `001`–`009`
  do template. A revisão formal acontece sobre o artefato promovido, via PR.
- Paridade conceitual Go/TS; APIs idiomáticas distintas.
- REST externo; gRPC/Protobuf síncrono interno; Kafka/SNS/SQS com Protobuf.

## Verificação e testes

### Critérios de aceite

Espelham AC-01…AC-12 do épico ARQ-436 (detalhados nas sub-specs):

- [ ] **[P0] AC-01** RFC e decisões
- [ ] **[P0] AC-02** Limites arquiteturais
- [ ] **[P0] AC-03** Modelo de domínio e processamento
- [ ] **[P0] AC-04** Mensagens e contratos
- [ ] **[P0] AC-05** Protobuf e CloudEvents
- [ ] **[P0] AC-06** Comunicação síncrona e assíncrona
- [ ] **[P0] AC-07** Transação, inbox e outbox
- [ ] **[P0] AC-08** Contexto, erros e segurança
- [ ] **[P0] AC-09** Resiliência e observabilidade
- [ ] **[P0] AC-10** Qualidade e interoperabilidade
- [ ] **[P0] AC-11** Governança e adoção
- [ ] **[P0] AC-12** Prontidão para implementação

### Cenários de teste (mínimo 3)

1. **Happy path**: RFC + ADRs aceitos e sub-specs FND-01…11 com evidências §11.
2. **Edge**: decisão pendente com owner/prazo que não bloqueia épicos seguintes.
3. **Erro / creep**: tentativa de incluir kernel Go/TS neste épico → rejeitada
   pelo Escopo fora (§6.2).

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

Conforme ARQ-436 §6.2:

- **Kernel Go / TypeScript completo**: épicos dependentes.
- **Adapters/providers de produção**: fora da fundação.
- **Migração de serviços existentes**: pós-fundação.
- **ES / CQRS físico / CDC obrigatórios**: permanecem opt-in.
- **Generators, portal, dashboards prod, golden path completo, v1.0**: épicos seguintes.
- **Funcionalidades de negócio**: não pertencem ao DMPF Foundation.
