---
id: SPEC-DBTRMM3X
slug: dmpf-adrs-minimos
title: DMPF — ADRs mínimos (ADR-001 a ADR-015)
stage: backlog
priority: P0
depends_on: [SPEC-8YVF0RR5]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-448
subtask_urls: []
created: 2026-08-13
---
# SPEC-DBTRMM3X: DMPF — ADRs mínimos (ADR-001 a ADR-015)

## Resumo

Entregar o item lógico **FND-11** do épico ARQ-436
(ARQ-448): produzir a evidência «Escrita das ADRs que estabelecem as decisões de arquitetura que norteiam o projeto».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448)
- **ACs do épico**: AC-01 (ADRs)
- **Evidência §11**: Escrita das ADRs que estabelecem as decisões de arquitetura que norteiam o projeto

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] ADR-001…015**: criar drafts cobrindo a tabela ARQ-436 §8
- [ ] **[P0] Formato**: contexto, decisão, alternativas, trade-offs, consequências
- [ ] **[P0] ADRs estruturais com a RFC**: limites, UPR, contratos, Protobuf (acompanham FND-02/03)
- [ ] **[P1] Demais ADRs**: acompanham a história temática correspondente (UoW, transportes, etc.)
- [ ] **[P0] Numeração contínua**: promover para `docs/adr/` na faixa `010`–`024`, dando sequência aos `001`–`009` do template

### Não-funcionais

- [ ] **[P0] Alternativa real**: cada ADR registra ao menos uma alternativa que foi genuinamente considerada, com o motivo do descarte
- [ ] **[P0] Consequência assumida**: o custo aceito é declarado; ADR que só lista benefícios é rejeitado na revisão
- [ ] **[P0] Rastreabilidade**: cada ADR aponta a sub-spec que o originou e é referenciado de volta por ela
- [ ] **[P1] Autocontenção**: o ADR é legível sem exigir leitura prévia da RFC inteira

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | ADRs de UPR, Decision e limite sem I/O |
| Application service (UoW, orquestração) | [x] | ADRs de UoW e fronteira transacional |
| Port / Provider (adapters, drivers) | [x] | ADRs de port, provider e regra de dependência |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | ADRs de Protobuf, CloudEvents e evolução |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | ADRs de escolha por transporte |
| Observabilidade e operação | [x] | ADRs de resiliência e observabilidade |

Cobertura ampla por natureza: FND-11 registra as decisões de todas as demais
sub-specs.

## Localização de código

| Fase | Caminho |
|------|---------|
| Draft de trabalho | `plans/references/` (flat) — local, fora do versionamento |
| Promoção (ao ser aceito) | `docs/adr/010-*.md` … `docs/adr/024-*.md` — versionado e revisável por PR |
| Índice | `docs/adr/README.md` — recebe as 15 entradas na promoção |

## Design

### Formato do ADR

Cinco seções obrigatórias:

1. **Contexto** — a situação que força a decisão.
2. **Decisão** — o que foi decidido, em linguagem imperativa.
3. **Alternativas** — o que mais foi considerado.
4. **Trade-offs** — o que se ganha e o que se perde.
5. **Consequências** — o que passa a ser verdade depois da decisão.

### Cadência de emissão

Os ADRs não saem todos ao final. Cada um acompanha a sub-spec que o origina:

| Grupo | Emitido com |
|-------|-------------|
| estruturais (limites, regra de dependência) | FND-02 |
| UPR, Decision, contratos | FND-03 |
| UoW, inbox, outbox | FND-04 |
| Protobuf, CloudEvents, evolução | FND-05 |
| transportes | FND-06 |
| contexto, erros, segurança | FND-07 |
| resiliência, observabilidade | FND-08 |

### Numeração e promoção

Os drafts ficam **flat** em `plans/references/` enquanto são escritos. Ao serem
aceitos, promovem para `docs/adr/` na faixa **`010`–`024`**, dando sequência
contínua aos `001`–`009` já ocupados pelo template. Não há prefixo dedicado nem
índice paralelo: os ADRs do DMPF e os do template convivem no mesmo diretório e
no mesmo `docs/adr/README.md`.

A numeração final de cada ADR é atribuída **na promoção**, não no draft — dois
drafts em revisão simultânea não disputam o mesmo número.

### Pendência explícita

ADR sem consenso não bloqueia a sub-spec: é registrado como **pendente**, com
owner e prazo. É o mecanismo previsto no cenário de edge da umbrella.

## Decisões técnicas

- **Índice único em `docs/adr/`, faixa `010`–`024`**: as decisões do DMPF
  convivem com as do template num só lugar, então quem procura uma decisão de
  arquitetura do repositório tem um único índice para consultar. Alternativa
  descartada: diretório isolado (`docs/dmpf/adr/`) com numeração própria a
  partir de `001`, porque criaria dois índices concorrentes e obrigaria o
  leitor a saber de antemão em qual procurar.
- **Numeração atribuída na promoção, não no draft**: evita disputa de número
  entre drafts em revisão simultânea. Alternativa descartada: reservar o número
  ao criar o draft, porque um draft abandonado deixaria buraco na sequência.
- **ADR emitido junto da sub-spec de origem**: a decisão é registrada enquanto
  o contexto está vivo. Alternativa descartada: escrever os 15 ao final, porque
  vira exercício de arqueologia e as alternativas consideradas se perdem.
- **Alternativa genuína obrigatória**: ADR sem alternativa real é decisão
  narrada, não registrada. Alternativa descartada: aceitar a seção preenchida
  com opções nunca consideradas, porque destrói o valor do documento para quem
  reavaliar a decisão depois.
- **Pendência com owner e prazo em vez de bloqueio**: mantém o épico avançando.
  Alternativa descartada: exigir consenso pleno antes de prosseguir, porque uma
  decisão controversa travaria as onze sub-specs.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] 15 ADRs mínimos promovidos para `docs/adr/010`–`024` e revisáveis em PR**
- [ ] **[P0] Cada ADR obrigatório aceito ou com pendência explícita (owner/prazo)**
- [ ] **[P0] `docs/adr/README.md` atualizado com as 15 entradas, e o índice referenciado pela RFC e pela umbrella**

### Cenários de teste (mínimo 3)

```
DADO um ADR estrutural emitido junto com a RFC de FND-02
QUANDO submetido à revisão
ENTÃO apresenta as cinco seções, com ao menos uma alternativa real descartada
     e o custo aceito declarado

DADO um ADR sobre o qual os revisores não chegam a consenso
QUANDO a sub-spec de origem é encerrada
ENTÃO o ADR fica registrado como pendente com owner e prazo, e as sub-specs
     dependentes não são bloqueadas

DADO dois drafts de ADR do DMPF aceitos na mesma rodada de revisão
QUANDO são promovidos para `docs/adr/`
ENTÃO recebem números distintos e contínuos a partir de `010`, sem colidir
     entre si nem com os `001`–`009` do template, e ambos entram no
     `docs/adr/README.md`
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **ADRs de decisões de implementação**: kernel e providers registram os seus
  nos épicos correspondentes.
- **Reescrita dos ADRs `001`–`009` do template**: os existentes não são
  tocados; esta spec apenas acrescenta a faixa `010`–`024` e as entradas
  correspondentes no índice.
- **Ferramenta de gestão de ADRs**: os drafts são markdown; tooling é
  pós-fundação.
- **Promoção dos demais artefatos normativos**: RFC, inventário e políticas vão
  para `docs/dmpf/`, cada um pela sua sub-spec.
