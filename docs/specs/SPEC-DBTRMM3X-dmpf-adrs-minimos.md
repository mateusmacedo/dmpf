---
id: SPEC-DBTRMM3X
slug: dmpf-adrs-minimos
title: DMPF — ADRs mínimos (tabela ARQ-436 §8)
stage: building
priority: P0
depends_on: [SPEC-8YVF0RR5]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-448
subtask_urls:
  - https://lider-cap.atlassian.net/browse/ARQ-511
  - https://lider-cap.atlassian.net/browse/ARQ-512
  - https://lider-cap.atlassian.net/browse/ARQ-513
  - https://lider-cap.atlassian.net/browse/ARQ-514
  - https://lider-cap.atlassian.net/browse/ARQ-515
  - https://lider-cap.atlassian.net/browse/ARQ-516
  - https://lider-cap.atlassian.net/browse/ARQ-517
  - https://lider-cap.atlassian.net/browse/ARQ-518
created: 2026-08-13
---
# SPEC-DBTRMM3X: DMPF — ADRs mínimos (tabela ARQ-436 §8)

## Resumo

Entregar o item lógico **FND-11** do épico ARQ-436
(ARQ-448): produzir a evidência «Escrita das ADRs que estabelecem as decisões de arquitetura que norteiam o projeto».

O acervo normativo já **acionou** dezenove decisões, identificadas provisoriamente
como `ADR-DMPF-A`…`ADR-DMPF-S`. Cada uma saiu nomeada, com assunto, decisão e
alternativa descartada, mas nenhuma foi redigida, numerada ou aceita. Esta
sub-spec responde por essas três coisas: redigir os dezenove, promovê-los para
`docs/adr/010`–`028` e reconciliar a série com as quinze linhas da tabela §8 do
épico.

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

- [ ] **[P0] Redigir os 19 acionamentos**: um ADR para cada identificador provisório `ADR-DMPF-A`…`ADR-DMPF-S` acionado pelo acervo, cobrindo a tabela ARQ-436 §8 pela correspondência de assunto
- [ ] **[P0] Formato**: contexto, decisão, alternativas descartadas e consequências, estas separadas em positivas e negativas — é no bloco das negativas que o trade-off e o custo aceito ficam declarados
- [ ] **[P0] ADRs estruturais com a RFC**: limites, UPR, contratos, Protobuf — acionados por FND-02/03 (que os nomeia e define o assunto) e redigidos aqui
- [ ] **[P0] Demais ADRs**: acionados pela história temática correspondente (UoW, transportes, governança) e igualmente redigidos aqui
- [ ] **[P0] Numeração contínua**: promover para `docs/adr/` na faixa `010`–`028`, dando sequência aos `001`–`009` do template
- [ ] **[P0] Reconciliação de cardinalidade**: registrar a correspondência entre os 19 ADRs promovidos e as 15 linhas da tabela §8, nomeando as linhas sem acionamento e os acionamentos sem linha

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
| Acionamento (insumo) | tabelas de acionamento em `docs/dmpf/` — versionadas, com assunto, decisão, alternativa descartada e origem |
| Draft de redação | área local (flat) — fora do versionamento, não citável como fonte |
| Promoção (ao ser aceito) | `docs/adr/010-*.md` … `docs/adr/028-*.md` — versionado e revisável por PR |
| Índice | `docs/adr/README.md` — recebe as 19 entradas na promoção |

A fase de acionamento **já está concluída** e é versionada: os 19 identificadores
provisórios saíram nos artefatos normativos, não em área local. O que resta em
área local é o rascunho da redação, até a promoção.

## Design

### Formato do ADR

Quatro seções obrigatórias, na ordem do guia de escrita do repositório
(`docs/adr/README.md`), mais o bloco `## Status`, que o guia declara opcional e
que, quando presente, abre o arquivo:

1. **Contexto** — a situação que força a decisão.
2. **Decisão** — o que foi decidido, em linguagem imperativa.
3. **Alternativas descartadas** — o que mais foi considerado, em tabela de duas
   colunas (`Alternativa` / `Por que foi rejeitada`).
4. **Consequências** — o que passa a ser verdade depois da decisão, separado em
   **Positivas** e **Negativas**.

Os **trade-offs** não recebem seção autônoma. O guia os aloca no bloco
«Consequências → Negativas», cujo placeholder no template é literalmente
`- Trade-off 1`; é ali que o custo aceito da decisão fica declarado. Exigir uma
quinta seção divergiria do guia que os nove ADRs existentes já seguem.

### Cadência de emissão

Os ADRs não saem todos ao final. Cada um é acionado pela sub-spec que o origina,
enquanto o contexto da decisão está vivo: a sub-spec de origem nomeia o ADR e
define seu assunto; a redação, a promoção e o aceite são desta sub-spec (FND-11).

| Grupo | Acionado por | Artefato de origem | IDs provisórios |
|-------|--------------|--------------------|-----------------|
| estruturais (limites, regra de dependência) | FND-02 | `rfc-dmpf-foundation-v0.1.md` §13.2 | `A`–`H` (8) |
| UPR, Decision, contratos | FND-03 | `upr-decision-mensagens.md` | `I`, `J` |
| UoW, inbox, outbox | FND-04 | `uow-inbox-outbox.md` | `K`, `L` |
| Protobuf, CloudEvents, evolução | FND-05 | `cloudevents-protobuf-buf.md` | `M`, `N` |
| transportes | FND-06 | `politicas-transporte.md` | `O`, `P` |
| contexto, erros, segurança | FND-07 | `contexto-erros-seguranca.md` | — (ANC-05 dispensou) |
| resiliência, observabilidade | FND-08 | `resiliencia-observabilidade.md` | `Q` |
| governança, BOM, autoridade | FND-10 | `governanca-bom-pilotos.md` | `R`, `S` |

Duas correções em relação à versão anterior desta tabela, ambas apuradas contra o
acervo publicado:

- **FND-07 não acionou nenhum ADR.** ANC-05 dispensou os dois que a story ARQ-444
  lista como entregáveis; o artefato de origem registrou a divergência como
  pendência nomeada, sem resolvê-la. Reconciliar o entregável da story é trabalho
  desta sub-spec.
- **FND-10 estava ausente por omissão, não por decisão.** ANC-08 declara «ADR
  exigido: Sim», e o artefato acionou dois. FND-09 continua legitimamente fora:
  ANC-07 declara «ADR exigido: Não», sem condicional.

### Numeração e promoção

Os drafts ficam **flat** em área local enquanto são escritos. Ao serem
aceitos, promovem para `docs/adr/` na faixa **`010`–`028`**, dando sequência
contínua aos `001`–`009` já ocupados pelo template. Não há prefixo dedicado nem
índice paralelo: os ADRs do DMPF e os do template convivem no mesmo diretório e
no mesmo `docs/adr/README.md`.

Antes da promoção, cada ADR circula por um **identificador provisório** no formato
`ADR-DMPF-<letra>`, atribuído pelo artefato que o aciona. A RFC §13.2 fixa a regra:
os IDs provisórios não prometem número final, e citar um ADR por número definitivo
antes da promoção é erro de rastreabilidade. Esta sub-spec é a única autorizada a
converter letra em número.

A numeração final de cada ADR é atribuída **na promoção**, não no draft — dois
drafts em revisão simultânea não disputam o mesmo número.

### Reconciliação de cardinalidade

A tabela ARQ-436 §8 tem 15 linhas; o acervo produziu **19 acionamentos**. O
desequilíbrio não está espalhado — concentra-se em `ADR-001`, que o épico tratou
como uma decisão e a RFC decompôs em oito (`A`–`H`).

**Critério adotado: um ADR por acionamento.** Os 19 acionamentos promovem 1:1 para
`010`–`028`, e a correspondência com as linhas do épico é registrada por assunto,
não por número. O critério de aceite «15 ADRs mínimos» é um piso, e 19 o satisfaz.

*Alternativa descartada*: consolidar os 19 em exatamente 15 para casar com a
tabela §8. Fundir `A`–`H` numa única entrada apagaria sete decisões com assuntos
genuinamente distintos — regra de dependência, unidade arquitetural, classificação
por metadado, autoridade sobre a classificação, aresta `domain → port`,
capabilities por bloco, domínio executável e identidade de bounded context. O
registro perderia exatamente aquilo que justifica sua existência.

*Alternativa também descartada*: acionar retroativamente as cinco linhas do épico
que ficaram sem acionamento, chegando a 24. Reabriria decisões já fechadas — ANC-05
dispensou `ADR-012` e `ADR-013` com fundamento, e reverter isso aqui contrariaria a
âncora do FND-07 sem contexto para fazê-lo.

Restam **cinco linhas do épico sem ADR próprio**: `ADR-006` Unit of Work
explícita, `ADR-008` inbox e ACK/redelivery, `ADR-009` at-least-once idempotente,
e o par `ADR-012`/`ADR-013` de contexto e erros. Cada uma exige uma decisão
declarada: redigir a partir do artefato normativo que já cobre o assunto, ou
registrar que a decisão vive no artefato e não recebe ADR próprio. A escolha é
caso a caso e não é antecipada aqui.

### Divergência de escopo com a story

A story [ARQ-448](https://lider-cap.atlassian.net/browse/ARQ-448) declara, em
«Escopo», que escrever o conteúdo técnico dos ADRs é responsabilidade das stories
de origem, reduzindo esta sub-spec a padrão, índice, consistência cruzada e ciclo
de vida. A RFC §13.1 declara o oposto: a redação, a promoção e o aceite são do
FND-11.

**Prevalece a RFC**, e por eliminação: todas as sub-specs que acionaram ADR estão
concluídas, e nenhuma redigiu um — acionaram e encaminharam explicitamente a
redação para cá. Manter a leitura da story deixaria os 19 acionamentos sem dono e o AC-01
do épico sem como fechar.

### Pendência explícita

ADR sem consenso não bloqueia a sub-spec: é registrado como **pendente**, com
owner e prazo. É o mecanismo previsto no cenário de edge da umbrella.

## Decisões técnicas

- **Índice único em `docs/adr/`, faixa `010`–`028`**: as decisões do DMPF
  convivem com as do template num só lugar, então quem procura uma decisão de
  arquitetura do repositório tem um único índice para consultar. Alternativa
  descartada: diretório isolado (`docs/dmpf/adr/`) com numeração própria a
  partir de `001`, porque criaria dois índices concorrentes e obrigaria o
  leitor a saber de antemão em qual procurar.
- **Numeração atribuída na promoção, não no draft**: evita disputa de número
  entre drafts em revisão simultânea. Alternativa descartada: reservar o número
  ao criar o draft, porque um draft abandonado deixaria buraco na sequência.
- **ADR acionado pela sub-spec de origem, redigido aqui**: a sub-spec de origem
  nomeia o ADR e define seu assunto enquanto o contexto da decisão está vivo; a
  redação fica concentrada nesta sub-spec, que responde pelo formato e pela
  promoção. Alternativa descartada: escrever os 15 ao final sem acionamento
  prévio, porque vira exercício de arqueologia e as alternativas consideradas se
  perdem. Alternativa também descartada: cada sub-spec redigir os próprios ADRs,
  porque dispersa o formato e a numeração por onze histórias.
- **Alternativa genuína obrigatória**: ADR sem alternativa real é decisão
  narrada, não registrada. Alternativa descartada: aceitar a seção preenchida
  com opções nunca consideradas, porque destrói o valor do documento para quem
  reavaliar a decisão depois.
- **Pendência com owner e prazo em vez de bloqueio**: mantém o épico avançando.
  Alternativa descartada: exigir consenso pleno antes de prosseguir, porque uma
  decisão controversa travaria as onze sub-specs.
- **Um ADR por acionamento, e não consolidação para 15**: preserva as dezenove
  decisões que o acervo registrou enquanto o contexto estava vivo. As alternativas
  descartadas — consolidar para 15, ou ampliar para 24 acionando retroativamente
  as linhas órfãs — estão em «Reconciliação de cardinalidade».
- **Correspondência com a tabela §8 por assunto, não por número**: a numeração do
  épico e a de `docs/adr/` são séries distintas, e forçar identidade entre elas
  produziria colisão com os `001`–`009` do template. Alternativa descartada:
  renumerar a tabela do épico, porque ela é fonte externa e citada pelas stories
  de origem já concluídas.
- **Um ADR cujo conteúdo ainda é decisão aberta**: `ADR-DMPF-N` (registry de
  schemas em runtime) foi acionado sem que a decisão tenha sido tomada — o FND-06
  condicionou a operação a ele. É o primeiro candidato ao mecanismo de pendência
  com owner e prazo, e não deve bloquear a promoção dos outros dezoito.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] 19 ADRs promovidos para `docs/adr/010`–`028` e revisáveis em PR** (satisfaz o piso de «15 ADRs mínimos» do épico)
- [ ] **[P0] Cada ADR obrigatório aceito ou com pendência explícita (owner/prazo)**
- [ ] **[P0] `docs/adr/README.md` atualizado com as 19 entradas, e o índice referenciado pela RFC e pela umbrella**
- [ ] **[P0] Mapa de reconciliação publicado**: cada uma das 15 linhas da tabela §8 aponta o ADR que a cobre, ou declara por que não recebe ADR próprio; cada acionamento sem linha correspondente é nomeado
- [ ] **[P0] Nenhum ADR promovido citando outro por identificador provisório**: as referências cruzadas usam o número definitivo após a promoção

### Cenários de teste (mínimo 3)

```
DADO um ADR estrutural acionado por FND-02 e redigido nesta sub-spec
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

DADO uma linha da tabela ARQ-436 §8 sem acionamento correspondente no acervo
QUANDO o mapa de reconciliação é fechado
ENTÃO a linha aparece nele com decisão declarada — ADR redigido a partir do
     artefato normativo que cobre o assunto, ou registro de que a decisão vive
     no artefato e não recebe ADR próprio

DADO um artefato do acervo que cita um ADR por identificador provisório
QUANDO o ADR correspondente é promovido e recebe número definitivo
ENTÃO a citação é atualizada para o número, e nenhuma referência a
     `ADR-DMPF-<letra>` sobrevive apontando para ADR já promovido
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
  tocados; esta spec apenas acrescenta a faixa `010`–`028` e as entradas
  correspondentes no índice.
- **Ferramenta de gestão de ADRs**: os drafts são markdown; tooling é
  pós-fundação.
- **Promoção dos demais artefatos normativos**: RFC, inventário e políticas vão
  para `docs/dmpf/`, cada um pela sua sub-spec.
