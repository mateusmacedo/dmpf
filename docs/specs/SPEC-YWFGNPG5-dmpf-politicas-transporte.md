---
id: SPEC-YWFGNPG5
slug: dmpf-politicas-transporte
title: DMPF — Políticas REST, gRPC, Kafka, SNS/SQS e AsyncAPI
stage: backlog
priority: P0
depends_on: [SPEC-7H08RZDG]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-443
subtask_urls: []
created: 2026-08-13
---
# SPEC-YWFGNPG5: DMPF — Políticas REST, gRPC, Kafka, SNS/SQS e AsyncAPI

## Resumo

Entregar o item lógico **FND-06** do épico ARQ-436
(ARQ-443): produzir a evidência «Matriz de transporte, contratos, deadlines, ordering, ACK, retry e documentação aprovada».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-443](https://lider-cap.atlassian.net/browse/ARQ-443)
- **ACs do épico**: AC-06
- **Evidência §11**: Matriz de transporte, contratos, deadlines, ordering, ACK, retry e documentação aprovada

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] REST/JSON/OpenAPI** como contrato externo
- [ ] **[P0] gRPC/Protobuf** síncrono interno com deadline e cancelamento
- [ ] **[P0] Kafka / SNS / SQS**: envelope, partition key, ordenação, ACK, retry
- [ ] **[P0] AsyncAPI** para canais e bindings assíncronos
- [ ] **[P0] Sem exactly-once E2E** na matriz de transporte

### Não-funcionais

- [ ] **[P0] Deadline obrigatório**: toda chamada síncrona interna carrega deadline propagado; chamada sem deadline é violação da política
- [ ] **[P0] Ordenação declarada**: cada canal declara se garante ordem e sob qual chave; "ordem global" nunca é prometida
- [ ] **[P0] Coerência com FND-05**: o envelope trafegado é exatamente o perfil CloudEvents definido, sem campos ad hoc por transporte
- [ ] **[P1] Documentação executável**: contratos publicados em OpenAPI e AsyncAPI, versionados junto ao código do adapter

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [ ] | Não conhece transporte |
| Application service (UoW, orquestração) | [ ] | Recebe contexto já normalizado pelo adapter |
| Port / Provider (adapters, drivers) | [x] | Define o comportamento exigido de cada adapter de transporte |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Define qual contrato vale em qual transporte |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Núcleo desta spec: a matriz de transporte |
| Observabilidade e operação | [x] | Define propagação de tracing e política de retry |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `plans/references/Parte-1-conceitual.md` §§8,10; docx Kafka Schema Registry |
| Draft de trabalho | `plans/references/` — local, fora do versionamento |
| Promoção (ao ser aprovada) | `docs/dmpf/politicas-transporte.md` — versionado e revisável por PR |

## Design

### Matriz de transporte

| Transporte | Uso | Contrato | Garantia |
|------------|-----|----------|----------|
| REST / JSON | externo (clientes, parceiros) | OpenAPI | request/response |
| gRPC / Protobuf | síncrono interno | `.proto` | request/response com deadline |
| Kafka | assíncrono, ordenado por chave | CloudEvents + Protobuf | at-least-once |
| SNS / SQS | assíncrono, fan-out e fila | CloudEvents + Protobuf | at-least-once |

Nenhuma linha promete exactly-once fim a fim.

### Síncrono interno (gRPC)

- **Deadline** obrigatório e propagado por toda a cadeia.
- **Cancelamento** propagado: cliente que desiste cancela o trabalho a jusante.
- Deadline restante diminui a cada salto; nenhum salto o reinicia.

### Assíncrono (Kafka, SNS/SQS)

- **Partition key** vem do envelope (FND-05), nunca inferida do payload.
- **Ordenação** garantida apenas dentro da partição, por chave.
- **ACK** só após o commit local do consumo (sequência de FND-04).
- **Retry** com backoff; esgotado o limite, DLQ ou quarantine.

### Documentação

REST em OpenAPI; canais assíncronos em AsyncAPI com bindings por broker.
Ambos versionados junto ao adapter que os implementa.

## Decisões técnicas

- **REST externo, gRPC interno**: JSON e OpenAPI onde há consumidor de
  terceiros; Protobuf e gRPC onde as duas pontas são nossas. Alternativa
  descartada: REST em todas as chamadas internas, porque paga custo de
  serialização e perde o contrato tipado sem ganho de interoperabilidade.
- **Deadline obrigatório em vez de timeout do cliente**: o deadline viaja com a
  chamada e encerra o trabalho a jusante. Alternativa descartada: timeout só no
  chamador, porque o servidor segue processando um pedido que ninguém espera.
- **Ordenação por chave, nunca global**: torna a garantia declarada e
  escalável. Alternativa descartada: prometer ordem total, porque exige
  partição única e destrói o throughput.
- **ACK após commit local**: alinha com a sequência de inbox de FND-04.
  Alternativa descartada: ACK ao receber, porque perde a mensagem se o processo
  cair antes de aplicar os efeitos.
- **AsyncAPI para canais**: dá aos fluxos assíncronos a mesma descoberta que o
  OpenAPI dá ao REST. Alternativa descartada: documentar tópicos em wiki,
  porque não é verificável nem versionado com o código.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Matriz de transporte promovida para `docs/dmpf/politicas-transporte.md` e aprovada em PR**
- [ ] **[P0] Convenções por broker documentadas no artefato promovido**
- [ ] **[P0] Alinhamento com perfil CloudEvents/Protobuf de FND-05**

### Cenários de teste (mínimo 3)

```
DADO uma cadeia de três chamadas gRPC internas com deadline de 2s na borda
QUANDO o segundo salto consome 1,5s
ENTÃO o terceiro salto recebe o deadline restante e não o reinicia, e a cadeia
     é cancelada em vez de exceder o limite da borda

DADO duas mensagens com a mesma partition key publicadas em sequência
QUANDO consumidas do mesmo tópico particionado
ENTÃO são entregues na ordem de publicação dentro daquela partição

DADO uma proposta de canal que declara entrega exactly-once fim a fim
QUANDO confrontada com a matriz de transporte
ENTÃO é rejeitada, e a política aplicável é at-least-once com consumo idempotente
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Implementação dos adapters**: aqui só a política; o código é dos épicos de
  kernel e providers.
- **Tuning de broker em produção**: dimensionamento de partições e retenção é
  operação, não fundação.
- **Semântica transacional do consumo**: é de FND-04 (SPEC-7PJ5WVCS); aqui só a
  regra de ACK que dela decorre.
- **Definição do envelope**: é de FND-05 (SPEC-7H08RZDG).
- **Políticas de resiliência (circuit breaker, bulkhead)**: são de FND-08
  (SPEC-E15TBHCD).
