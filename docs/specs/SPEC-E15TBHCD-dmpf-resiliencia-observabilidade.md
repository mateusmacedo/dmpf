---
id: SPEC-E15TBHCD
slug: dmpf-resiliencia-observabilidade
title: DMPF — Resiliência, observabilidade e operação
stage: done
priority: P0
depends_on: [SPEC-7PJ5WVCS, SPEC-7H08RZDG, SPEC-YWFGNPG5, SPEC-XQWGGAXF]
ticket_url: null
subtask_urls: []
created: 2026-08-13
---
# SPEC-E15TBHCD: DMPF — Resiliência, observabilidade e operação

## Resumo

Entregar o item lógico **FND-08** do épico ARQ-436
(ARQ-445): produzir a evidência «Políticas, fluxos de tracing, métricas, logs, DLQ/replay e runbook mínimo definidos».

## Contexto

- **Umbrella**: [SPEC-QG2N8STY](./SPEC-QG2N8STY-dmpf-foundation.md)
- **Issue**: ARQ-445
- **ACs do épico**: AC-09
- **Evidência §11**: Políticas, fluxos de tracing, métricas, logs, DLQ/replay e runbook mínimo definidos
- **Obrigações herdadas**: 30 predicados encaminhados a esta spec por cinco
  artefatos já promovidos em `develop` — FND-03 (2), FND-04 (12), FND-05 (4),
  FND-06 (4) e FND-07 (8). O critério de fronteira que governa todas é o
  enunciado por FND-04: **capacidade é do artefato de origem, catálogo é de
  FND-08** (`uow-inbox-outbox.md:167`). As quatro de FND-06 resolvem em
  `politicas-transporte.md:156`, `:189`, `:269` e `:1186`.

<constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Limites**: timeout, retry, circuit breaker, bulkhead, rate limiting, cache
- [ ] **[P0] Retry remoto ≠ reexecução** automática do caso de uso
- [ ] **[P0] Tracing**: fluxos e atributos mínimos OpenTelemetry
- [ ] **[P0] Métricas**: services, inbox, outbox, relay, consumers, DLQ, pools
- [ ] **[P0] Logging**: estruturado, redaction, sampling, auditoria separada
- [ ] **[P0] Decorators de saída**: os nove decorators de Parte-1 §12.2, cuja
  subseção inteira FND-07 encaminhou a esta âncora
  (`contexto-erros-seguranca.md:316`), com o sujeito declarado por bloco
- [ ] **[P0] Política de retryability**: quantas vezes se tenta, com que
  espaçamento, sob qual orçamento, com qual limiar de alarme e qual runbook
  (`contexto-erros-seguranca.md:231`)
- [ ] **[P0] Contenção de carga**: emitir a norma de mitigação do eixo *Denial of
  service* que FND-07 avaliou nos sete vetores e deixou de emitir
  (`THR-01`, `contexto-erros-seguranca.md:1380`)

### Não-funcionais

- [ ] **[P0] Limite obrigatório em toda saída**: nenhuma chamada a dependência externa fica sem timeout e sem política de falha declarada
- [ ] **[P0] Continuidade do trace**: o trace sobrevive ao salto assíncrono, ligando produtor e consumidor pelo envelope de FND-05
- [ ] **[P0] Diagnóstico sem acesso a dado sensível**: a operação consegue diagnosticar uma falha sem ler PII, apoiada em correlation id
- [ ] **[P1] Custo de observabilidade previsível**: sampling declarado por classe de tráfego, não por serviço individual

## Camadas afetadas

Esta é uma spec **normativa**: as camadas abaixo são as do DMPF que o
artefato descreve ou normatiza, não módulos de código a alterar.

| Camada | Normatizada? | Descrição |
|--------|--------------|-----------|
| Domínio (UPR, Decision, eventos) | [ ] | Permanece sem logger e sem I/O |
| Application service (UoW, orquestração) | [x] | Define a distinção entre retry remoto e reexecução do caso de uso |
| Port / Provider (adapters, drivers) | [x] | Define onde vivem timeout, circuit breaker e bulkhead |
| Contract / wire (Protobuf, CloudEvents, OpenAPI) | [ ] | Consome os atributos de tracing definidos em FND-05 |
| Transporte (REST, gRPC, Kafka, SNS/SQS) | [x] | Define o orçamento transversal de retry e backoff; os valores por transporte são de ANC-04 / FND-06 |
| Observabilidade e operação | [x] | Núcleo desta spec: métricas, tracing, logging e runbook |

## Localização de código

| Fase | Caminho |
|------|---------|
| Fontes | `Parte-1 §15` e as seções de operação e resiliência, na convenção de citação que a RFC fixa em §1.3; a referência versionada é `docs/dmpf/rfc-dmpf-foundation-v0.1.md`, que recepciona a base conceitual e declara o que dela já foi consolidado |
| Draft de trabalho | Fora do versionamento, local à máquina do autor — não é citável como fonte |
| Promoção (ao ser aprovado) | `docs/dmpf/resiliencia-observabilidade.md` — versionado e revisável por PR |

## Design

### Limites por dependência

| Mecanismo | Papel |
|-----------|-------|
| timeout | teto de espera por chamada |
| retry + backoff | tolera falha transitória |
| circuit breaker | para de chamar dependência degradada |
| bulkhead | isola pool para que uma dependência lenta não consuma todos os recursos |
| rate limiting | protege a dependência e o próprio serviço |
| cache | reduz pressão em leitura |

### Retry remoto ≠ reexecução do caso de uso

Retry de uma chamada a provider é retentativa **daquela chamada**, dentro da
mesma execução. Não reexecuta o caso de uso nem reabre a UoW. Reexecução do
caso de uso só ocorre por redelivery da mensagem, com a inbox de FND-04
garantindo idempotência. A base normativa é Parte-1 §12.2, que proíbe
reexecutar o service inteiro sem idempotência comprovada.

### Autorização de retry como conjunção

Retentar não é consequência da retryability. A autorização exige a conjunção de
quatro fatores: **erro retentável × operação idempotente ou com efeito
conhecidamente ausente × orçamento suficiente para a tentativa e o seu backoff ×
prazo remanescente**, verificados antes de **cada** tentativa. `ERR-12` é explícito ao separar
os dois atos — a classificação «não autoriza, ordena nem dimensiona a
repetição» (`contexto-erros-seguranca.md:1035`) —, e `ERR-11` fixa o default
fail-closed que esta spec apenas parametriza, sem substituir (`:1034`).

### Fronteira do rate limiting

O decorator de saída de Parte-1 §12.2 protege a dependência e é normatizado
aqui por sucessão da subseção. O lado de **entrada** — admissão e contenção de
pressão — é normatizado por outra via: `THR-01` encaminhou a mitigação do eixo
*Denial of service* a esta âncora. O estágio `rate limit/idempotency` de
Parte-1 §12.1 permanece **vigente**, não consolidado: `idempotency` é de FND-04
e a spec não o absorve.

### Tracing

Span na borda, propagação por todos os saltos síncronos e continuidade no salto
assíncrono via atributos do envelope (FND-05). O trace liga produtor e consumidor
através do broker **sob fronteira confiável**, no critério de `CTX-27` do FND-07;
fora dela o consumidor inicia trace novo e registra o valor recebido como
proveniência, nunca como parentesco.

### Catálogo de métricas

| Domínio | Métricas |
|---------|----------|
| services | latência, taxa de erro por categoria, throughput |
| inbox | duplicatas descartadas, lag de consumo |
| outbox | linhas pendentes, idade da mais antiga |
| relay | leases ativos, publicações por lote, falhas |
| consumers | redeliveries, tempo até ACK |
| DLQ | volume, idade, replays executados |
| pools | conexões em uso, saturação, espera |

"Idade da linha mais antiga da outbox" é o indicador primário de relay parado.

### Logging e runbook

Logging estruturado, redaction na origem (FND-07), sampling por classe de
tráfego e trilha de auditoria em canal separado do log operacional. O runbook
mínimo cobre inspeção de DLQ, decisão de replay e execução do replay.

Backlog de outbox, poison message, graceful shutdown e backpressure entram como
procedimentos **derivados** desse núcleo, cada um apontando a métrica ou o trace
que o dispara. Não são normas operacionais novas: graceful shutdown, em
particular, é propriedade do mecanismo em `OBX-13` e aqui é observado e tratado,
nunca redefinido.

## Decisões técnicas

- **Retry remoto separado de reexecução do caso de uso**: evita duplicar efeito
  de negócio ao tentar de novo uma chamada de rede. Alternativa descartada:
  reexecutar o caso de uso inteiro a cada falha de provider, porque multiplica
  efeitos colaterais já aplicados.
- **Circuit breaker no provider, não no application service**: o limite
  pertence à fronteira de I/O. Alternativa descartada: breaker no caso de uso,
  porque mistura política de infraestrutura com regra de aplicação.
- **Idade da outbox como sinal primário**: fila crescente indica relay parado
  antes de qualquer alarme de latência. Alternativa descartada: monitorar só a
  contagem de pendentes, porque volume alto e saudável é indistinguível de
  volume represado.
- **Auditoria em canal separado do log operacional**: retenções e controles de
  acesso diferentes. Alternativa descartada: log único, porque força a
  retenção longa da auditoria sobre todo o volume operacional.
- **Sampling por classe de tráfego**: mantém custo previsível preservando cauda
  de erro. Alternativa descartada: sampling uniforme, porque descarta
  justamente os traces raros que interessam ao diagnóstico.

## Verificação e testes

### Critérios de aceite

- [x] **[P0] Políticas de resiliência promovidas para `docs/dmpf/resiliencia-observabilidade.md` e aprovadas em PR**
- [x] **[P0] Catálogo de métricas/tracing/logging definido no artefato promovido**
- [x] **[P0] Runbook mínimo DLQ/replay documentado**

### Cenários de teste (mínimo 3)

```text
DADO um caso de uso cuja chamada a um provider falha de forma transitória
QUANDO o retry da chamada tem sucesso na segunda tentativa
ENTÃO o caso de uso conclui uma única vez, sem reabrir a UoW e sem duplicar
     efeitos já aplicados

DADO um evento publicado por um serviço e consumido por outro através do broker,
     sob fronteira confiável
QUANDO o trace é consultado pelo correlation id
ENTÃO produtor e consumidor aparecem no mesmo trace, apesar do salto assíncrono

DADO o mesmo evento entregue por fronteira NÃO confiável
QUANDO o consumidor o recebe
ENTÃO um trace novo é iniciado e o `traceparent` recebido é registrado como
     atributo de proveniência, não como parentesco

DADO o relay parado por falha de conectividade com o broker
QUANDO a idade da linha mais antiga da outbox ultrapassa o limite definido
ENTÃO a métrica dispara o alarme e o runbook indica a inspeção do relay
```

<critical_constraints>
- [P0] Domínio sem I/O: domínio NÃO importa Protobuf, ORM, broker, SDK cloud, HTTP, logger ou framework
- [P0] Protobuf apenas no wire — NUNCA como modelo interno de domínio
- [P0] Semântica oficial at-least-once com efeitos idempotentes — NUNCA prometer exactly-once E2E
- [P0] Kernels Go/TS, adapters e providers de produção ficam FORA deste épico (só fundação normativa)
</critical_constraints>

## Escopo fora

- **Dashboards e alertas em produção**: aqui se define o catálogo; construir
  painéis é operação pós-fundação.
- **Escolha de vendor de observabilidade**: a especificação é OpenTelemetry;
  o backend é decisão de plataforma.
- **Implementação dos mecanismos de resiliência**: pertence aos épicos de
  kernel e providers.
- **SLOs por serviço**: dependem do serviço concreto; a fundação define o que
  medir, não a meta.
- **Runbook completo de operação**: aqui só o mínimo de DLQ e replay, mais os
  derivados que se apoiam nele.
- **Redação, promoção e aceite de ADR**: esta spec aciona e nomeia a decisão
  arquitetural; redigir, promover e aceitar é de FND-11 (ARQ-448), conforme
  [SPEC-DBTRMM3X](./SPEC-DBTRMM3X-dmpf-adrs-minimos.md).
- **Valores de retry e backoff por transporte**: são de ANC-04 / FND-06; aqui
  fica o orçamento transversal.
