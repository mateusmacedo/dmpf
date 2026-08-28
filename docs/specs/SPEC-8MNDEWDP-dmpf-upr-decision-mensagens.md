---
id: SPEC-8MNDEWDP
slug: dmpf-upr-decision-mensagens
title: DMPF — UPR, Decision, application services e modelo de mensagens
stage: done
priority: P0
depends_on: [SPEC-8YVF0RR5]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-440
subtask_urls: []
created: 2026-08-13
---
# SPEC-8MNDEWDP: DMPF — UPR, Decision, application services e modelo de mensagens

## Resumo

Entregar o item lógico **FND-03** do épico ARQ-436
(ARQ-440): produzir a evidência «Contratos conceituais, exemplos e separação domínio/aplicação/wire aceitos».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-440](https://lider-cap.atlassian.net/browse/ARQ-440)
- **ACs do épico**: AC-03, AC-04
- **Evidência §11**: Contratos conceituais, exemplos e separação domínio/aplicação/wire aceitos
- **Artefato produzido**: [`docs/dmpf/upr-decision-mensagens.md`](../dmpf/upr-decision-mensagens.md)

### Adaptação do entregável de ADRs

A story ARQ-440 pedia originalmente "ADR-002 e ADR-003 **aceitos**". A RFC
`docs/dmpf/rfc-dmpf-foundation-v0.1.md` §13.1 atribui a redação, a promoção e o
aceite dos ADRs estruturais ao FND-11
([ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)), na faixa
`docs/adr/010`–`024`. Esta entrega, portanto, **aciona** `ADR-DMPF-I` (forma do
desfecho da UPR) e `ADR-DMPF-J` (evento de domínio distinto do de integração) no
formato da RFC §13.2 — nomeando, definindo assunto, registrando origem e
alternativas descartadas, e encaminhando —, sem redigir nem aceitar nenhum dos
dois. O registro está em §9 do artefato. O critério original não é reescrito
aqui: fica registrado que o aceite formal é critério do FND-11.

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] UPR**: unidade síncrona, determinística e sem I/O
- [ ] **[P0] Decision<Response, DomainEvent>**: resultado oficial da UPR
- [ ] **[P0] Separação UPR vs application service**: ciclos de vida distintos
- [ ] **[P0] Taxonomia de mensagens**: command, query, response, domain event, integration event, rejection, job
- [ ] **[P0] Três níveis de contrato**: domínio ≠ aplicação ≠ wire; sem DTO universal
- [ ] **[P1] ES/CQRS**: documentados como extensões opt-in

### Não-funcionais

- [ ] **[P0] Determinismo**: dada a mesma entrada e o mesmo estado, a UPR produz a mesma `Decision` — sem relógio, aleatoriedade ou I/O implícitos
- [ ] **[P0] Independência de linguagem**: os exemplos são expressos em pseudocódigo, aplicáveis a Go e TS sem favorecer uma stack
- [ ] **[P0] Testabilidade sem infraestrutura**: todo exemplo de UPR é executável em memória
- [ ] **[P1] Extensibilidade opt-in**: ES e CQRS aparecem como extensões, nunca como requisito do golden path

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | Núcleo desta spec: define UPR, Decision e domain event |
| Application service (UoW, orquestração) | [x] | Define a fronteira e o ciclo de vida distinto da UPR |
| Port / Provider (adapters, drivers) | [ ] | Só como consumidor dos contratos definidos aqui |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Define o terceiro nível de contrato e a proibição do DTO universal |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [ ] | Políticas por transporte são de FND-06 |
| Observabilidade e operação | [ ] | Baselines são de FND-08 |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `Parte-1 §§5–6`, na convenção que a RFC fixa em §1.3; a referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md` |
| Draft de trabalho | área local — fora do versionamento, não citável como fonte |
| Promoção (ao ser aprovado) | `docs/dmpf/upr-decision-mensagens.md` — versionado e revisável por PR |

## Design

### UPR e Decision

A **UPR** (unidade de processamento de requisição) é síncrona, determinística e
sem I/O. Recebe entrada já validada e o estado necessário; devolve uma
`Decision`. Nunca busca dados: quem carrega estado é o application service.

```
Decision<Response, DomainEvent> =
  | Accepted(response: Response, events: DomainEvent[])
  | Rejected(rejection: Rejection)
```

### Ciclos de vida distintos

| | UPR | Application service |
|---|---|---|
| I/O | proibido | permitido (via ports) |
| Transação | não conhece | abre e fecha a UoW |
| Duração | uma decisão | um caso de uso completo |
| Teste | em memória, sem mocks de infra | com ports fakes |

### Três níveis de contrato

1. **Domínio** — tipos internos do bounded context; não cruzam a fronteira.
2. **Aplicação** — entrada e saída do caso de uso.
3. **Wire** — Protobuf, CloudEvents, JSON; só nos adapters.

Não existe DTO universal atravessando os três níveis.

### Taxonomia de mensagens

`command`, `query`, `response`, `domain event`, `integration event`,
`rejection`, `job` — cada um com emissor, consumidor e ciclo de vida
definidos. **Domain event** é interno ao contexto; **integration event** é o
que cruza a fronteira e tem contrato publicado.

## Decisões técnicas

- **`Decision` como retorno único da UPR**: rejeição é valor de retorno, não
  exceção. Alternativa descartada: lançar exceção para regra de negócio
  violada, porque mistura fluxo de erro técnico com decisão de domínio e
  dificulta exaustividade no consumidor.
- **UPR sem I/O, estado injetado pelo service**: torna a decisão determinística
  e testável em memória. Alternativa descartada: UPR buscando o próprio estado
  via repositório, porque reintroduz I/O no domínio e viola a constraint P0.
- **Domain event ≠ integration event**: eventos internos podem mudar sem
  coordenação; os de integração têm contrato versionado. Alternativa
  descartada: publicar o evento de domínio direto no broker, porque transforma
  todo detalhe interno em contrato público.
- **Sem DTO universal**: cada nível tem seu tipo. Alternativa descartada:
  reaproveitar o tipo gerado de Protobuf nos três níveis, porque é exatamente
  a violação que a constraint P0 proíbe.
- **ES e CQRS opt-in**: permanecem extensões documentadas. Alternativa
  descartada: torná-los obrigatórios no golden path, porque impõe custo alto a
  contextos que não precisam.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] UPR e Decision especificados com exemplos language-agnostic, no artefato promovido para `docs/dmpf/upr-decision-mensagens.md`**
- [ ] **[P0] Domain event ≠ integration event documentado e aprovado em PR**
- [ ] **[P0] Convenções de ownership/versionamento de contratos publicadas (nível conceitual) no artefato promovido**

### Cenários de teste (mínimo 3)

```
DADO uma UPR recebendo entrada válida e o estado necessário já carregado
QUANDO é executada duas vezes com a mesma entrada e o mesmo estado
ENTÃO produz a mesma Decision nas duas execuções, sem qualquer I/O

DADO uma regra de negócio violada pela entrada
QUANDO a UPR processa essa entrada
ENTÃO devolve Decision.Rejected com a rejection tipada, sem lançar exceção
     e sem emitir domain event

DADO um exemplo que usa o tipo gerado de Protobuf como estado do domínio
QUANDO confrontado com a separação de três níveis de contrato
ENTÃO é rejeitado por violar "Protobuf apenas no wire"
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Semântica transacional, inbox e outbox**: são de FND-04 (SPEC-7PJ5WVCS);
  aqui a UoW aparece apenas como fronteira do application service.
- **Serialização concreta em Protobuf/CloudEvents**: é de FND-05
  (SPEC-7H08RZDG); aqui o wire é tratado no nível conceitual.
- **Implementação de kernel Go/TS da UPR**: pertence aos épicos de kernel.
- **Modelagem de domínio de cada bounded context**: cada squad modela o seu; a
  spec normatiza a forma, não o conteúdo.
