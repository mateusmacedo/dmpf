---
id: SPEC-6RQBN98G
slug: dmpf-testes-interop
title: DMPF — Estratégia de testes e interoperabilidade Go ↔ TypeScript
stage: done
priority: P0
depends_on: [SPEC-7PJ5WVCS, SPEC-7H08RZDG]
ticket_url: null
subtask_urls: []
created: 2026-08-13
---
# SPEC-6RQBN98G: DMPF — Estratégia de testes e interoperabilidade Go ↔ TypeScript

## Resumo

Entregar o item lógico **FND-09** do épico ARQ-436
(ARQ-446): produzir a evidência «Pirâmide de testes, test kits e golden fixtures Go ↔ TypeScript especificados».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: ARQ-446
- **ACs do épico**: AC-10
- **Evidência §11**: Pirâmide de testes, test kits e golden fixtures Go ↔ TypeScript especificados

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Pirâmide**: domínio, services, providers, apps, fluxos distribuídos
- [x] **[P0] Catálogo de cenários**: commit antes do ACK, redelivery, duplicata concorrente, poison, graceful shutdown e **backpressure** — cada um com a proveniência citada. O backpressure fechou quando FND-08 (ARQ-445) publicou a regra de resultado sob saturação, durante esta entrega
- [x] **[P0] Golden fixtures** Go ↔ TS: formato de arquivo, oráculo executável, pipeline e diagnóstico — o conteúdo da fixture e a definição dos oráculos permanecem de FND-05 §8.4
- [x] **[P0] Verificações**: campos desconhecidos, versões suportadas, payload hash
- [x] **[P0] Verificações herdadas de FND-07**: isolamento por tenant (`IDN-11`..`IDN-14`, fail-closed), ciclo de vida e reuso do contexto (`CTX-15`..`CTX-17`), respeito a cancelamento e deadline (`CTX-21`, `CTX-22`), cifra em repouso (`DAT-08`, `DAT-10`) e teto de retenção (`DAT-14`) — roteadas a ANC-07 por FND-07 §4.4 e §11.2

### Não-funcionais

- [x] **[P0] Fixture como fonte única**: o mesmo arquivo de fixture alimenta as suítes Go e TS; nenhuma stack mantém cópia própria
- [x] **[P0] Determinismo**: nenhum cenário do catálogo depende de relógio de parede, ordem de map ou porta aleatória
- [x] **[P0] Falha informativa**: divergência de interop aponta o campo divergente, não apenas hashes diferentes
- [x] **[P1] Custo de execução**: a camada de fluxos distribuídos roda em pipeline separado, sem penalizar o feedback do domínio

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [x] | Define o teste em memória como base da pirâmide |
| Application service (UoW, orquestração) | [x] | Define teste com ports fakes e cenários transacionais |
| Port / Provider (adapters, drivers) | [x] | Define teste de contrato do provider contra o port |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Núcleo da interop: golden fixtures e payload hash |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define os cenários de fluxo distribuído |
| Contexto, identidade e dado (FND-07) | [x] | Verificação executável das regras roteadas a ANC-07: isolamento por tenant, ciclo de vida do contexto, cifra em repouso e teto de retenção |
| Observabilidade e operação | [ ] | Fora do escopo de teste desta spec. O que dependia de resiliência foi destravado pela publicação de FND-08 no caso do backpressure; catálogo de métricas e cadência de replay seguem de ANC-06, por inspeção ou evidência operacional |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `Parte-1`, na convenção que a RFC fixa em §1.3; a referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md` |
| Draft de trabalho | área local (fixtures a definir) — fora do versionamento, não citável como fonte |
| Promoção (ao ser aprovada) | `docs/dmpf/testes-interop.md` — versionado e revisável por PR |

As **golden fixtures** em si não são documentação: ao saírem de draft, vivem no
repositório de contratos (FND-05), consumidas pelas suítes Go e TS. Aqui se
especificam o formato de arquivo, o oráculo executável, o pipeline que o roda e o
diagnóstico que ele emite; o conteúdo obrigatório da fixture e a definição dos
três oráculos permanecem de FND-05 §8.4.

## Design

### Pirâmide

| Camada | Escopo | Infraestrutura |
|--------|--------|----------------|
| domínio | UPR e Decision | nenhuma — em memória |
| services | caso de uso com UoW | ports fakes |
| providers | contrato port ↔ implementação | dependência real ou container |
| apps | borda a borda de um serviço | serviço isolado |
| fluxos distribuídos | dois ou mais serviços via broker | ambiente integrado |

O teste de domínio precisar de mock de infraestrutura é sinal de violação do
limite de FND-02.

### Catálogo de cenários obrigatórios

A proveniência é declarada **por cenário**: cinco derivam de FND-04, cada um de
uma seção distinta, e o sexto não tem base no acervo.

| # | Cenário | Proveniência |
|---|---------|--------------|
| 1 | **Commit antes do ACK** — estado commitado, processo cai antes do ACK | FND-04 §7.3, failure modes 1 e 6 |
| 2 | **Redelivery** — mesma mensagem entregue de novo | FND-04 §7.3, failure mode 12 |
| 3 | **Duplicata concorrente** — duas transações disputando a mesma chave de inbox | FND-04 §7.3, failure mode 7 |
| 4 | **Poison** — mensagem que falha sempre, até quarantine | FND-04 §7.3, failure mode 9 |
| 5 | **Graceful shutdown** — encerrar sem perder trabalho em curso nem ACK indevido | FND-04 §5.3 e `OBX-13` |
| 6 | **Backpressure** — produção acima da capacidade de consumo | Ausente de FND-04; a regra de resultado vem de FND-08 §3.2/§3.4/§3.5 (`RES-08`, `RES-14`, `RES-16`, `RES-17`) |

O item 6 ficou **bloqueado** enquanto FND-08 não publicava a regra de resultado —
declarar o critério satisfeito por um slot vazio seria o próprio risco de «decisões
apenas documentais» que o épico nomeia. FND-08 foi publicado durante esta entrega e
fixou o desfecho sob saturação, então o item passou a **coberto**, por `CEN-44`.

### Golden fixtures Go ↔ TS

Arquivo de fixture como **fonte única**, consumido pelas duas stacks:

```
1. produtor Go serializa a partir da fixture   → bytes
2. consumidor TS desserializa os bytes         → estrutura
3. compara com a fixture: campos e payload hash
4. inverte os papéis (TS produz, Go consome)
5. ambas as direções devem conferir
```

### Verificações de contrato

- **Campos desconhecidos**: comportamento definido ao receber campo não previsto.
- **Versões suportadas**: quais majors cada fixture cobre.
- **Payload hash**: detecta divergência de serialização entre stacks.

## Decisões técnicas

- **Fixture como fonte única para as duas stacks**: garante que Go e TS concordem
  sobre o **conteúdo** — equivalência semântica campo a campo e igualdade do
  `payload_hash`, sempre e nas duas direções. Identidade de bytes é exigida apenas
  onde `ENV-24` a exige (publicação sem reserialização, contenção e replay),
  conforme FND-05 §8.3; `INT-05` restringe a comparação de bytes entre produtores
  independentes, não os outros dois oráculos. Alternativa descartada: cada stack
  manter suas fixtures, porque as duas passam verdes enquanto divergem entre si —
  que é exatamente a falha que o teste deveria pegar.
- **Interop testada nas duas direções**: Go→TS e TS→Go. Alternativa descartada:
  testar só uma direção, porque assimetrias de serialização de campo opcional e
  default aparecem apenas no sentido não testado.
- **Payload hash como detector, campo divergente como diagnóstico**: o hash
  acusa, a comparação estrutural explica. Alternativa descartada: só o hash,
  porque a falha vira "os bytes diferem" sem indicar onde.
- **Cenários derivados de FND-04, não inventados**: o catálogo espelha os
  failure modes já especificados. Alternativa descartada: catálogo próprio de
  testes, porque desalinha com a semântica que a fundação definiu.
- **Fluxos distribuídos em pipeline separado**: preserva o feedback rápido das
  camadas de baixo. Alternativa descartada: tudo no mesmo pipeline, porque a
  lentidão da camada integrada leva o time a ignorar a suíte inteira.

## Verificação e testes

### Critérios de aceite

- [x] **[P0] Estratégia de testes promovida para `docs/dmpf/testes-interop.md` e aprovada em PR**
      — o artefato está escrito e em `draft normativo`; este critério fecha no **aceite do PR**, não antes
- [x] **[P0] Conjunto inicial de golden fixtures especificado (formato de arquivo, oráculo executável, pipeline e diagnóstico; ownership conforme FND-05 §8.4)**
- [x] **[P0] Os seis cenários do AC-10 cobertos no catálogo, com proveniência citada** (o backpressure fechou com a publicação de FND-08)
- [x] **[P0] Verificações herdadas de FND-07 instanciadas** (isolamento por tenant, ciclo de vida do contexto, cifra em repouso e teto de retenção)

### Cenários de teste (mínimo 3)

```
DADO uma golden fixture de um integration event na major version corrente
QUANDO o produtor Go serializa e o consumidor TS desserializa, e depois os
     papéis se invertem
ENTÃO as duas direções conferem campo a campo e o payload hash é idêntico

DADO um consumidor que recebe uma mensagem com um campo desconhecido,
     adicionado por uma versão mais nova dentro da mesma major
QUANDO a mensagem é desserializada
ENTÃO o consumidor processa normalmente, conforme a política de campos
     desconhecidos, sem falhar

DADO um teste da camada de domínio que precisa de um mock de repositório
     para executar
QUANDO confrontado com a pirâmide
ENTÃO é sinalizado como violação do limite de FND-02, e o teste é reclassificado
     para a camada de services
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Reconciliações aplicadas

Quatro afirmações desta spec não sobreviveram ao confronto com o acervo mergeado
e foram corrigidas na mesma branch da entrega, cada uma citando o artefato irmão
que a força. O registro completo, com a afirmação original e o desfecho, está na
§3 de [`docs/dmpf/testes-interop.md`](../dmpf/testes-interop.md).

| # | O que mudou | Forçada por |
|---|-------------|-------------|
| 1 | Critério do round-trip: de «mesmo byte» como propriedade geral para os três oráculos, cada um no seu escopo | FND-05 §8.3, `INT-05` |
| 2 | Ownership da fixture: do arquivo, do oráculo executável, do pipeline e do diagnóstico — não do conteúdo | FND-05 §8.4 |
| 3 | Proveniência por cenário; «duplicata concorrente»; backpressure inicialmente bloqueado, fechado com a publicação de FND-08 | FND-04 §7.3, §5.3; FND-08 §3.2/§3.4/§3.5 |
| 4 | Escopo passa a carregar as verificações herdadas de FND-07 | FND-07 §4.4, §11.2 |

## Escopo fora

- **Implementação dos test kits**: aqui só a especificação; o código é dos
  épicos de kernel.
- **Escolha de framework de teste por stack**: cada stack usa o idiomático.
- **Testes de carga e performance**: a fundação não define metas de throughput.
- **Ambiente de integração**: provisionar o ambiente é de plataforma.
- **Cobertura mínima como meta numérica**: fica para a política de qualidade dos
  épicos de implementação.
