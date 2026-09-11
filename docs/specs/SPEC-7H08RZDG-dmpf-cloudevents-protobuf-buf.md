---
id: SPEC-7H08RZDG
slug: dmpf-cloudevents-protobuf-buf
title: DMPF — Perfil CloudEvents e governança Protobuf (Buf)
stage: done
priority: P0
depends_on: [SPEC-8MNDEWDP, SPEC-7PJ5WVCS]
ticket_url: null
subtask_urls: []
created: 2026-08-13
---
# SPEC-7H08RZDG: DMPF — Perfil CloudEvents e governança Protobuf (Buf)

## Resumo

Entregar o item lógico **FND-05** do épico ARQ-436
(ARQ-442): produzir a evidência «Perfil, nomenclatura, evolução, skeleton do repositório e checks Buf validados».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: ARQ-442
- **ACs do épico**: AC-05
- **Evidência §11**: Perfil, nomenclatura, evolução, skeleton do repositório e checks Buf validados

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Perfil CloudEvents**: atributos obrigatórios (payload, correlation, causation, tracing, tenant, partition key)
- [x] **[P0] Modalidade de payload** da primeira major version definida
- [x] **[P0] Evolução Protobuf**: proibir reuso de field numbers; breaking checks
- [x] **[P0] Buf no CI**: format, lint, breaking e geração determinística especificados
- [ ] **[P0] Exemplo interop**: serializar numa stack e desserializar na outra sem perda semântica

### Não-funcionais

- [ ] **[P0] Compatibilidade retroativa**: consumidor de uma versão anterior do contrato continua desserializando mensagens da versão seguinte dentro da mesma major
- [ ] **[P0] Geração determinística**: mesma entrada de `.proto` produz artefatos idênticos byte a byte, em Go e em TS
- [ ] **[P0] Verificação automática**: format, lint e breaking rodam em CI e barram o merge, sem depender de revisão humana
- [x] **[P1] Legibilidade do envelope**: os atributos obrigatórios são inspecionáveis sem desserializar o payload

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [ ] | Explicitamente fora: Protobuf não entra no domínio |
| Application service (UoW, orquestração) | [ ] | Não conhece o formato de wire |
| Port / Provider (adapters, drivers) | [x] | É onde a serialização acontece |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [x] | Núcleo desta spec: perfil, nomenclatura e evolução |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define o envelope que os transportes carregam |
| Observabilidade e operação | [x] | Define os atributos de tracing e correlation do envelope |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `Parte-1 §7`, na convenção que a RFC fixa em §1.3; a referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md`; docx Buf — rascunho local, fora do versionamento |
| Draft de trabalho | área local — fora do versionamento, não citável como fonte |
| Promoção (ao ser aprovado) | `docs/dmpf/cloudevents-protobuf-buf.md` — versionado e revisável por PR |

## Design

### Envelope CloudEvents — atributos obrigatórios

| Atributo | Papel |
|----------|-------|
| `payload` | corpo do evento, em Protobuf |
| `correlation id` | agrupa toda a cadeia originada de uma requisição |
| `causation id` | identifica a mensagem que causou esta |
| `tracing` | contexto de propagação (OpenTelemetry) |
| `tenant` | isolamento multi-tenant, obrigatório após autenticação |
| `partition key` | determina ordenação no broker |

### Evolução de contrato

Regras de compatibilidade dentro de uma major version:

- Adicionar campo opcional: **permitido**.
- Remover campo: **reservar** o número e o nome; nunca reutilizar.
- Renomear campo: equivale a remover e adicionar.
- Mudar tipo de um campo: **breaking**; exige nova major.

### Buf no CI

```
buf format --diff --exit-code   → estilo
buf lint                        → convenções de nomenclatura
buf breaking --against <base>   → compatibilidade contra a base
buf generate                    → artefatos Go e TS determinísticos
```

Os quatro rodam no CI do repositório de contratos e barram o merge.

### Skeleton do repositório de contratos

Estrutura inicial com um módulo Buf, diretórios por bounded context e
versionamento por major no path (`v1`, `v2`), de modo que duas majors
coexistam durante a migração.

## Decisões técnicas

- **CloudEvents como envelope, Protobuf como payload**: separa metadado de
  roteamento do corpo do evento. Alternativa descartada: envelope proprietário,
  porque perde a interoperabilidade e as ferramentas prontas do ecossistema.
- **Reuso de field number proibido, com reserva obrigatória**: o `reserved` do
  Protobuf é a única defesa contra desserialização silenciosamente errada.
  Alternativa descartada: confiar em revisão de código, porque o erro é
  invisível no diff e só aparece em produção como dado corrompido.
- **Breaking check contra a base em CI**: a compatibilidade é verificada
  mecanicamente. Alternativa descartada: política escrita sem verificação,
  porque não sobrevive à pressão de prazo.
- **Geração determinística versionada**: artefatos reproduzíveis permitem
  detectar drift entre o `.proto` e o código gerado. Alternativa descartada:
  geração local ad hoc por consumidor, porque cada squad passa a ter uma versão
  ligeiramente distinta do mesmo contrato.
- **Partition key no envelope, não inferida do payload**: torna a ordenação
  explícita e auditável. Alternativa descartada: derivar a chave de um campo do
  payload, porque acopla o roteamento à estrutura interna da mensagem.

## Verificação e testes

### Critérios de aceite

- [ ] **[P0] Perfil organizacional promovido para `docs/dmpf/cloudevents-protobuf-buf.md` e aprovado em PR**
- [x] **[P0] Skeleton/estrutura inicial do repo de contratos especificada**
- [x] **[P0] Checks Buf descritos de forma executável**

### Cenários de teste (mínimo 3)

```
DADO uma mensagem serializada por um produtor Go conforme o perfil
QUANDO desserializada por um consumidor TypeScript da mesma major version
ENTÃO todos os atributos obrigatórios e o payload são recuperados sem perda
     semântica, e o payload hash confere

DADO um .proto que adiciona um campo opcional a uma mensagem existente
QUANDO `buf breaking` roda contra a base
ENTÃO o check passa e o merge é permitido

DADO um .proto que reutiliza um field number previamente removido
QUANDO `buf breaking` roda contra a base
ENTÃO o check falha e o merge é barrado
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Criação do repositório de contratos**: aqui se especifica o skeleton; criar
  e popular é dos épicos de contratos.
- **Schema Registry em produção**: a convenção por broker é de FND-06
  (SPEC-YWFGNPG5); a operação é pós-fundação.
- **Geradores e templates de código**: pertencem aos épicos de tooling.
- **Migração dos contratos existentes**: o inventário está em FND-01; a
  migração é pós-fundação.
