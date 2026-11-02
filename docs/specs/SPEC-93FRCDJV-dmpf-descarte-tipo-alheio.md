---
id: SPEC-93FRCDJV
slug: dmpf-descarte-tipo-alheio
title: DMPF — Descarte de tipo alheio em tópico multi-tipo, sem quarentena
stage: done
priority: P0
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-130/kernel-dmpf-descarte-de-tipo-alheio-em-topico-multi-tipo-sem
subtask_urls: []
created: 2026-11-01
---

# SPEC-93FRCDJV: Descarte de tipo alheio em tópico multi-tipo

## Resumo

O canal passa a declarar o conjunto fechado de tipos que carrega (`KFK-03b`), e o
consumidor, o subconjunto que assina. O adapter de consumo do kernel
(`kernelapp.Consumer`) reconhece o tipo pelo atributo CloudEvents `type` do envelope.
Quando o tipo está declarado no canal mas o consumidor não o assina, o adapter confirma
a mensagem sem efeito: sem `Handler`, sem inbox, sem quarentena e sem DLQ, e conta o
descarte numa série própria. Um tipo fora do conjunto do canal deixa de ser confirmado em
silêncio. Ele passa a ser contido como `undeclared-type`, que é o "schema desconhecido"
de §7.4.

Como contexto consumidor, quero assinar só parte dos tipos de um tópico multi-tipo, para
que os demais não encham a quarentena nem exijam um filtro escrito à mão em cada app.

Origem: o Gateway de antecipa-market (SPEC-N1X148AW, DEVS-84) consome uma parte dos tipos
do `partner.events`, que carrega 11 (SPEC-9TEQ9EV9, seção Integração). Para descartar o
resto, a spec dele planeja copiar o `Sink` local do `reservations` (`SPEC-N1X148AW:481`).

## Contexto

### Estado atual

Verificado no código de `lidercap-platform` em 2026-11-01, na `master` (`597b3cf4`). Os
caminhos `.go` são relativos a `libs/backend/go/`, salvo os que começam por `apps/` ou
`tools/`.

| Ponto | Evidência |
| ----- | --------- |
| O descarte existe só no app: o `Sink` do `reservations` compara `env.Type` com um tipo e chama `Ack`. Registra só log em debug, sem métrica, span, inbox ou contenção | `apps/backend/reservations/app/sink.go:26-37`; ligado em `apps/backend/reservations/app/wiring.go:150-154` |
| O canal declara um tipo e um contrato (`EventType`, `ContractRef`), e `Validate` exige os dois | `transport/channel/channel.go:133-148,165-170` |
| `EventTypeOf` monta o tipo de fio de um canal (`<tipo>.v<major>`) | `transport/channel/kafka_channel.go:42-46` |
| O `orders` publica `order-placed.v1` e `item-added.v1` no mesmo tópico, mas o canal declara só `order-placed` | `apps/backend/orders/provider/mapper.go:12-15`; `apps/backend/orders/app/catalog.go:15-17`; `apps/backend/reservations/app/catalog.go:22-24` |
| O tipo vem do atributo CloudEvents `type`, decodificado do valor do registro, não de header | `contracts/envelope/envelope.go:42-61,171`; `kafka/worker.go:206` |
| O adapter trata envelope inválido e origem fora da fronteira antes do `Handler`: nos dois casos, contém e depois confirma | `app/consumer.go:128-157` |
| Payload fora do contrato do `Handler` vira `Failure(Validation)` → R1×D4 → quarentena `terminal-failure` | `apps/backend/reservations/app/consumer.go:62-80`; `app/consumer.go:219-220` |
| Toda `Reason` vai para quarentena, e nenhuma `Reason` cobre tipo não declarado | `app/containment.go:28-34`; `ports/containment.go:9-18` |
| A coluna `quarantine.reason` é `text`, sem `CHECK` de valores | `postgres/inbox.sql:33-44` |
| A única métrica de consumo é `messaging.process.duration`, e o catálogo não tem série de descarte | `app/metrics.go:47-59`; `observability/metrics/catalog.go:51-66` |
| `kernelapp.Channel` só carrega `Address` e `Group`, para o span | `app/consumer.go:106-110` |
| Não há template de consumidor: o golden-path aponta o `consumer.go` do `reservations` e não cita o `sink.go` | `tools/dmpf-plugin/templates/ai/.agents/skills/dmpf-bounded-context/references/golden-path.md__tmpl__:145` |

### Norma

- `KFK-03` (`docs/dmpf/politicas-transporte.md:1112`): por padrão, um tópico carrega um
  tipo. `KFK-03b` (`:1114`): para carregar vários, é preciso declarar um conjunto fechado
  na catalogação do canal, modelar a variação no contrato e garantir uma ordenação válida
  para o conjunto. Hoje o kernel não tem onde declarar esse conjunto.
- §7.4 (`docs/dmpf/uow-inbox-outbox.md:1588-1595`): envelope inválido ou schema
  desconhecido não recebe disposição e vai para quarantine ou DLQ. `INB-10` (`:1265`): o
  que se decide no passo 1 de §6.3 fica fora das sete disposições.
- Nenhuma regra trata o tipo **conhecido** que o consumidor não assina. Num consumidor sem
  `Sink` local, esse tipo cai em R1×D4 e vai para a quarentena. Com o `Sink`, recebe `Ack`
  sem rastro.
- O DEVS-130 lista `CapabilityRolloutChanged` entre os tipos do Gateway, mas a
  SPEC-N1X148AW (decisão 4, `:61`) diz que o Gateway não o consome. O caso desta spec não
  muda por isso.

### Alternativas avaliadas

| Alternativa | Por que não |
| ----------- | ----------- |
| Manter o `Sink` local por app, como no `reservations` | Cada consumidor reescreve o filtro, e o descarte não é medido. Um tipo desconhecido também recebe `Ack` em silêncio, contra §7.4 |
| Descartar todo tipo fora do subconjunto assinado, sem conjunto declarado no canal | Não distingue tipo alheio de tipo desconhecido: um erro do produtor sumiria sem rastro |
| Oitava disposição (`Discarded`), devolvida pelo `Handler` | §6.4 é fechada em sete. O `Handler` roda depois do passo 1, quando `Receipt` e contexto já foram montados. `INB-10` põe fora das disposições o que não chega à inbox |
| Quarentena com `Reason` própria | Tipo alheio não é defeito: tráfego legítimo encheria a quarentena, que é o que o DEVS-130 recusa |

<constraints>
- [P0] O descarte de tipo alheio confirma a mensagem (`Ack`) sem chamar o `Handler`, sem `Inbox.Register`, sem `Containment.Quarantine` e sem DLQ.
- [P0] Só é alheio o tipo declarado no conjunto do canal e fora do subconjunto do consumidor. Tipo fora do conjunto do canal é contido como `undeclared-type`, nunca descartado.
- [P0] A fronteira (`untrusted-boundary`) é avaliada antes do tipo: mensagem de origem fora da fronteira é contida mesmo que o tipo seja alheio.
- [P0] As sete disposições de §6.4 não mudam. O descarte não é disposição (`INB-10`).
- [P1] Todo descarte confirmado incrementa `dmpf.consumer.discarded`, cuja cardinalidade fica limitada ao conjunto declarado do canal.
- [P1] Nenhum contexto filtra tipo no bloco `app`: o filtro é do kernel.
</constraints>

## Requisitos

### Funcionais

#### Onda 0 — norma e ADR (antes do código)

- [x] **[P0] ADR-072**: registrar a decisão em `docs/adr/072-descarte-de-tipo-alheio-no-consumo.md`, com a tabela de alternativas desta spec, e indexá-lo em `docs/adr/README.md`. O ADR cita ADR-036, ADR-039 e SPEC-93FRCDJV.
- [x] **[P0] `KFK-03c`**: em `docs/dmpf/politicas-transporte.md`, depois de `KFK-03b`, uma regra normativa com três partes:
  - o consumidor de tópico multi-tipo declara o subconjunto que assina, contido no conjunto do canal;
  - tipo do conjunto e fora do subconjunto é descartado: confirmado sem efeito, sem inbox, sem quarantine nem DLQ, e contado;
  - tipo fora do conjunto é schema desconhecido (§7.4).
- [x] **[P0] §7.4 e `INB-10`**: em `docs/dmpf/uow-inbox-outbox.md`, duas mudanças:
  - nova linha na tabela de §7.4: "Tipo declarado no canal e não assinado pelo consumidor" \| **nenhuma**, descartado no passo 1 de §6.3 \| confirma sem efeito e conta o descarte, nunca quarantine nem DLQ;
  - menção ao descarte no texto de `INB-10`.
- [x] **[P0] `MET-21`**: em `docs/dmpf/resiliencia-observabilidade.md`, a linha `dmpf.consumer.discarded` na tabela do catálogo de consumo (contador, `{message}`, sem limiar).

#### Onda 1 — kernel

- [x] **[P0] Conjunto do canal**: em `transport/channel`, criar o tipo `Event{Type, ContractRef string}` e o campo `Channel.Events []Event`, que substitui `EventType` e `ContractRef`.
  - `Validate` recusa `Events` vazio, `Type` ou `ContractRef` vazios e `Type` repetido. A recusa usa `ErrMissingItem`, com mensagem que nomeia o canal e o item, no estilo de `channel.go:163-171`.
  - `ContractMajor` e `Ordering` continuam no canal e valem para o conjunto inteiro (`KFK-03b`).
  - Nova assinatura: `NewKafka(name, address, dlq, group string, events ...Event) Channel`.
  - `WireTypes(ch Channel) []string` substitui `EventTypeOf` e devolve `<Type>.v<ContractMajor>` para cada evento, na ordem declarada.
- [x] **[P0] Assinatura do consumidor**: dois campos novos.
  - `kernelapp.Channel.Carries []string`: os tipos de fio do canal, vindos de `channel.WireTypes`.
  - `kernelapp.Consumer.Accepts []string`: os tipos de fio que o `Handler` trata.
  - `Consumer.Validate() error` devolve `ErrIncompleteConsumer` quando `Carries` ou `Accepts` estão vazios. Devolve o novo `ErrSubscriptionOutsideChannel` quando um tipo de `Accepts` está fora de `Carries` ou se repete.
  - `Consume` recusa com o mesmo erro, como já faz com os campos obrigatórios (`consumer.go:125-127`). O composition root chama `Validate` na partida.
- [x] **[P0] Ordem no `Consume`**: envelope inválido → fronteira → tipo fora de `Carries` → tipo fora de `Accepts` → `admitted`. Os dois primeiros passos não mudam.
- [x] **[P0] Tipo não declarado**: criar `ports.ReasonUndeclaredType = "undeclared-type"` e mapeá-lo para `MechanismQuarantine` em `ContainmentMap`.
  - A mensagem é contida com `Delivery.Raw` (`GAR-07`) e só depois confirmada.
  - Tem o mesmo span `process`, a mesma `messaging.process.duration` e o mesmo `message consumed` do `untrusted-boundary` (`consumer.go:141-157`).
- [x] **[P0] Descarte**: tipo em `Carries` e fora de `Accepts` recebe `Ack`, e o `Consume` volta `Outcome{Discarded: true}`, com `Classified: false`.
  - O descarte não monta `Receipt`, não reconstrói o contexto de execução, não abre o span `process` e não registra `messaging.process.duration`.
  - Depois do `Ack` bem-sucedido, incrementa `dmpf.consumer.discarded` e emite o log em debug `delivery of another event type discarded`, com `cloudevents.event_type` e `messaging.message.id`.
  - Se o `Ack` falha, o `Consume` devolve o erro sem incrementar a série, e o transporte segue a política que já tem para gesto falho.
- [x] **[P0] Ponte do kernel**: `kernelapp.Sink{Consumer Consumer}`, com `Handle(ctx, raw []byte, attempt int, ack ports.Acknowledger) error`. Satisfaz `kafka.Sink` e `sqs.Sink` e repassa a `Consume`, sem filtro próprio.
- [x] **[P1] Série no catálogo**: em `observability/metrics/catalog.go`:
  - nome: `metrics.ConsumerDiscardedTotal = "dmpf.consumer.discarded"`;
  - tipo `Counter`, unidade `UnitMessage`, owner `serviço`;
  - labels: `messaging.system`, `messaging.destination.name`, `messaging.consumer.group.name` e `cloudevents.event_type`.
- [x] **[P2] Documentação do módulo**:
  - em `libs/backend/go/app/README.md`, acrescentar à tabela de efeitos (`:40-48`) as linhas "tipo fora do canal" (`Ack` após conter, `undeclared-type`) e "tipo não assinado" (`Ack`, sem contenção, contado), e descrever a nova ordem do passo 1;
  - em `libs/backend/go/transport/README.md`, documentar o `Events` do canal e o `WireTypes`.

#### Onda 2 — apps e instruções

- [x] **[P0] `orders`**: `OrdersChannel` declara `order-placed` (`company/orders/event/v1/order_placed.proto#OrderPlaced`) e `item-added` (`company/orders/event/v1/item_added.proto#ItemAdded`), que o relay já publica.
- [x] **[P0] `reservations`**:
  - o canal `orders.events` declara os mesmos dois eventos, e o consumidor declara `Accepts` com `com.company.orders.order-placed.v1`;
  - o wiring passa `kernelapp.Sink` ao `kafka.Consumer`;
  - `app/sink.go` e `app/sink_test.go` saem do app (via `trash`);
  - `distkit/roles.go:90-100` adota `Events`.
- [x] **[P0] Canais dos demais produtores**: `ReservationsChannel` declara `reservation-confirmed` e `reservation-cancelled`. O canal do `bookings` declara `booking-reserved`, `booking-cancelled` e `resource-registered`. Os dois listam o que os mappers publicam (`apps/backend/<app>/provider/mapper.go`).
- [x] **[P0] Demais chamadores**: o `bookings` e os testes adotam `NewKafka(..., events...)` e `WireTypes`, e não resta nenhum uso de `EventType` nem de `EventTypeOf`. Hoje são 14 ocorrências de `NewKafka(` e `EventTypeOf` fora de `kafka_channel.go`, testes incluídos.
- [x] **[P1] Instruções de IA**:
  - em `tools/dmpf-plugin/templates/ai/.claude/rules/dmpf-bounded-context.md__tmpl__`, na seção de consumo (`:92`), registrar que o canal declara o conjunto (`Events`), que o consumidor declara `Accepts`, que o kernel descarta o tipo alheio e que o app não filtra tipo;
  - registrar a mesma regra no `golden-path.md__tmpl__`, junto do molde (`:145`);
  - renderizar as cópias com o `init`.
- [x] **[P2] SPEC-Z2HM6NAP**: o item 2 ("Canal declara todos os eventos que carrega") passa a remeter a esta spec.

#### Onda 3 — publicação

- [x] **[P1] Release `1.0.0-rc.8`**: publicada pela sequência de `CONTRIBUTING.md` (PRs #73, #74 e #75). Saiu como release única com a Onda 2 da SPEC-CWBA4BPV, rastreada pela DEVS-95, e não em issue própria. Os módulos do kernel têm a tag `v1.0.0-rc.8`, e o `@mateusmacedo/dmpf-plugin@1.0.0-rc.8` foi publicado. É essa release que desbloqueia a adoção pelo Gateway (DEVS-84).

### Não-funcionais

- [x] Custo: o descarte não toma conexão do pool, não abre transação e não aloca `Receipt`. O reconhecimento é uma busca linear sobre `Carries` e `Accepts`, sem I/O. Um teste com dublês prova zero chamadas a `Handler`, `Containment` e `Inbox`.
- [x] Cardinalidade: na série de descarte, `cloudevents.event_type` só assume valores de `Carries`. Um tipo não declarado nunca entra na série.
- [x] Compatibilidade: a API pública de `transport/channel` e de `app` muda, e a mudança sai numa nova pré-release. O consumidor externo se ajusta na adoção (ADR-060).
- [x] Transporte: o comportamento é o mesmo em Kafka (o offset entra em `commitContiguous`, `kafka/worker.go:262`) e em SQS (`DeleteMessage`, `sqs/acknowledger.go:41`), porque o gesto é o `Ack` de sempre.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| `domain`, `application` | [ ] | Sem mudança. As sete disposições ficam como estão |
| `ports` | [x] | `ReasonUndeclaredType` |
| `transport/channel` | [x] | `Event`, `Events`, `NewKafka`, `WireTypes` e `Validate` |
| `app` (kernel) | [x] | `Accepts`, `Carries`, `Validate`, ordem do passo 1, descarte, `Sink` e a contenção nova |
| `observability/metrics` | [x] | `dmpf.consumer.discarded` no catálogo |
| `kafka`, `sqs` | [ ] | Sem mudança de código. Ganham testes de commit e de delete do descarte |
| `postgres` | [x] | `InboxSignals` conta `undeclared-type` em campo próprio (GAR-12). `quarantine.reason` é `text` sem `CHECK`, então não há migração |
| Apps `orders`, `reservations`, `bookings` | [x] | Catálogo com `Events`. O `reservations` perde o `Sink` local |
| Instruções de IA (`tools/dmpf-plugin/templates/ai`) | [x] | Regra do contexto e golden-path |
| Norma (`docs/dmpf`, `docs/adr`) | [x] | `KFK-03c`, §7.4, `INB-10` e ADR-072 |

## Localização de código

```text
libs/backend/go/
  transport/channel/channel.go        — Event, Events, Validate
  transport/channel/kafka_channel.go  — NewKafka variádico, WireTypes
  ports/containment.go                — ReasonUndeclaredType
  app/containment.go                  — undeclared-type → quarantine
  app/consumer.go                     — Accepts, Carries, Validate, ordem do passo 1, Outcome.Discarded
  app/discard.go                      — descarte: Ack, série e log
  app/sink.go                         — kernelapp.Sink
  app/metrics.go                      — instrumento da série de descarte
  observability/metrics/catalog.go    — ConsumerDiscardedTotal
  postgres/signals.go                 — InboxSignals.UndeclaredTypes
  app/README.md, transport/README.md  — documentação
apps/backend/orders/app/catalog.go
apps/backend/reservations/app/catalog.go, wiring.go, consumer.go
apps/backend/reservations/app/sink.go, sink_test.go   — removidos
apps/backend/reservations/distkit/roles.go
apps/backend/bookings/app/catalog.go
docs/adr/072-descarte-de-tipo-alheio-no-consumo.md, docs/adr/README.md
docs/dmpf/politicas-transporte.md, docs/dmpf/uow-inbox-outbox.md
tools/dmpf-plugin/templates/ai/.claude/rules/dmpf-bounded-context.md__tmpl__
tools/dmpf-plugin/templates/ai/.agents/skills/dmpf-bounded-context/references/golden-path.md__tmpl__
docs/specs/SPEC-Z2HM6NAP-endurecimento-borda-kernel.md
```

## Design

### Fluxo principal

1. O transporte entrega o registro ao `kernelapp.Sink`, que chama `Consume`.
2. `Consume` valida o consumidor. Consumidor inválido devolve erro, sem gesto.
3. Se `envelope.Unmarshal` falha, contém como `invalid-envelope` (sem mudança).
4. Se `env.Source` está fora da fronteira, contém como `untrusted-boundary` (sem mudança).
5. Se `env.Type` está fora de `Carries`, contém como `undeclared-type` e depois confirma.
6. Se `env.Type` está fora de `Accepts`, confirma, incrementa a série, registra o log e volta `Discarded`.
7. Nos demais casos, segue para `admitted`, como hoje.

### Pseudocódigo

```text
consume(delivery):
  validate(consumer)                    ; ErrIncompleteConsumer | ErrSubscriptionOutsideChannel
  env, err = unmarshal(delivery.raw)
  if err: return contain(invalid-envelope)
  if !boundary.admits(env.source): return contain(untrusted-boundary)
  if env.type not in channel.carries: return contain(undeclared-type)
  if env.type not in accepts:
      if err = ack(): return err
      discarded.add(1, system, destination, group, env.type)
      debug("delivery of another event type discarded", env.type, env.id)
      return Outcome{Discarded: true}
  return admitted(delivery, env)
```

## Decisões técnicas

- **Conjunto no canal e subconjunto no consumidor**: o canal é onde `KFK-03b` manda declarar o conjunto, e o consumidor só sabe o que trata. Separar as duas declarações distingue o tipo alheio, que é descartado, do desconhecido, que é contido.
  Alternativa descartada: só `Accepts`, que descartaria um erro de produtor sem deixar rastro.
- **Descarte no adapter, no passo 1**: fica junto das outras decisões tomadas sem inbox (`INB-10`) e vale igual para Kafka e SQS.
  Alternativa descartada: filtro no `kafka.Worker`, que deixaria o SQS de fora e decodificaria o envelope duas vezes.
- **Sem span `process` no descarte**: o descarte não processa a mensagem. O sinal operacional é a série, e o rastro é o log em debug. No Gateway, a maior parte dos tipos do `partner.events` é alheia, e abrir um span por descarte custaria um span para cada uma dessas mensagens.
- **`undeclared-type` vai para a quarentena**: segue o `ContainmentMap` (`app/containment.go:28-34`). Nele, toda `Reason` vai para a quarentena enquanto o DLQ (`KRN-10`) não tem realização no adapter.
- **`Events` substitui `EventType`**: um campo só, que no caso de `KFK-03` tem um elemento, evita dois caminhos de declaração.
  Alternativa descartada: manter `EventType` como tipo principal e acrescentar uma lista de extras. O canal multi-tipo ficaria com um tipo privilegiado sem razão normativa.
- **`Accepts` com tipos de fio (com major)**: é o que `env.Type` traz. Um `.v2` não declarado no canal cai em `undeclared-type`, como pede `PTB-03`.

## Regras relacionadas

- `docs/dmpf/politicas-transporte.md`: `KFK-03`, `KFK-03b`, `ASY-02` e `ASY-03`.
- `docs/dmpf/uow-inbox-outbox.md`: §6.3, §6.4, §7.4, `INB-08`, `INB-10`, `GAR-07` e `GAR-11`.
- ADR-036 (recepção como tipo fechado), ADR-039 (ponte por `Sink`), ADR-041 (assinatura por tipo na ponte, supersedida em parte pelo ADR-072) e ADR-060 (consumo externo por versão).
- SPEC-Z2HM6NAP, item 2: absorvido por esta spec.
- SPEC-9TEQ9EV9 (Integração) e SPEC-N1X148AW (consumo do `partner.events`): o caso de origem, em antecipa-market.

## Verificação e testes

### Critérios de aceite

- [x] Consumidor com `Accepts = [A]` sobre canal com `Carries = [A, B]`: a mensagem `B` recebe 1 `Ack`, e há 0 chamadas ao `Handler`, 0 a `Containment.Quarantine` e 0 a `Inbox.Register`. `dmpf.consumer.discarded` soma 1, com `cloudevents.event_type = B`.
- [x] Mensagem `C` fora de `Carries`: 1 quarentena com `reason = undeclared-type` e `Envelope` igual a `Delivery.Raw` byte a byte, seguida de 1 `Ack`. A série de descarte não muda.
- [x] Mensagem `B` de origem fora da fronteira: é contida como `untrusted-boundary`, e não descartada.
- [x] O descarte não registra `messaging.process.duration` nem abre span `process`.
- [x] `Ack` falho no descarte: `Consume` devolve o erro, e a série não muda.
- [x] `Accepts` vazio, repetido ou fora de `Carries`: `Validate` e `Consume` recusam, e o `reservations` não parte.
- [x] `channel.Validate` recusa `Events` vazio, `Type` repetido e `ContractRef` vazio.
- [x] Kafka: no e2e do `reservations` com Kafka real, um `item-added.v1` seguido de um `order-placed.v1` na mesma partição produz a reserva. Ou seja, o registro descartado recebeu gesto e não travou a partição (`kafka/worker.go:139-196`). A prova não fica em `kafka/consumer_test.go` porque `kafka` (`layer:providers`) não importa `app` (`layer:apps`).
- [x] SQS: a prova é por composição. O teste do kernel mostra descarte → `Ack`, e o `Ack` do SQS é `DeleteMessage` (`sqs/acknowledger.go:41`), já coberto pelos testes do módulo.
- [x] `reservations`: o `item-added.v1` publicado pelo `orders` é descartado e contado, e o `order-placed.v1` segue o fluxo de hoje (teste de integração do app). O `TestPayloadOfAnotherContractIsTerminal` mantém a intenção (tipo assinado com payload de outro contrato → R1×D4), agora com `order-placed.v1` e payload `ItemAdded`, porque o `item-added.v1` passa a ser descartado.
- [x] Não resta no workspace nenhum uso de `EventTypeOf` nem de `Channel.EventType`, e o arquivo `apps/backend/reservations/app/sink.go` não existe mais.
- [x] `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` passam.
- [x] A validação do projeto passa: `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source`, além de `fmt-check`, `vet` e `test-race` nos projetos afetados.

### Cenários de teste

```text
DADO um consumidor com Accepts [com.company.orders.order-placed.v1]
  E o canal com Carries [com.company.orders.order-placed.v1, com.company.orders.item-added.v1]
QUANDO chega um envelope válido do tipo com.company.orders.item-added.v1, com origem dentro da fronteira
ENTÃO Ack é chamado 1 vez, e Handler, Quarantine e Register são chamados 0 vezes
  E dmpf.consumer.discarded soma 1, com cloudevents.event_type = com.company.orders.item-added.v1
  E nenhum span process é aberto

DADO o mesmo consumidor
QUANDO chega com.company.orders.order-placed.v2 (major fora do canal)
ENTÃO a mensagem é contida com reason undeclared-type e Envelope igual ao recebido
  E o Ack vem depois da quarentena
  E a série de descarte não muda

DADO o mesmo consumidor
QUANDO chega com.company.orders.item-added.v1 com source fora da fronteira declarada
ENTÃO a mensagem é contida com reason untrusted-boundary
  E a série de descarte não muda

DADO o mesmo consumidor, com um Acknowledger cujo Ack falha
QUANDO chega com.company.orders.item-added.v1
ENTÃO Consume devolve o erro do Ack
  E a série de descarte não muda

DADO um consumidor com Accepts [com.company.orders.order-cancelled.v1], fora de Carries
QUANDO o composition root chama Validate na partida
ENTÃO volta ErrSubscriptionOutsideChannel
  E o processo não parte
```

<critical_constraints>
- [P0] O descarte de tipo alheio confirma a mensagem (`Ack`) sem chamar o `Handler`, sem `Inbox.Register`, sem `Containment.Quarantine` e sem DLQ.
- [P0] Só é alheio o tipo declarado no conjunto do canal e fora do subconjunto do consumidor. Tipo fora do conjunto do canal é contido como `undeclared-type`, nunca descartado.
- [P0] A fronteira (`untrusted-boundary`) é avaliada antes do tipo: mensagem de origem fora da fronteira é contida mesmo que o tipo seja alheio.
- [P0] As sete disposições de §6.4 não mudam. O descarte não é disposição (`INB-10`).
- [P1] Todo descarte confirmado incrementa `dmpf.consumer.discarded`, cuja cardinalidade fica limitada ao conjunto declarado do canal.
- [P1] Nenhum contexto filtra tipo no bloco `app`: o filtro é do kernel.
</critical_constraints>

## Escopo fora

- **Adoção no Gateway (antecipa-market)**: duas mudanças ficam em antecipa-market, depois da `1.0.0-rc.8`. Uma é trocar o `Sink` local planejado (`SPEC-N1X148AW:481`) por `Accepts`. A outra é declarar os tipos do `partner.events` no canal do Partner (DEVS-171).
- **DLQ para `undeclared-type`**: o `ContainmentMap` manda toda `Reason` para a quarentena até o `KRN-10` existir. Mudar o mecanismo é decisão desse item.
- **Filtro no broker**: assinar por header ou por partição exigiria o tipo fora do valor do registro (`kafka/worker.go:206`). Isso não entra nesta versão do kernel.
- **Demais itens da SPEC-Z2HM6NAP**: amostragem, drenagem, classe de tráfego e orçamento operacional continuam naquela spec.
