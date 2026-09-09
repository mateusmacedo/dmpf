---
id: SPEC-F7S5B6KV
slug: dmpf-generator-integracao-fechamento
title: DMPF KRN-12.2c — Integração por contrato gerado e fechamento da prova
stage: deferred
priority: P2
depends_on: [SPEC-VZ16X0MS]
ticket_url: null
subtask_urls: []
created: 2026-09-09
---

# SPEC-F7S5B6KV: DMPF KRN-12.2c — Integração por contrato gerado e fechamento da prova

## Resumo

> **Deferida em 2026-09-09.** O generator orientado ao domínio foi preterido pelo
> caminho híbrido AI SDD: esqueleto determinístico pelo generator existente e
> código de negócio escrito por agentes a partir da spec do bounded context
> (`SPEC-SPEC-VDP9XX65`). Esta spec fica como caminho avaliado — duas revisões
> externas (24 achados) documentam o custo do determinístico — e pode ser
> retomada se a geração por agentes não sustentar os gates.

Terceira sub-spec da reescrita do generator `bounded-context`: fecha o ponta a
ponta entre contextos. Para cada evento em `integration.publishes`, o
generator escreve a **fonte** `.proto` em `contracts/proto/company/<goIdent>/event/v1/`,
o mapper de domínio → message no provider e a publicação pela outbox; para
cada `integration.consumes`, o consumer que decodifica o envelope, aplica as
sete disposições e dispara o comando local. `gen/go` continua produto do rito
Buf, nunca do generator — a instrução final manda rodá-lo. Depois, a prova
mecânica ganha as fases `integration` e `self-test` sobre `bookings.jsonc`,
entra no CI em três passos, e a documentação operacional (guia de composição,
`AGENTS.md`, `tasks.md`, README do plugin) reflete o generator orientado ao
domínio.

Como squad, quero que `BookingReserved` chegue à outbox como CloudEvent com
payload protobuf gerado do meu contrato e que outro contexto o consuma pela
inbox — sem eu escrever proto, mapper ou consumer à mão.

## Contexto

- **Problema**: um integration event exige payload protobuf (FND-05) e o
  bloco `contract` estava fora do generator; a primeira iteração usava o
  placeholder `orders.event.v1.OrderPlaced`, um evento de outro contexto.
- **Impacto**: o contexto gerado publica o próprio contrato; o rito Buf
  (`buf-lint`, `buf-generate-check`, baseline BUF-08) continua o guardião do
  `gen/go`.
- **Inspiração**: `contracts/README.md` (árvore, proveniência do envelope,
  máquina de estados BUF-08); `dmpf-provider-postgres/example/orders/mapper.go`;
  `dmpf-app/example/reservations/consumer.go`; `dmpf-reference` (relay e
  consumer).
- **Links relevantes**:
  - `SPEC-VZ16X0MS` — templates dos cinco blocos (esta spec acrescenta o que
    depende de contrato)
  - `SPEC-8FSD8505` — `integration` na DSL; relatório e instrução final
  - `SPEC-H1A190Y8` — guarda-chuva: prova, CI, docs
  - FND-05 (CloudEvents + Protobuf), FND-04 §5/§6 (outbox, inbox), ADR-033
    (adaptações monorepo dos contratos), ADR-035, ADR-036, ADR-038 (relay)
  - `docs/adr/041-*.md` — dívida "contrato próprio pelo rito Buf" que esta
    spec quita

<constraints>
- [P0] NUNCA escrever em `gen/go`: o generator emite só a fonte `.proto`; o código gerado do contrato vem do rito Buf.
- [P0] NUNCA regravar o baseline Buf nem o baseline de unidades pelo generator: ambos são atos governados em commit próprio.
- [P0] O payload da outbox são os bytes do `Any` do integration event, nunca o CloudEvent inteiro (ADR-035); o consumer usa `envelope.Unpack` e devolve `R1D4` em `ErrSchemaMismatch`/`ErrMalformed` sem abrir UoW.
- [P1] A prova `integration` reprova, nunca pula, sem `DMPF_PG_DSN` e `DMPF_KAFKA_BROKERS`.
- [P1] A prova preserva a integridade do `node_modules` da raiz (`pnpm_config_verify_deps_before_run=false`; nunca define `CI`; guard sobre `node_modules/*` e `node_modules/@*/*`).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Fonte `.proto` por integration event**:
  `contracts/proto/company/<goIdent>/event/v1/<event_snake>.proto` com `package
  company.<goIdent>.event.v1` — `company` é o `<org>` fixo do workspace
  (PTB-01, REP-01; `contracts/proto/company/orders/event/v1/order_placed.proto:3`)
  e `<goIdent>` é o identificador Go do `name` do módulo, sem hífen, que não é
  segmento válido de package Protobuf; o diretório espelha o package —
  `option go_package` conforme `contracts/README.md`, um
  `message <Event>` com os campos do evento mapeados (`string`, `int64`,
  `bool`, `string` decimal, `google.protobuf.Timestamp`, `string` id) e os
  comentários mínimos que o `buf lint STANDARD` exige. Se `contracts/proto/company/<goIdent>/` já existe, o generator só escreve os
  `.proto` de eventos ainda ausentes e nunca regrava um existente (contrato
  publicado é imutável; a evolução é rito Buf). **Drift**: para cada evento
  publicado que já tem `.proto`, o generator lê a `message` existente (campos
  e tipos) e compara com a definição; qualquer divergência recusa a execução
  nomeando o evento e indicando o rito de evolução (`v2` ou campo novo pelo
  rito Buf) — o mapper mecânico é derivado do contrato efetivo, nunca de uma
  DSL que divirja dele.
  - `contracts/buf.yaml` (v2) tem um único módulo `proto` que cobre toda a
    árvore `contracts/proto/company/**`; não existe `buf.yaml` por contexto e
    o generator não cria nenhum — o `.proto` novo entra no módulo existente.
- [ ] **[P0] Instrução do rito Buf** na saída final quando há `.proto` novo,
  na ordem que o rito exige (`contracts/README.md`): primeiro materializar o
  código — `(cd contracts && bash ../tools/buf.sh generate)`, o único passo que
  escreve em `gen/go` (`buf-generate-check` apenas gera em diretórios
  temporários e compara, `tools/buf-gate.sh:69-78`) — depois os quatro gates
  `pnpm nx run dmpf-contracts-go:buf-lint`, `buf-pins`, `buf-generate-check` e
  `NX_BASE=<base> … buf-breaking`, e a nota BUF-08 sobre o baseline dos
  contratos — tudo ANTES de compilar os módulos gerados que importam `gen/go`.
- [ ] **[P0] Unidade `contract` do contexto**: o `gen/go` novo
  (`dmpf-contracts/gen/go/company/<goIdent>/event/v1`) precisa de dono no
  universo — hoje `libs/backend/go/dmpf-contracts/dmpf-units.json` enumera
  exatamente três packages, e um package sem unidade recebe `DMPF-U001`, o que
  faz o próprio `--write-baseline` abortar (`cmd/dmpf-conformance/main.go`). O
  generator faz merge por campo nesse manifesto acrescentando a unidade
  `{id: "<boundedContext>/contract", block: "contract", bounded_context:
  "<boundedContext>", include: [<import path do gen/go do contexto>],
  external: protobuf}` (pública por construção, como todo `contract`); ela
  entra no baseline no ato de classificação. Vetor negativo na prova: sem a
  unidade, `U001` e `--write-baseline` abortando.
- [ ] **[P0] Mapper e outbox**: no provider, `<aggregate>_mapper.go` mapeia
  cada evento de domínio em `publishes` para a message gerada
  (`eventv1.<Event>`), com `DataSchema` e `Type` do CloudEvent conforme
  FND-05, convertendo `instant` (inteiro de nanossegundos no domínio) em
  `google.protobuf.Timestamp` — o `external` do provider declara os
  `entrypoints` que o mapper realmente importa (`proto`, `anypb`,
  `timestamppb`); a `Service` (sub-spec B) enfileira só os eventos de
  `publishes`. Testes de mapeamento por evento, incluindo o round-trip do
  `instant`.
- [ ] **[P0] Consumer por `consumes`**: `Handler` com `envelope.Unpack` para a
  message `messageFQN` do produtor (import `goImport` do `gen/go`, ambos
  vindos da definição e validados contra o `.proto` do produtor em
  `contracts/proto/company/**` antes de escrever), `R1D4` com
  `Failure(Validation)` em `ErrSchemaMismatch`/`ErrMalformed` sem UoW,
  chamada a `service.Consume<Event>` com `MessageID`, `MessageType`,
  `PayloadHash`, `ReceivedAt` e os campos traduzidos pelo `fieldMap` para o
  comando local — o precedente nominal é `dmpf-app/example/reservations/consumer.go`;
  `NewConsumer`; teste de envelope de outro contrato → `R1D4` e contenção.
- [ ] **[P0] Fases `integration` e `self-test` da prova**:
  `tools/dmpf-generator-check.sh --phase integration` roda `test-race` dos
  módulos gerados a partir de `bookings.jsonc` com Postgres e Redpanda
  (exige `DMPF_PG_DSN` e `DMPF_KAFKA_BROKERS`), conferência do manifesto SHA-256,
  `git diff --exit-code`, `status --porcelain` vazio; `--phase self-test`
  reprova os quatro vetores (edição pós-commit; classificação misturada →
  `T002`; instrução do baseline ausente; JSON fora do padrão Biome) mais o
  quinto: `.proto` gerado sem o rito Buf → o build dos módulos falha e a
  prova nomeia o passo; e o sexto: evento publicado alterado na definição com
  `.proto` já existente → o generator recusa por drift. A fase `structural` passa a
  materializar `gen/go` no worktree (`buf.sh generate`), rodar os quatro gates
  Buf, hashear e commitar o contrato gerado (fonte `.proto` + `gen/go` +
  manifesto do `dmpf-contracts`) num commit próprio anterior ao de
  classificação, e só então seguir com a cadeia Go.
- [ ] **[P0] CI**: `ci.yml` ganha "DMPF generator check (structural)" e
  "(self-test)" no bloco Gates DMPF (após `dmpf-cell-check.sh`) e
  "(integration)" após Postgres e Redpanda, antes do estágio 3 — sem `if` de
  `go_affected` (mudança só no plugin TS também exige a prova).
- [ ] **[P1] Guia de composição** `docs/guides/dmpf-composicao.md`: escrever a
  definição; gerar; rodar o rito Buf; classificar (baseline em commit próprio);
  preencher os stubs (`_rules.go`, `config.go`); `pnpm install` (o módulo
  gerado vira importer do pnpm); cabear o composition root copiando
  `dmpf-reference`; subir Postgres e Redpanda; rodar cadeia e verificador;
  regenerar com `--update`. Seções "O que o generator nunca toca" (baselines,
  `gen/go`, stubs), "Divergir do golden path" (sub-spec 3 do KRN-12) e
  "Contrato próprio" (esta spec). Indexado em `docs/dmpf/README.md` e
  `docs/dmpf/navegacao.md`.
- [ ] **[P1] Documentação**: `AGENTS.md` (árvore, bullet do plugin, comandos
  do generator com `--definition` e das três fases), `docs/nx-reference/
  tasks.md` (seção do generator), `tools/dmpf-plugin/README.md` (revisar para
  a DSL), addendum no ADR-041 fechando a dívida do contrato próprio.
- [ ] **[P1] Verificação final**: cadeia completa do workspace verde; as três
  fases da prova verdes; gate Nível C no plano.

### Não-funcionais

- [ ] `buf lint STANDARD` e `buf format` aprovam o `.proto` gerado sem edição.
- [ ] Fase `structural` em menos de 3 min no runner, incluindo o rito Buf;
  `integration` em menos de 6 min.
- [ ] Sem dependência TS nova; o código Go gerado depende de `protobuf` só
  pelos packages `provider` e `app` (como o kernel).

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `contract` | [x] | Fonte `.proto` gerada por contexto; `gen/go` pelo rito |
| `provider`, `app` gerados | [x] | Mapper, outbox, consumer com `envelope.Unpack` |
| Prova / CI | [x] | Fases `integration` e `self-test`; três passos no `ci.yml` |
| Documentação | [x] | Guia de composição, `AGENTS.md`, `tasks.md`, README, ADR-041 |

## Localização de código

```text
tools/dmpf-plugin/src/generators/bounded-context/
    files/contract/event.proto__tmpl__                                   — NOVO (módulo Buf único; sem buf.yaml por contexto)
  generator.ts                                                         — MODIFICAR: leitura do .proto existente (drift), merge da unidade contract em dmpf-contracts/dmpf-units.json
libs/backend/go/dmpf-contracts/dmpf-units.json                         — MODIFICAR (pelo generator, merge por campo): unidade <boundedContext>/contract
  files/provider/aggregate_mapper.go__tmpl__, mapper_test.go__tmpl__   — NOVO
  files/app/consumer.go__tmpl__, consumer_test.go__tmpl__              — MODIFICAR: envelope.Unpack, R1D4, comando local
  generator.ts                                                         — MODIFICAR: .proto só se ausente; instrução do rito Buf
tools/dmpf-generator-check.sh                                          — MODIFICAR: rito Buf na structural; fases integration e self-test; guard completo de node_modules
.github/workflows/ci.yml                                               — MODIFICAR: três passos
docs/guides/dmpf-composicao.md                                         — NOVO
docs/dmpf/README.md, docs/dmpf/navegacao.md, AGENTS.md, docs/nx-reference/tasks.md, tools/dmpf-plugin/README.md — MODIFICAR
docs/adr/041-*.md                                                      — MODIFICAR: addendum (dívida do contrato quitada)
```

## Design

### Arquitetura

```text
 DomainModel.integration.publishes ──► contracts/proto/company/<goIdent>/event/v1/<event>.proto (fonte; só se ausente)
                                                                                │ rito Buf (humano/CI): buf.sh generate → gen/go; depois buf-lint, buf-pins, buf-generate-check, buf-breaking
                                        │ drift DSL × .proto existente → recusa; unidade <ctx>/contract no manifesto do dmpf-contracts
                                        ▼
 provider: <aggregate>_mapper.go ── evento de domínio → eventv1.<Event> (Any) ──► outbox (ADR-035)
 app: Handler ── envelope.Unpack(env, &msg) ── ErrSchemaMismatch/ErrMalformed → R1D4 (sem UoW)
                                             └─ ok → service.Consume<Event>(...) → 7 disposições → Ack/Release/contenção
 prova: structural (gen/go materializado + 4 gates Buf + commit do contrato + cadeia + verificador) · integration (test-race com infra) · self-test (6 vetores)
```

### Fluxo principal

1. A squad gera `bookings` com `publishes: [BookingReserved]`.
2. O generator escreve `contracts/proto/company/bookings/event/v1/booking_reserved.proto`,
   a unidade `resource-scheduling/contract` no manifesto do `dmpf-contracts`,
   e termina com a instrução do rito Buf.
3. A squad roda `buf.sh generate` (materializa `gen/go`), depois `buf-lint`,
   `buf-pins`, `buf-generate-check` e `buf-breaking`; commit do contrato
   conforme BUF-08.
4. A squad roda `--write-baseline` em commit próprio; depois commita o código.
5. `Reserve` aceito → `BookingReserved` → mapper → outbox → relay publica o
   CloudEvent com o `Any` do payload.
6. Um contexto consumidor gerado com `consumes: [{context: bookings, event:
   BookingReserved, command: ...}]` recebe, decodifica, decide e executa.

## Decisões técnicas

- **Gerar só a fonte `.proto`, nunca `gen/go`** porque o rito Buf (lint,
  pins, breaking, generate-check, baseline BUF-08) é o guardião do contrato
  publicado (ADR-033); gerar código do contrato fora dele criaria duas fontes
  de verdade. A materialização é `buf.sh generate`, passo humano/CI; a
  instrução final o nomeia primeiro porque `buf-generate-check` só compara.
  Alternativa descartada: rodar `buf generate` dentro do generator, porque
  acoplaria o plugin TS ao toolchain Buf via `go run` e esconderia do CI o
  passo governado.
- **Recusa de drift entre DSL e `.proto` publicado** porque o mapper é
  mecânico e seria recalculado a partir da definição divergente — deixaria
  de compilar ou publicaria payload diferente do contrato. Alternativa
  descartada: regravar o `.proto`, porque contrato publicado é imutável.
- **Unidade `contract` do contexto no manifesto do `dmpf-contracts`** porque
  o `gen/go` novo é package de produção e precisa de dono no universo; sem
  isso `--write-baseline` nem roda.
- **`.proto` existente nunca é regravado** porque contrato publicado é
  imutável; evolução é `v2` ou campo novo pelo rito. Alternativa descartada:
  regravar com `DO NOT EDIT`, porque o `buf breaking` acusaria e o generator
  não sabe o que já foi publicado.
- **Import do `gen/go` do produtor no consumer** porque é como
  `example/reservations` consome `orders` hoje; a aresta `app → contract` é
  permitida (célula 18) e `contract` é público por construção.
- **Seis vetores no `self-test`** porque "gerei o proto mas não rodei o rito"
  e "mudei o evento com contrato já publicado" são as falhas mais prováveis
  na primeira semana de uso e precisam de mensagem própria.

## Regras relacionadas

- FND-04 §5/§6, FND-05, FND-06 §11; ADR-033, ADR-035, ADR-036, ADR-038.
- `contracts/README.md` — árvore e BUF-08.
- `SPEC-8FSD8505`, `SPEC-VZ16X0MS`, `SPEC-H1A190Y8`.

## Verificação e testes

### Critérios de aceite

- [ ] `.proto` gerado para `BookingReserved` passa `buf-lint`; após
  `buf-generate-check`, `gen/go/<ctx>/event/v1` existe e os módulos gerados
  compilam.
- [ ] Teste de mapper: `BookingReserved` → `eventv1.BookingReserved` com todos
  os campos; teste de consumer: envelope de outro contrato → `R1D4`.
- [ ] `--phase structural`, `--phase integration` e `--phase self-test`
  passam localmente e no CI; `self-test` mostra seis `ok (reprovou como
  esperado)`.
- [ ] `yq '.jobs.main.steps[].name' .github/workflows/ci.yml` mostra os três
  passos na ordem.
- [ ] Guia criado e indexado; `AGENTS.md`, `tasks.md`, README revisados;
  addendum no ADR-041; `pt-reviewer`/`en-reviewer` ✓.
- [ ] Cadeia do workspace verde.

### Cenários de teste

```text
DADO bookings.jsonc com publishes [BookingReserved] e contracts/proto/company/bookings ausente
QUANDO o generator roda
ENTÃO contracts/proto/company/bookings/event/v1/booking_reserved.proto existe e a saída instrui o rito Buf antes da cadeia

DADO o mesmo contexto já com booking_reserved.proto commitado
QUANDO o generator roda com --update e um evento novo BookingCancelled em publishes
ENTÃO booking_cancelled.proto é criado e booking_reserved.proto permanece byte a byte

DADO o mesmo contexto e uma definição em que BookingReserved ganhou um campo novo
QUANDO o generator roda com --update
ENTÃO recusa por drift nomeando BookingReserved, indica o rito de evolução e nada é escrito

DADO o gen/go novo do contexto sem a unidade contract no manifesto do dmpf-contracts (vetor da prova)
QUANDO dmpf-conformance --write-baseline roda
ENTÃO aborta com U001 nomeando o package, e a prova reprova nomeando o passo

DADO um consumer gerado para BookingReserved e um envelope com DataSchema de orders.event.v1.OrderPlaced
QUANDO o Handler recebe a entrega
ENTÃO devolve R1D4 com Failure(Validation), não abre UoW e a mensagem é contida

DADO o worktree da prova com o .proto gerado e o rito Buf não executado (vetor do self-test)
QUANDO a fase structural roda
ENTÃO a prova reprova nomeando "rito Buf" e o módulo que não compila
```

<critical_constraints>
- [P0] NUNCA escrever em `gen/go`: só a fonte `.proto`; o código vem do rito Buf.
- [P0] NUNCA regravar o baseline Buf nem o baseline de unidades pelo generator.
- [P0] Payload da outbox = bytes do `Any` do integration event (ADR-035); consumer com `envelope.Unpack` e `R1D4` sem UoW em `ErrSchemaMismatch`/`ErrMalformed`.
- [P1] A prova `integration` reprova, nunca pula, sem as variáveis de infra.
- [P1] A prova preserva a integridade do `node_modules` da raiz.
</critical_constraints>

## Escopo fora

- **Contratos síncronos (gRPC/HTTP entre contextos)**: FND-06 §9-10; o
  generator emite só rotas HTTP da borda do próprio contexto e integration
  events assíncronos.
- **Evolução de contrato (`v2`, campos novos em proto publicado)**: rito Buf e
  BUF-08; o generator nunca regrava `.proto` existente.
- **Composition root e canais Kafka por ambiente**: golden path
  (`dmpf-reference`, `infra/`); a sub-spec 3 do KRN-12 cobre divergências.
