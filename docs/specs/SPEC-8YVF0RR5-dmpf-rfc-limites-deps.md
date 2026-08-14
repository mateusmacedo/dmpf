---
id: SPEC-8YVF0RR5
slug: dmpf-rfc-limites-deps
title: DMPF — RFC, limites arquiteturais e regra de dependência
stage: backlog
priority: P0
depends_on: [SPEC-K9H204F1]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-439
subtask_urls: []
created: 2026-08-13
---
# SPEC-8YVF0RR5: DMPF — RFC, limites arquiteturais e regra de dependência

## Resumo

Entregar o item lógico **FND-02** do épico ARQ-436
(ARQ-439): produzir a evidência «RFC revisada, diagramas, matriz de responsabilidades e ADRs estruturais aceitos».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-439](https://lider-cap.atlassian.net/browse/ARQ-439)
- **ACs do épico**: AC-01, AC-02
- **Evidência §11**: RFC revisada, diagramas, matriz de responsabilidades e ADRs estruturais aceitos

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] RFC v0.1**: consolidar Parte-1 + inventário em RFC versionada
- [ ] **[P0] Blocos sem sobreposição**: domain, application service, app, port, provider, contract
- [ ] **[P0] Matriz de dependências**: permitidas/proibidas; verificação automática planejada (Go e TS)
- [ ] **[P0] Domínio em memória**: domínio executável/testável sem SDK/ORM/broker
- [ ] **[P1] ADRs estruturais**: acionar FND-11 para ADR-001 (e correlatos) com a RFC

### Não-funcionais

- [ ] **[P0] Verificabilidade mecânica**: cada aresta proibida da matriz é expressa de forma que um linter de import possa checá-la, em Go e em TS
- [ ] **[P0] Ausência de sobreposição**: todo elemento do sistema pertence a exatamente um bloco, sem zona cinzenta
- [ ] **[P0] Paridade conceitual Go/TS**: os mesmos limites valem nas duas stacks, com APIs idiomáticas distintas
- [ ] **[P1] Versionamento**: a RFC é versionada e mudanças normativas exigem ADR correspondente

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | Define o domínio como bloco sem I/O, executável em memória |
| Application service (UoW, orquestração) | [x] | Define o bloco e sua fronteira com domínio e ports |
| Port / Provider (adapters, drivers) | [x] | Define port como abstração e provider como implementação |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Define contract como bloco próprio, fora do domínio |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [ ] | Políticas por transporte são de FND-06 |
| Observabilidade e operação | [ ] | Baselines são de FND-08 |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `plans/references/Parte-1-conceitual.md` (baixado do Jira) |
| Draft de trabalho | `plans/references/rfc-dmpf-foundation-v0.1.md` — local, fora do versionamento |
| Promoção (ao ser aprovada) | `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — repositório canônico, revisável por PR |

## Design

### Blocos e regra de dependência

Seis blocos, sem sobreposição. A regra é unidirecional: dependências apontam
para dentro, em direção ao domínio.

```
app  ──▶  application service  ──▶  domain
 │               │                    ▲
 │               ▼                    │
 └──────▶  port (abstração)  ─────────┘
                 ▲
                 │ implementa
           provider (driver, SDK, broker)

contract (wire) ──▶ usado por provider e app; NUNCA pelo domain
```

### Matriz de dependências

A RFC publica a matriz completa permitido/proibido entre os seis blocos. As
arestas P0 proibidas — `domain → provider`, `domain → contract`,
`domain → port de infraestrutura` — são as que o linter de import deve barrar.

### Fluxo

1. Consolidar Parte-1 e o inventário de FND-01 na RFC v0.1.
2. Publicar blocos, matriz e diagramas.
3. Submeter à revisão de Arquitetura, Segurança, Plataforma, Go e TS.
4. Acionar FND-11 para ADR-001 e correlatos estruturais.

## Decisões técnicas

- **Regra de dependência verificável, não apenas descrita**: a matriz é
  escrita de modo que um linter de import a implemente depois. Alternativa
  descartada: descrever os limites só em prosa, porque limite não verificável
  mecanicamente erode a cada sprint.
- **Domínio executável em memória como teste do limite**: se o domínio precisa
  de SDK, ORM ou broker para rodar um teste, o limite foi violado. Alternativa
  descartada: permitir mocks de infraestrutura no domínio, porque isso legitima
  a dependência que a regra existe para proibir.
- **`contract` é bloco próprio, não parte do domínio**: separa o modelo de wire
  do modelo interno. Alternativa descartada: tipos gerados de Protobuf como
  modelo de domínio, porque acopla o domínio ao formato de serialização e é
  proibido pelas constraints P0.
- **ADRs estruturais junto com a RFC**: os ADRs estruturais são emitidos por
  FND-11 acompanhando esta entrega e promovidos para `docs/adr/` na faixa
  `010`–`024`. Alternativa descartada: emitir todos os ADRs ao final do épico,
  porque decisões estruturais precisam estar aceitas antes de FND-03…09
  dependerem delas.
- **Promoção como gatilho de canonicidade**: a RFC só é "publicada" quando sai
  de `plans/` (local, ignorado pelo git) para `docs/dmpf/`. Alternativa
  descartada: tratar o draft local como publicação, porque as sub-specs
  dependentes citam seções da RFC e precisam de um alvo versionado.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] RFC promovida para `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — repositório canônico, versionada e com owner**
- [ ] **[P0] Matriz de dependências e diagramas revisados em PR**
- [ ] **[P0] Revisão Arquitetura/Segurança/Plataforma/Go/TS iniciada ou concluída (per AC-01)**

### Cenários de teste (mínimo 3)

```
DADO a RFC v0.1 com os seis blocos e a matriz de dependências
QUANDO submetida às revisões de Arquitetura, Segurança, Plataforma, Go e TS
ENTÃO é aceita e os ADRs estruturais correspondentes são acionados em FND-11

DADO um elemento do sistema que parece caber em dois blocos ao mesmo tempo
QUANDO a matriz é aplicada a ele
ENTÃO a RFC resolve a ambiguidade nomeando o bloco único ao qual pertence

DADO um exemplo de código de domínio que importa um driver de banco
QUANDO confrontado com a matriz de dependências
ENTÃO a aresta é identificada como proibida e o exemplo é rejeitado
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Implementação do linter de dependências**: aqui a matriz é especificada de
  forma verificável; construir a checagem em CI é dos épicos de kernel.
- **Modelagem interna de cada bounded context**: a RFC é prescritiva nas
  fronteiras e flexível na modelagem de cada contexto.
- **UPR, Decision e taxonomia de mensagens**: são de FND-03 (SPEC-8MNDEWDP).
- **Redação dos 15 ADRs**: é de FND-11 (SPEC-DBTRMM3X); aqui só se aciona os
  estruturais.
