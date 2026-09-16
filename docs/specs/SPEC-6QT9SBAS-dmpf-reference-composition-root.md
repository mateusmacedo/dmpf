---
id: SPEC-6QT9SBAS
slug: reference-composition-root
title: DMPF KRN-12.1 — Composition root de referência reference
stage: done
priority: P2
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-XF9TF9A0, SPEC-ZHE7DN1H, SPEC-WYX5GW87, SPEC-3R80KNMS, SPEC-ANZX2WPG, SPEC-CGPX20NP, SPEC-NYD18TGD, SPEC-EAGAXQN1, SPEC-SJ66880S]
ticket_url: null
subtask_urls: []
created: 2026-09-08
---

# SPEC-6QT9SBAS: DMPF KRN-12.1 — Composition root de referência reference

## Resumo

Primeira sub-spec de `SPEC-8HWBWJCB` (`KRN-12`). Entrega o serviço de
referência `apps/backend/reference` — o primeiro projeto Nx sob `apps/` e o
único lugar onde instanciar provider concreto é permissivo (ADR-015). Um binário,
três papéis por flag: `api` serve a borda HTTP de `orders`, `relay` drena a
outbox para Kafka, `consumer` lê de Kafka e alimenta `reservations` pela inbox.
Reutiliza os agregados de exemplo do kernel e o padrão que
`app/example/reservations/` já provou em miniatura.

Como usuário de uma squad, quero abrir `reference` e ver, arquivo por
arquivo, como os blocos do kernel se cabeiam num processo real — e copiar.

## Contexto

- **Problema**: `apps/backend` é um `.gitkeep`. Nenhum serviço mostra a
  composição completa: `app/example/reservations/` cabeia consumer e relay,
  mas não tem borda síncrona, não sobe como processo e não roda observabilidade.
- **Impacto**: o guia de composição da sub-spec 2 aponta para arquivos reais;
  o BOM da sub-spec 4 lista `reference` como a coordenada exercitada do
  composition root.
- **Inspiração**: `app/example/reservations/{consumer.go,relay.go}`
  (`NewService`, `NewConsumer`, `NewRelay` com `RandomClaimIDs` e
  `SystemClock`); `testkit/distkit/roles.go` (`adapterSink`, a ponte
  `kafka.Sink` → `app.Consumer`).
- **Links relevantes**:
  - `SPEC-8HWBWJCB` — guarda-chuva; decisões transversais (identidade da
    release, um transporte por processo)
  - `SPEC-ZHE7DN1H` — `ordersapp.Service` e os nove passos de FND-04 §3.2
  - `SPEC-ANZX2WPG` — `reservationsapp.Service.Consume` e as sete disposições
  - `SPEC-CGPX20NP` — `relay.New`, `relay.Config`
  - `SPEC-EAGAXQN1` — `http.Route` (outbound), `http.Admission`,
    `kafka.Publisher`, `kafka.Consumer`, `Sink`
  - `SPEC-NYD18TGD` — `otelboot.Start`
  - ADR-015, ADR-034 (`UOW-11`: query lê fora da UoW), ADR-035, ADR-039

### Divergências entre o ticket e o repositório

| O ticket diz | O repositório tem | O que esta spec adota |
| --- | --- | --- |
| "cabeia os providers de `KRN-10`" | sem contrato OpenAPI nem serviço gRPC publicado | HTTP com contrato OpenAPI mínimo publicado aqui; Kafka; Postgres; OTel. gRPC e SQS fora (guarda-chuva) |
| "escrita" via `PlaceOrder` | `Service.PlaceOrder` só **carrega** (`place_order.go:32-35`) — ordem ausente é erro técnico; `Order.Place` rejeita ordem sem itens (`domain/orders/place.go:11-13`). `AddItem` faz `loadOrCreate` (`add_item.go:31`) | Rotas: `POST /orders/{id}/items` (cria ou carrega e adiciona), `POST /orders/{id}/place` (coloca) e `GET /orders/{id}` |
| idempotência de POST | `http.Route` é **outbound** (`route.go:31-33`): `IdempotencyKey` só torna o POST elegível a retry e o `Client` gera o header quando falta (`client.go:149-151`); `Validate` não recusa request de entrada | O handler de entrada exige o header `Idempotency-Key` (400 sem ele) e o propaga ao contexto; `Route` é usado para declarar o contrato (RST-04) e o orçamento. Replay persistido de resposta é escopo fora |
| `GET /orders/{id}` | `FindOrder` lê por `Service.Reader` fora da UoW (`find_order.go:11-16`, `UOW-11`); `orderspg` só tem `NewRepository(tx *postgres.Tx)` (`repository.go:36`) | Esta spec adiciona `orderspg.NewReader(pool *pgxpool.Pool) ports.Reader[...]` ao provider Postgres, com teste; a leitura roda em `pool.Query`, sem transação |
| targets para subir os papéis | `@nx-go/nx-go` deriva o nome do projeto do último segmento do diretório (`create-nodes-v2.js:34-36`), então `cmd/reference/main.go` ativa a inferência de `build` e `serve` mesmo com o projeto chamado `reference-go` (confirmado na implementação: o `serve` inferido aparece assim que `cmd/` existe) | Targets declarados `serve-api`, `serve-relay`, `serve-consumer` (`nx:run-commands`, `go run ./cmd/reference --role <papel>`); o `build` explícito sobrescreve o inferido (merge do Nx) e o `serve` inferido existe, fica sem uso e não é redeclarado |
| `type:app` | `nx-release.yml` trata todo `tag:type:app` como candidato Docker | Filtro passa a `tag:type:app,!tag:stack:go`, precedente do `build_projects_filter` de `nx-publish-libs.yml` |

### Fontes normativas

| Fonte | O que fixa |
| --- | --- |
| ADR-015 | Instanciação de provider concreto só no composition root |
| FND-04 §3.2 (`SPEC-ZHE7DN1H`) | Nove passos da sequência canônica dentro de `UoW.Within`; outbox na mesma transação |
| FND-04 §5.4 / `BLK-02` | Relay em processo próprio |
| FND-04 §6 / `INB-*` | Inbox deduplica; efeito de broker depois do commit (`INB-08`); nunca exactly-once |
| `UOW-11` (ADR-034) | Query lê por `Reader`, fora da UoW |
| FND-06 §9 `RST-02`/`RST-04` | POST idempotente só com chave declarada; rota referencia contrato publicado |
| FND-06 §11 (ADR-039) | `Sink` é a ponte; teto de tentativas do adapter = teto do canal |
| `RES-17` (`SPEC-EAGAXQN1`) | `Admission` recusa antes de ler o corpo |
| `AGENTS.md` §Convenções | Tags 3D + `layer:*`; targets Go via `nx:run-commands`; sem redeclarar target inferido |

<constraints>
- [P0] NUNCA construir `pgxpool`, `franz-go`, `net/http` ou `otel` fora deste módulo: `reference` é a única unidade `app` de produção que instancia provider concreto (ADR-015).
- [P0] NUNCA abrir transação para `GET /orders/{id}`: a leitura passa por `orderspg.NewReader(pool)` (`UOW-11`).
- [P0] NUNCA aplicar `Ack`, `Release` ou contenção antes do retorno de `UoW.Within` no consumer (`INB-08`).
- [P0] NUNCA declarar exactly-once: o e2e republica a mesma entrega e prova idempotência pela inbox.
- [P0] NUNCA subir publisher no papel `api` nem servidor HTTP no papel `relay`: um papel por processo (`BLK-02`).
- [P1] Toda rota HTTP declara `ContractRef` para `contracts/openapi/orders/v1/openapi.yaml` (RST-04); `Route.Validate` roda na construção e falha o processo.
- [P1] `MaxAttempts` do `app.Consumer` é lido do `channel.Catalog`, nunca duplicado como literal (ADR-039).
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Módulo e projeto**: `apps/backend/reference` com `go.mod`
  (`module github.com/mateusmacedo/dmpf/apps/backend/reference`,
  `go 1.26.6`, sem `require` de irmão — ADR-034), entrada `use
  ./apps/backend/reference` no `go.work`, `project.json` `reference-go`
  com tags `["type:app", "scope:backend", "stack:go", "layer:apps"]` e targets
  `fmt-check`, `vet`, `build`, `test-race` (`cache: false`, `dependsOn`
  `postgres:test-race` e `app:test-race` — partilham
  o Postgres do job), `govulncheck`, `serve-api`, `serve-relay`,
  `serve-consumer`; `package.json` `@mateusmacedo/reference-go` `0.0.0`
  `private: true`; `dmpf-units.json` com a unidade `kernel/reference-app`,
  bloco `app`, `bounded_context` `kernel`, `external` com `pgx/v5`
  (`io.storage`), `franz-go` (`io.messaging`), `go.opentelemetry.io/otel`
  (`observability`) e os exporters OTLP (`io.network`), copiados dos providers
  que já os declaram.
  - Baseline: a unidade entra em `tools/dmpf-baseline/units-baseline.json` em
    commit próprio (`chore(workspace): [ARQ-…] Registrar a unidade do
    reference no baseline`).
  - Edge case: `pnpm nx show projects --projects=tag:layer:apps` lista
    `app` e `reference-go`; a guarda de cobertura de tags do
    `ci.yml` passa.
- [ ] **[P0] Binário e papéis**: `cmd/reference/main.go` lê `--role
  api|relay|consumer`; ausente ou desconhecido → exit 2 com mensagem listando os
  três. `config.go` lê `DMPF_PG_DSN`, `DMPF_KAFKA_BROKERS`, `DMPF_HTTP_ADDR`
  (default `:8080`), `DMPF_SERVICE` (default `reference`) e, para o
  canal, `DMPF_KAFKA_TOPIC`, `DMPF_KAFKA_GROUP` e `DMPF_KAFKA_DLQ` — o **nome**
  do canal não é configurável: é `ordersapp.Destination` (`orders.events`),
  porque o publisher resolve pelo destino que o caso de uso autorou (achado B
  da revisão do plano; `DMPF_CHANNEL` foi descartada); variável obrigatória
  ausente para o papel → exit 2 nomeando-a. Todo papel chama `otelboot.Start` e faz graceful
  shutdown por `SIGTERM`/`SIGINT` com prazo de 10 s.
- [ ] **[P0] Papel `api`**: `wiring.NewOrdersService(pool, clock, ids)` monta
  `ordersapp.Service{UoW: postgres.NewUnitOfWork(pool, bindOrders),
  Reader: orderspg.NewReader(pool), Clock, IDs, Authorize: AllowAll,
  ItemLimit}`. Rotas em `api/routes.go`, cada uma um `http.Route`
  validado na construção, servidas por `net/http.ServeMux` com o middleware
  `http.Admission` (`RES-17`) antes do handler:
  - `POST /orders/{id}/items` → `Service.AddItem` (`loadOrCreate`; corpo
    `{sku, quantity}`); `201` quando aceito, `422` com o código da rejeição,
    `409` em conflito de versão.
  - `POST /orders/{id}/place` → `Service.PlaceOrder`; `200` aceito, `422`
    rejeitado (`CodeOrderNotOpen`, `CodeEmptyOrder`), `404` ordem ausente.
  - `GET /orders/{id}` → `Service.FindOrder`; `200` com o snapshot, `404`
    ausente.
  - Os dois POST exigem `Idempotency-Key` (`400` sem; o valor entra no
    contexto e no log estruturado); `RST-02` — sem chave não há retry
    admissível, e o handler não finge tê-la.
  - Edge case: status fora do declarado no contrato reprova o teste de
    rotas.
- [ ] **[P0] Contrato OpenAPI mínimo**: `contracts/openapi/orders/v1/openapi.yaml`
  (OpenAPI 3.1): as três operações, schemas de request/response derivados de
  `orders.Snapshot`, `ItemAccepted`, `PlacedResponse`, header
  `Idempotency-Key` obrigatório nos POST, e os códigos de resposta acima. É a
  fonte que `RST-04` exige; não gera código.
- [ ] **[P0] `orderspg.NewReader`**: em
  `libs/backend/go/postgres/example/orders/reader.go`,
  `NewReader(pool *pgxpool.Pool) ports.Reader[orders.OrderID,
  orders.Snapshot]`, `Load` por `pool.QueryRow` reutilizando o `SELECT` e o
  mapeador de `repository.go`; teste `reader_test.go` (build tag `integration`)
  prova que `Load` não abre transação (sem `BEGIN` no log do pool de teste) e
  devolve `ErrNotFound` para id ausente.
- [ ] **[P0] Papel `relay`**: `wiring.NewRelay(pool, publisher, cfg)` chama
  `relay.New(postgres.NewOutboxStore(pool, clock), publisher, claimIDs,
  clock, relay.Config{Source, Interval, BatchSize, Lease, Concurrency})`;
  `publisher` é `kafka.NewPublisher(kafka.Config{Brokers, Catalog,
  Sheet, Service, Clock, Tracer}, observer)`; `claimIDs` e `clock` são os
  mesmos `RandomClaimIDs`/`SystemClock` de `app/example/reservations`,
  promovidos a `reference/ports.go`.
- [ ] **[P0] Papel `consumer`**: `wiring.NewReservationsConsumer(pool, clock,
  ids, wait, maxAttempts)` monta `reservationsconsumer.NewConsumer(...)`
  (reutilizado de `app/example/reservations`) e `sink.go` realiza
  `kafka.Sink` chamando `consumer.Consume(ctx, app.Delivery{Raw,
  Attempt}, ack)`; `kafka.Consumer{Config, Channel, Sink}` roda
  `Run(ctx)`. `maxAttempts` = teto do canal no `channel.Catalog`.
- [ ] **[P0] E2e** (`e2e_test.go`, build tag `integration`, exige
  `DMPF_PG_DSN` e `DMPF_KAFKA_BROKERS`): sobe os três papéis em goroutines do
  mesmo teste (processos separados são provados pelo `distkit` de `KRN-11`;
  aqui o que se prova é a cablagem), executa `POST items` → `POST place`,
  espera o relay marcar publicado, espera o consumer criar a reserva, republica
  a mesma mensagem e prova `DuplicateIgnored` com uma reserva e um registro de
  inbox. Sob `CI`, variável ausente falha; sem `CI`, `t.Skip` nomeando-a.
- [ ] **[P1] `nx-release.yml`**: filtro de candidatos Docker →
  `tag:type:app,!tag:stack:go`, comentário de uma linha citando o precedente.
- [ ] **[P1] ADR-041** criado com as decisões transversais da guarda-chuva e
  as desta spec; **`AGENTS.md`**: seção Apps lista `reference-go`; seção
  Libs corrige `app` para três unidades (`app-consumer`, `app-relay`,
  `example-reservations-app`); tabela `layer:*` ganha `reference-go`;
  Comandos ganham `serve-*`. **`libs/backend/go/app/README.md`** e
  **`doc.go`** passam a listar o relay como entregue. `README.md` da app
  documenta papéis, variáveis e como subir local com Postgres e Redpanda.

### Não-funcionais

- [ ] Conformidade: `conformance` aprova o módulo; `dmpf-gate-check.sh` e
  `dmpf-cell-check.sh` continuam verdes.
- [ ] Cadeia verde: `fmt-check`, `vet`, `lint`, `build`, `test-race`,
  `govulncheck`; `biome ci`; `adr-verify`.
- [ ] Sem dependência Go nova: só módulos do kernel e as dependências que os
  providers já declaram.
- [ ] Segurança: DSN e brokers só por variável de ambiente; nenhum segredo em
  arquivo; `tb/pg.OpenPool` continua loopback-only nos testes.
- [ ] Prazo: `serve-api` responde `GET /orders/{id}` em menos de 50 ms sobre
  Postgres local (medido no e2e, sem asserção dura).

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `domain` | [ ] | — |
| `application` | [ ] | — |
| `port` | [ ] | — |
| `contract` | [x] | `contracts/openapi/orders/v1/openapi.yaml` (fonte, sem código gerado) |
| `provider` | [x] | `postgres/example/orders/reader.go` (`NewReader`) |
| `app` | [x] | `apps/backend/reference` (novo) |
| Workspace | [x] | `go.work`, `units-baseline.json` (commit próprio), `nx-release.yml`, `AGENTS.md`, `app/README.md` e `doc.go`, ADR-041 |

## Localização de código

```text
apps/backend/reference/                          — NOVO; projeto reference-go; unidade kernel/reference-app (app)
  go.mod, project.json, package.json, dmpf-units.json, README.md, doc.go
  config.go                                           — Config{DSN, Brokers, HTTPAddr, Service, Channel}; FromEnv(role); Validate
  ports.go                                            — RandomClaimIDs (io.random), SystemClock (io.clock)
  wiring.go                                           — NewPool, NewOrdersService, NewRelay, NewPublisher, NewReservationsConsumer, NewTelemetry
  sink.go                                             — adapterSink: kafka.Sink → app.Consumer.Consume
  api/routes.go                                       — Routes(): 3 http.Route com ContractRef; Handler(service) http.Handler; Admission
  api/idempotency.go                                  — requireIdempotencyKey middleware (400 sem header)
  api/routes_test.go                                  — contrato × status; header obrigatório; Route.Validate
  cmd/reference/main.go                          — --role; otelboot.Start; graceful shutdown
  e2e_test.go                                         — integration: items → place → outbox → relay → Kafka → consumer → inbox; reentrega
  testing_test.go                                     — harness: pool, brokers, TRUNCATE, canal
contracts/openapi/orders/v1/openapi.yaml               — NOVO
libs/backend/go/postgres/example/orders/reader.go, reader_test.go — NOVO: NewReader(pool)
go.work                                                — MODIFICAR: use ./apps/backend/reference
tools/dmpf-baseline/units-baseline.json                — MODIFICAR em commit próprio
.github/workflows/nx-release.yml                       — MODIFICAR: tag:type:app,!tag:stack:go
docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md — NOVO
AGENTS.md, README.md                                   — MODIFICAR: Apps, Libs (app: 3 unidades), layer:*, Comandos
libs/backend/go/app/README.md, doc.go             — MODIFICAR: relay entregue; 3 unidades
```

**Arquivos a modificar, e o que muda**

`go.work` ganha o primeiro módulo fora de `libs/` — o `namedInput` `go` de
`nx.json` já cobre o arquivo inteiro. `nx-release.yml` muda um filtro. O
provider Postgres ganha um construtor de leitura fora de transação, o único
gesto que `UOW-11` exige e que faltava para uma query real. `AGENTS.md` fecha a
lacuna preexistente do `app` e registra a app.

## Design

### Arquitetura

```text
 --role api                          --role relay                        --role consumer
 ┌──────────────────────────────┐    ┌──────────────────────────────┐    ┌──────────────────────────────────┐
 │ ServeMux                     │    │ relay.New(                   │    │ kafka.Consumer               │
 │  Admission (RES-17)          │    │   OutboxStore(pool, clock),  │    │   Sink = adapterSink             │
 │  requireIdempotencyKey       │    │   Publisher(kafka),          │    │     → app.Consumer.Consume   │
 │  POST /orders/{id}/items ───►│    │   RandomClaimIDs, clock, cfg)│    │       → reservationsapp.Service  │
 │  POST /orders/{id}/place ───►│    │ claim → publish → mark       │    │         → UoW(pg): inbox+outbox  │
 │  GET  /orders/{id} ─────────►│    └──────────────┬───────────────┘    │   Containment = Quarantine(pool) │
 │ ordersapp.Service            │                   │                    └────────────────▲─────────────────┘
 │  UoW(pg) → outbox (AddItem,  │                   ▼                                     │
 │            PlaceOrder)       │             Kafka (Redpanda) ───────────────────────────┘
 │  Reader(pool) (FindOrder)    │
 └──────────────────────────────┘
 otelboot.Start em todo papel; graceful shutdown por sinal
```

### Fluxo 1 — escrita, drenagem e consumo

1. `POST /orders/{id}/items` com `Idempotency-Key`: `Admission` decide antes
   de ler o corpo; `requireIdempotencyKey` recusa sem header; o handler monta
   `ordersapp.AddItem{Order, SKU, Quantity}` e chama `Service.AddItem`, que faz
   `loadOrCreate` e percorre os nove passos dentro de `UoW.Within`.
2. `POST /orders/{id}/place`: `Service.PlaceOrder` carrega, decide
   (`Order.Place`), grava e **enfileira `OrderPlaced` na mesma transação**.
3. O papel `relay` faz `claim` por lease, publica pelo `kafka.Publisher`
   no canal catalogado e marca publicado.
4. O papel `consumer` recebe os bytes; `adapterSink` entrega
   `Delivery{Raw, Attempt}` ao `app.Consumer`, que decodifica, calcula
   `payload_hash` e invoca `reservationsapp.Service.Consume` na UoW: inbox
   deduplica, reserva é criada, efeito de broker depois do commit.
5. A mesma mensagem republicada termina em `DuplicateIgnored`.

### Fluxo 2 — leitura

1. `GET /orders/{id}` → `Service.FindOrder` → `Reader.Load` por
   `pool.QueryRow`; nenhuma transação, nenhum registro de outbox (`UOW-11`).

### Onde cada regra é provada

| Regra | Instrumento | Onde |
| --- | --- | --- |
| ADR-015 | verificador sobre `reference` | CI Gates DMPF |
| `RST-02`, `RST-04` | `api/routes_test.go`: POST sem header → 400; `Route.Validate` sem `ContractRef` reprova | `test-race` |
| `RES-17` | `Admission` com limite 1 → segundo request 429 antes do corpo | `test-race` |
| `UOW-11` | `reader_test.go`: sem `BEGIN`; `ErrNotFound` | `test-race` (integration) |
| `BLK-02` | `main_test.go`: `--role api` não constrói publisher; `--role relay` não abre porta | `test-race` |
| `INB-*` idempotência | `e2e_test.go` com reentrega | `test-race` (integration) |
| Contrato × status | `routes_test.go` compara com `openapi.yaml` | `test-race` |
| Tags e camada | guarda de cobertura do `ci.yml` | CI passo 1 |

## Decisões técnicas

- **Reutilizar `orders` e `reservations`** porque já têm UPR, caso de uso,
  mapeador e fixtures; bounded context novo é a vertical do épico de ordem 4.
- **`POST /orders/{id}/items` + `POST /orders/{id}/place`** porque
  `PlaceOrder` só carrega e `Place` rejeita ordem vazia; um `POST /orders` que
  criasse e colocasse exigiria caso de uso novo fora do kernel. Alternativa
  descartada: caso de uso `CreateAndPlace`, porque muda `KRN-04`.
- **`orderspg.NewReader(pool)` no provider** porque `FindOrder` exige `Reader`
  e o repositório é preso à transação por `UOW-01`; um `Reader` por pool é a
  realização de `UOW-11`. Alternativa descartada: `Within` para ler, porque
  viola a leitura fora da UoW declarada pelo caso de uso.
- **Header obrigatório, sem replay persistido** porque `RST-02` trata de
  retry do cliente e `Route` é outbound; replay de resposta é store próprio,
  fora do escopo. Alternativa descartada: reusar `Route.IdempotencyKey` como
  validação de entrada, porque o tipo não tem essa semântica.
- **Targets `serve-*` declarados** porque o papel entra por `--role`, e o
  `serve` que o `nx-go` infere (a partir de `cmd/reference/main.go`, pelo
  nome do diretório) não recebe flag; ele existe, fica sem uso e não é
  redeclarado, e o `build` explícito sobrescreve o inferido. Alternativa
  descartada: um `cmd/` por papel, porque `BLK-02` pede processo, não binário.
- **Um transporte assíncrono (Kafka)** — decisão da guarda-chuva.
- **`type:app` com exclusão do release Docker** — taxonomia canônica é regra
  dura; imagem é matéria do golden path.

## Regras relacionadas

- `SPEC-8HWBWJCB` — decisões transversais.
- `AGENTS.md` — targets Go via `nx:run-commands`; sem redeclarar inferidos.
- `SPEC-EAGAXQN1` — `Sink`, tetos iguais, `channel.Catalog`.

## Verificação e testes

### Critérios de aceite

- [ ] O composition root executa a escrita e a drenagem fim a fim, com estado de
  negócio e registro de outbox na mesma transação (critério 2 do ticket).
- [ ] O e2e republica a mesma entrega e termina com uma reserva e um registro
  de inbox; nenhum artefato declara exactly-once.
- [ ] `POST` sem `Idempotency-Key` → 400 antes de chamar o serviço.
- [ ] `GET /orders/{id}` não abre transação (`reader_test.go`).
- [ ] `--role` ausente → exit 2 listando os três papéis; variável obrigatória
  ausente → exit 2 nomeando-a.
- [ ] `pnpm nx show projects --projects="tag:type:app,!tag:stack:go"` não lista
  `reference-go`.
- [ ] `AGENTS.md` lista a app e as três unidades do `app`;
  `app/README.md` e `doc.go` coerentes.
- [ ] ADR-041 criado; `adr-verify` passa; verificador sem diagnóstico; cadeia
  Go e `biome ci` verdes.

### Cenários de teste

**Rotas (`api/routes_test.go`)**

- Dado o contrato, quando `POST /orders/o-1/items` chega sem
  `Idempotency-Key`, então 400 nomeando o header e `Service.AddItem` não é
  chamado.
- Dado ordem inexistente, quando `POST /orders/o-1/items` com
  `{sku: "A", quantity: 1}`, então 201 e a ordem existe com um item.
- Dado ordem sem itens, quando `POST /orders/o-1/place`, então 422 com
  `CodeEmptyOrder`.
- Dado ordem ausente, quando `GET /orders/o-9`, então 404.
- Dado `Route` sem `ContractRef`, quando `Validate`, então reprova.
- Dado `Admission` com limite 1, quando dois requests simultâneos, então o
  segundo recebe 429 antes de o corpo ser lido.

**Reader (`reader_test.go`, integration)**

- Dado uma ordem gravada, quando `NewReader(pool).Load`, então devolve o
  snapshot e o log do pool não contém `BEGIN`.
- Dado id ausente, quando `Load`, então `ErrNotFound`.

**Fim a fim (`e2e_test.go`, integration)**

- Dado Postgres e Redpanda, quando `POST items` e `POST place` e o relay roda
  um ciclo, então a outbox tem o registro publicado e o tópico tem a mensagem
  com `payload_hash` igual ao gravado.
- Dado a mensagem no tópico, quando o consumer processa, então existe uma
  reserva e um registro de inbox para o `event_id`.
- Dado a mesma mensagem republicada, quando o consumer processa, então
  `DuplicateIgnored`, uma reserva, `Ack` depois do commit.
- Dado `DMPF_PG_DSN` ausente e `CI` definida, quando o e2e roda, então falha
  nomeando a variável; sem `CI`, `t.Skip`.

**Papéis (`cmd/reference/main_test.go`)**

- Dado `--role` ausente, quando o binário sobe, então exit 2 e a mensagem lista
  `api|relay|consumer`.
- Dado `--role api`, quando a cablagem roda, então nenhum `Publisher` é
  construído; dado `--role relay`, nenhuma porta HTTP é aberta.

<critical_constraints>
- [P0] NUNCA construir `pgxpool`, `franz-go`, `net/http` ou `otel` fora deste módulo: `reference` é a única unidade `app` de produção que instancia provider concreto (ADR-015).
- [P0] NUNCA abrir transação para `GET /orders/{id}`: a leitura passa por `orderspg.NewReader(pool)` (`UOW-11`).
- [P0] NUNCA aplicar `Ack`, `Release` ou contenção antes do retorno de `UoW.Within` no consumer (`INB-08`).
- [P0] NUNCA declarar exactly-once: o e2e republica a mesma entrega e prova idempotência pela inbox.
- [P0] NUNCA subir publisher no papel `api` nem servidor HTTP no papel `relay`: um papel por processo (`BLK-02`).
- [P1] Toda rota HTTP declara `ContractRef` para `contracts/openapi/orders/v1/openapi.yaml` (RST-04); `Route.Validate` roda na construção e falha o processo.
- [P1] `MaxAttempts` do `app.Consumer` é lido do `channel.Catalog`, nunca duplicado como literal (ADR-039).
</critical_constraints>

## Escopo fora

- **Replay persistido de resposta por `Idempotency-Key`**: exige store e porta
  próprios; `RST-02` não o exige do servidor.
- **gRPC e SNS/SQS**: guarda-chuva.
- **Processos OS separados no e2e**: `distkit` de `KRN-11` já prova; aqui a
  cablagem.
- **Imagem Docker**: golden path.
- **Bounded context de negócio novo**: golden path.
