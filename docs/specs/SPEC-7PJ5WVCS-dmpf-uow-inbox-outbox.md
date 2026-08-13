---
id: SPEC-7PJ5WVCS
slug: dmpf-uow-inbox-outbox
title: DMPF — Unit of Work, inbox, outbox e garantias de entrega
stage: backlog
priority: P0
depends_on: [SPEC-8MNDEWDP]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-441
subtask_urls: []
created: 2026-08-13
---
# SPEC-7PJ5WVCS: DMPF — Unit of Work, inbox, outbox e garantias de entrega

## Resumo

Entregar o item lógico **FND-04** do épico ARQ-436
(ARQ-441): produzir a evidência «Sequências transacionais, failure modes, idempotência, relay e disposições de consumo aprovados».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-441](https://lider-cap.atlassian.net/browse/ARQ-441)
- **ACs do épico**: AC-07
- **Evidência §11**: Sequências transacionais, failure modes, idempotência, relay e disposições de consumo aprovados

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] UoW explícita** no application service
- [ ] **[P0] Outbox**: estado de negócio + outbox na mesma transação
- [ ] **[P0] Inbox**: inbox + efeitos locais + outbox derivada na mesma transação de consumo
- [ ] **[P0] Relay**: claim por lease; sem lock de banco durante I/O no broker
- [ ] **[P0] Failure modes**: commit/publicação/ACK e recuperações com cenários verificáveis
- [ ] **[P0] Idempotência / DLQ / quarantine / replay**: regras de duplicidade e poison message

### Não-funcionais

- [ ] **[P0] Atomicidade sem transação distribuída**: as garantias são obtidas com transação local + outbox, nunca com two-phase commit sobre banco e broker
- [ ] **[P0] Ausência de lock durante I/O**: nenhuma sequência mantém transação de banco aberta enquanto aguarda o broker
- [ ] **[P0] Recuperação sem intervenção manual**: todo failure mode tem caminho de recuperação automático descrito; a intervenção manual é exceção documentada no runbook
- [ ] **[P1] Convergência**: duplicata reprocessada converge ao mesmo estado final observável

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [ ] | Permanece sem I/O; consome o resultado da UPR definida em FND-03 |
| Application service (UoW, orquestração) | [x] | Núcleo desta spec: define a fronteira transacional explícita |
| Port / Provider (adapters, drivers) | [x] | Define o contrato de relay, claim por lease e publicação |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [ ] | O envelope publicado é definido em FND-05 |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define ACK, redelivery e disposições de consumo |
| Observabilidade e operação | [x] | Define DLQ, quarantine e replay como operação |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `plans/references/Parte-1-conceitual.md` §§9–10 |
| Draft de trabalho | `plans/references/` — local, fora do versionamento |
| Promoção (ao ser aprovado) | `docs/dmpf/uow-inbox-outbox.md` — versionado e revisável por PR |

## Design

### Sequência de produção (outbox)

```
1. application service abre a UoW (transação local)
2. carrega estado, invoca a UPR, recebe Decision
3. grava estado de negócio  ──┐
4. grava linhas na outbox   ──┴─ MESMA transação
5. commit
6. relay (assíncrono) faz claim por lease, publica no broker, marca enviado
```

O passo 6 acontece **fora** da transação. Se o relay falhar após publicar e
antes de marcar, a mensagem é republicada — daí at-least-once.

### Sequência de consumo (inbox)

```
1. consumer recebe mensagem
2. abre transação local
3. grava na inbox a chave de idempotência   ──┐
4. aplica efeitos locais                     ──┼─ MESMA transação
5. grava outbox derivada (se houver)         ──┘
6. commit
7. ACK no broker
```

Se a chave já existe na inbox, o consumo é descartado como duplicata e o ACK
é emitido mesmo assim.

### Failure modes

| Falha | Momento | Recuperação |
|-------|---------|-------------|
| Crash antes do commit | passo 5 produção | nada persistiu; retry do caso de uso |
| Crash após commit, antes do relay | passo 6 produção | relay retoma pelo claim de lease |
| Publicou mas não marcou enviado | passo 6 produção | republicação; consumidor deduplica pela inbox |
| Crash após commit, antes do ACK | passo 7 consumo | redelivery; inbox descarta duplicata e emite ACK |
| Mensagem sempre falha | consumo | contador de tentativas → DLQ / quarantine |

### Relay

Claim por **lease** com prazo: o worker reivindica um lote, publica e libera.
Lease expirado volta ao pool. Nenhum lock de banco é mantido durante o I/O.

## Decisões técnicas

- **Outbox transacional em vez de publicação direta**: garante que estado e
  intenção de publicar commitam juntos. Alternativa descartada: publicar no
  broker dentro da transação, porque broker não participa da transação do banco
  e a falha entre commit e publish perderia o evento silenciosamente.
- **At-least-once com efeitos idempotentes**: é a semântica oficial, e a
  constraint P0 proíbe prometer exactly-once E2E. Alternativa descartada:
  exactly-once via transação distribuída, porque o custo operacional é alto e
  a garantia não sobrevive à fronteira de rede.
- **Claim por lease, não lock pessimista**: o relay não segura transação
  durante I/O. Alternativa descartada: `SELECT FOR UPDATE` mantido durante o
  publish, porque prende conexão do pool pela latência do broker e degrada sob
  carga.
- **Inbox como tabela de idempotência, não cache**: a deduplicação participa da
  mesma transação dos efeitos. Alternativa descartada: dedupe em cache externo,
  porque cache e banco podem divergir e reintroduzem a janela de duplicidade.
- **Poison message vai para quarantine, não descarte**: preserva a evidência
  para replay. Alternativa descartada: descartar após N tentativas, porque
  perde dado sem trilha de auditoria.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Sequências e failure modes AC-07 documentados no artefato promovido para `docs/dmpf/uow-inbox-outbox.md`**
- [ ] **[P0] At-least-once explícito; exactly-once E2E proibido na especificação**
- [ ] **[P0] Política de sagas/process managers referenciada (sem implementação)**

### Cenários de teste (mínimo 3)

```
DADO um caso de uso que grava estado de negócio e enfileira um evento na outbox
QUANDO o processo cai depois do commit e antes de o relay publicar
ENTÃO o relay retoma pelo claim de lease e publica o evento, sem perda

DADO um consumidor que já processou a mensagem de chave K e commitou a inbox
QUANDO a mesma mensagem K é redelivered pelo broker
ENTÃO o efeito não é reaplicado, o ACK é emitido e o estado final é o mesmo

DADO uma mensagem que falha em todas as tentativas configuradas
QUANDO o limite de tentativas é atingido
ENTÃO a mensagem vai para quarantine com a causa registrada e fica disponível
     para replay, sem ser descartada
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Implementação do relay e das tabelas**: aqui só a semântica; o código é
  dos épicos de kernel.
- **Escolha de broker e tuning**: convenções por transporte são de FND-06
  (SPEC-YWFGNPG5).
- **Formato do envelope publicado**: é de FND-05 (SPEC-7H08RZDG).
- **Implementação de sagas e process managers**: a política é referenciada,
  não implementada.
- **CDC como fonte da outbox**: permanece opt-in, fora da fundação.
