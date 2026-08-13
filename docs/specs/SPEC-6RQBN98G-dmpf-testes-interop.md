---
id: SPEC-6RQBN98G
slug: dmpf-testes-interop
title: DMPF — Estratégia de testes e interoperabilidade Go ↔ TypeScript
stage: backlog
priority: P0
depends_on: [SPEC-7PJ5WVCS, SPEC-7H08RZDG]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-446
subtask_urls: []
created: 2026-08-13
---
# SPEC-6RQBN98G: DMPF — Estratégia de testes e interoperabilidade Go ↔ TypeScript

## Resumo

Entregar o item lógico **FND-09** do épico ARQ-436
(ARQ-446): produzir a evidência «Pirâmide de testes, test kits e golden fixtures Go ↔ TypeScript especificados».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: [ARQ-446](https://lider-cap.atlassian.net/browse/ARQ-446)
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

- [ ] **[P0] Pirâmide**: domínio, services, providers, apps, fluxos distribuídos
- [ ] **[P0] Catálogo de cenários**: commit antes do ACK, redelivery, duplicata, poison, backpressure, graceful shutdown
- [ ] **[P0] Golden fixtures** Go ↔ TS: formato, ownership e pipeline
- [ ] **[P0] Verificações**: campos desconhecidos, versões suportadas, payload hash

### Não-funcionais

- [ ] **[P0] Fixture como fonte única**: o mesmo arquivo de fixture alimenta as suítes Go e TS; nenhuma stack mantém cópia própria
- [ ] **[P0] Determinismo**: nenhum cenário do catálogo depende de relógio de parede, ordem de map ou porta aleatória
- [ ] **[P0] Falha informativa**: divergência de interop aponta o campo divergente, não apenas hashes diferentes
- [ ] **[P1] Custo de execução**: a camada de fluxos distribuídos roda em pipeline separado, sem penalizar o feedback do domínio

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
| Observabilidade e operação | [ ] | Fora do escopo de teste desta spec |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `plans/references/Parte-1-conceitual.md` |
| Draft de trabalho | `plans/references/` (fixtures a definir) — local, fora do versionamento |
| Promoção (ao ser aprovada) | `docs/dmpf/testes-interop.md` — versionado e revisável por PR |

As **golden fixtures** em si não são documentação: ao saírem de draft, vivem no
repositório de contratos (FND-05), consumidas pelas suítes Go e TS. Aqui se
especifica formato, ownership e pipeline delas.

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

Derivados das sequências de FND-04:

1. **Commit antes do ACK** — estado commitado, processo cai antes do ACK.
2. **Redelivery** — mesma mensagem entregue de novo.
3. **Duplicata** — chave já presente na inbox.
4. **Poison** — mensagem que falha sempre, até quarantine.
5. **Backpressure** — produção acima da capacidade de consumo.
6. **Graceful shutdown** — encerrar sem perder trabalho em curso nem ACK indevido.

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

- **Fixture como fonte única para as duas stacks**: garante que Go e TS
  concordem sobre o mesmo byte. Alternativa descartada: cada stack manter suas
  fixtures, porque as duas passam verdes enquanto divergem entre si — que é
  exatamente a falha que o teste deveria pegar.
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

- [ ] **[P0] Estratégia de testes promovida para `docs/dmpf/testes-interop.md` e aprovada em PR**
- [ ] **[P0] Conjunto inicial de golden fixtures especificado (formato, ownership e pipeline)**
- [ ] **[P0] Cenários AC-10 cobertos no catálogo**

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

## Escopo fora

- **Implementação dos test kits**: aqui só a especificação; o código é dos
  épicos de kernel.
- **Escolha de framework de teste por stack**: cada stack usa o idiomático.
- **Testes de carga e performance**: a fundação não define metas de throughput.
- **Ambiente de integração**: provisionar o ambiente é de plataforma.
- **Cobertura mínima como meta numérica**: fica para a política de qualidade dos
  épicos de implementação.
