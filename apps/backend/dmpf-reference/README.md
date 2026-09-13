# dmpf-reference-go

Composition root de referência do kernel DMPF: o primeiro projeto Nx sob `apps/` e o único lugar do workspace onde instanciar provider concreto é permissivo (ADR-015). Um binário, três papéis escolhidos por `--role`:

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Borda HTTP de `orders`: `POST /orders/{id}/items`, `POST /orders/{id}/place`, `GET /orders/{id}` | `dmpf-provider-http` (rotas, admissão) → `ordersapp` → `dmpf-provider-postgres` (UoW, outbox) |
| `relay` | Drena a outbox para o Kafka | `dmpf-app/relay` → `dmpf-provider-postgres` (claim) + `dmpf-provider-kafka` (publisher) |
| `consumer` | Lê do Kafka e alimenta `reservations` pela inbox | `dmpf-provider-kafka` (consumer) → `dmpf-app` (adapter) → `reservationsapp` → `dmpf-provider-postgres` (inbox) |

Criado por `KRN-12` (ARQ-545, `docs/specs/SPEC-6QT9SBAS-dmpf-reference-composition-root.md`; guarda-chuva em `docs/specs/SPEC-8HWBWJCB-dmpf-sdk-referencia-bom.md`).

## Estrutura

Um binário (`cmd/dmpf-reference`), três processos. Providers concretos só nesta composition root (ADR-015, BLK-02).

Contexto — quem chama, o que a app alcança:

```mermaid
flowchart LR
  client["Cliente HTTP<br/>opera o contrato OpenAPI de orders"]
  ref["dmpf-reference<br/>api, relay ou consumer"]
  pg[("Postgres<br/>estado, outbox e inbox")]
  kafka[/"Kafka / Redpanda<br/>orders.events e contenção"/]
  otel["Collector OTLP<br/>opcional: ausente, telemetria em memória"]

  client -->|"POST/GET orders (HTTP JSON)"| ref
  ref -->|"UoW, outbox, inbox"| pg
  ref -->|"publica e consome envelopes"| kafka
  ref -->|"traces e métricas (OTLP/gRPC)"| otel
```

Containers — um processo por papel, todos do mesmo binário:

```mermaid
flowchart TB
  client["Cliente HTTP"]

  subgraph ref["dmpf-reference (--role)"]
    api["api<br/>net/http, borda REST de orders"]
    relay["relay<br/>dmpf-app/relay, drena a outbox"]
    consumer["consumer<br/>dmpf-app + Sink, assina OrderPlaced"]
  end

  pg[("Postgres<br/>pedidos, reservas, outbox, inbox")]
  ordersTopic[/"orders.events<br/>ItemAdded e OrderPlaced"/]
  ordersDlq[/"orders.events.dlq<br/>contenção do canal"/]
  resTopic[/"reservations.events<br/>ReservationConfirmed"/]

  client -->|"POST items, POST place, GET order"| api
  api -->|"pedido + linha de outbox na mesma Tx"| pg
  relay -->|"claim por lease, marca published"| pg
  relay -->|"publica envelope CloudEvents"| ordersTopic
  relay -->|"drena Destination de reservations"| resTopic
  ordersTopic -->|"lê pelo group do canal"| consumer
  consumer -->|"inbox + Reserve na mesma Tx"| pg
  consumer -->|"contenção após o teto de tentativas"| ordersDlq
```

Packages do módulo — as libs do kernel entram só como dependência, não como processo:

```mermaid
flowchart LR
  subgraph bin["cmd/dmpf-reference"]
    main["main --role"]
  end
  subgraph root["package dmpfreference"]
    run["Run / RunWith"]
    cfg["FromEnv"]
    wire["serveAPI / runRelay / runConsumer"]
    cat["NewCatalog"]
    sink["Sink"]
    ports["Clock, IDs"]
  end
  subgraph edge["package api"]
    routes["Routes + NewHandler"]
    mw["admissão, Idempotency-Key, message context"]
    h["handlers"]
  end
  main --> cfg --> run --> wire
  wire --> routes
  routes --> mw --> h
  wire --> cat
  wire --> sink
  wire --> ports
```

## Unidade do manifesto

| Unidade | Bloco | Packages |
| --- | --- | --- |
| `dmpf-kernel/reference-app` | `app` | raiz do módulo, `api`, `cmd/dmpf-reference` |

O `external` do manifesto é a união do que os providers cabeados declaram (pgx, franz-go, OpenTelemetry SDK e exportadores OTLP): a composition root é o único lugar que os alcança ao mesmo tempo.

## O canal e a assinatura por tipo

O canal Kafka chama-se `ordersapp.Destination` (`orders.events`), porque o publisher resolve pelo destino que o caso de uso autorou; só o endereço físico, o grupo e o tópico de contenção vêm do ambiente. Esse destino carrega os **dois** eventos do agregado `orders` — `ItemAdded` e `OrderPlaced` — e o consumer de `reservations` só entende o segundo. O `Sink` da app (a ponte transporte→adapter de FND-06 §11) é também a assinatura: uma entrega de tipo diferente do assinado é confirmada sem passar pelo adapter nem pela inbox; o tipo assinado e os bytes que não decodificam vão ao adapter como sempre (envelope inválido continua indo para a quarentena, INB-10). Um canal por tipo de evento é matéria do generator (SPEC-H1A190Y8), não desta app.

O contexto de mensagem (`correlationid`, `causationid`, `traceparent`) nasce na borda: o `api` abre o span de servidor, injeta o `traceparent` W3C, usa `X-Correlation-ID` do cliente ou cunha um, e o application service copia tudo para cada linha da outbox — a causação de quem inicia a cadeia é o próprio `message_id` (FND-05). Sem isso o relay não drenaria nada (ADR-038).

## Fluxos

Cadeia que o e2e prova: escrita na mesma transação, dreno at-least-once, consumo idempotente pela inbox. Nenhum artefato afirma exactly-once.

```mermaid
sequenceDiagram
  autonumber
  participant C as Cliente
  participant API as api
  participant Svc as ordersapp.Service
  participant PG as Postgres
  participant Rel as relay
  participant K as Kafka orders.events
  participant Snk as Sink
  participant Adp as dmpf-app Consumer
  participant Rsv as reservationsapp

  C->>API: POST /orders/{id}/items<br/>Idempotency-Key, X-Correlation-ID
  API->>API: admissão, span, message context
  API->>Svc: AddItem
  Svc->>PG: UoW: estado + outbox ItemAdded
  API-->>C: 201

  C->>API: POST /orders/{id}/place
  API->>Svc: PlaceOrder
  Svc->>PG: UoW: estado + outbox OrderPlaced
  API-->>C: 200

  Rel->>PG: claim por lease
  Rel->>K: publica envelope
  Rel->>PG: marca published

  K->>Snk: Delivery Raw
  alt type ≠ OrderPlaced.v1
    Snk->>K: Ack sem inbox
  else type assinado ou bytes ilegíveis
    Snk->>Adp: Consume
    Adp->>Rsv: Reserve via inbox
    Rsv->>PG: inbox + reserva na mesma Tx
    Adp-->>K: Ack / Release / contenção
  end
```

Borda HTTP, da recusa mais barata ao caso de uso:

```mermaid
sequenceDiagram
  autonumber
  participant C as Cliente
  participant CORS as withCORS
  participant Adm as Admission
  participant Idem as Idempotency-Key
  participant Ctx as withMessageContext
  participant H as handler
  participant Svc as ordersapp

  C->>CORS: POST /orders/{id}/items
  CORS->>Adm: 429 antes de ler o corpo
  Adm->>Idem: POST sem chave → 400
  Idem->>Ctx: span + correlationid + traceparent
  Ctx->>H: decode do contrato
  H->>Svc: AddItem / PlaceOrder / FindOrder
  Svc-->>H: Outcome ou Failure
  H-->>C: JSON do contrato
```

Relay (processo separado do caminho de requisição):

```mermaid
sequenceDiagram
  autonumber
  participant Rel as relay.Relay
  participant Store as OutboxStore Postgres
  participant Pub as Kafka Publisher
  participant Cat as channel.Catalog

  Rel->>Store: claim por lease
  Store-->>Rel: linha pending
  Rel->>Cat: resolve Destination
  Rel->>Pub: publica bytes do envelope
  Rel->>Store: marca published
```

## Rodar localmente

A infraestrutura é a mesma do CI (`.github/workflows/ci.yml`): um Postgres e um Redpanda por `docker run`, sem compose.

```bash
docker run -d --name dmpf-postgres -p 5432:5432 \
  -e POSTGRES_USER=app -e POSTGRES_PASSWORD=app -e POSTGRES_DB=app \
  postgres:16-alpine
docker run -d --name dmpf-redpanda -p 9092:9092 \
  redpandadata/redpanda:v26.2.2 redpanda start --mode dev-container --smp 1
```

Cada papel lê a configuração só de variáveis de ambiente e reprova na partida (exit 2) nomeando a variável obrigatória ausente:

| Variável | Papéis | Default | Significado |
| --- | --- | --- | --- |
| `DMPF_PG_DSN` | todos | — (obrigatória) | DSN do Postgres |
| `DMPF_HTTP_ADDR` | `api` | `:8080` | Endereço do servidor HTTP |
| `DMPF_MIGRATE` | `api` | `false` | `true` aplica `dmpfpostgres.Migrate` antes de servir |
| `DMPF_KAFKA_BROKERS` | `relay`, `consumer` | — (obrigatória) | Brokers, separados por vírgula |
| `DMPF_KAFKA_INSECURE` | `relay`, `consumer` | `false` | `true` desliga TLS — só desenvolvimento e CI |
| `DMPF_KAFKA_TOPIC` | `relay`, `consumer` | — (obrigatória) | Endereço físico do canal `orders.events` |
| `DMPF_KAFKA_DLQ` | `relay`, `consumer` | — (obrigatória) | Tópico de contenção do canal |
| `DMPF_KAFKA_GROUP` | `consumer` | — (obrigatória) | Consumer group |
| `DMPF_OTLP_ENDPOINT` | todos | — | Collector OTLP/gRPC; ausente, a telemetria fica em memória (modo dev) |
| `DMPF_OTLP_INSECURE` | todos | `false` | `true` desliga TLS no OTLP |
| `DMPF_SERVICE` | todos | `dmpf-reference` | `service.name` do resource |
| `DMPF_SERVICE_VERSION` | todos | `dev` | `service.version` do resource |
| `DMPF_INSTANCE_ID` | todos | hostname | `service.instance.id` do resource |
| `DMPF_ITEM_LIMIT` | `api` | `10` | Limite de itens por pedido |

```bash
export DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable'
export DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true
export DMPF_KAFKA_TOPIC=orders.events DMPF_KAFKA_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=reservations
DMPF_MIGRATE=true pnpm nx run dmpf-reference-go:serve-api
pnpm nx run dmpf-reference-go:serve-relay
pnpm nx run dmpf-reference-go:serve-consumer
```

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck` seguem o padrão dos módulos Go do workspace; `serve-api`, `serve-relay` e `serve-consumer` executam `go run ./cmd/dmpf-reference --role <papel>` a partir da raiz do módulo. O `test-race` declara `dependsOn` sobre os de `dmpf-provider-postgres-go` e `dmpf-app-go`, porque os três compartilham o Postgres do job e cada harness faz `TRUNCATE`.

O plugin `@nx-go/nx-go` infere `build` e `serve` para todo módulo com `cmd/<nome-do-diretório>/main.go`, e `dmpf-reference` cai nesse caso. O `build` declarado acima sobrescreve o inferido (merge do Nx); o `serve` inferido existe, mas não é usado — o papel entra por `--role`, e é isso que os três `serve-*` fazem.

## Testes

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' \
DMPF_KAFKA_BROKERS=localhost:9092 \
pnpm nx run dmpf-reference-go:test-race
```

Os testes de unidade (`config`, `cmd`, `api`, catálogo) rodam sem infraestrutura. O e2e (`e2e_test.go`, build tag `integration`) sobe os três papéis em goroutines sobre o Postgres e o Redpanda locais, cria tópicos únicos por execução e republica a mesma entrega para provar que o consumo termina com uma reserva e um registro de inbox — sem exactly-once em nenhum artefato. Sem `DMPF_PG_DSN` ele faz `t.Skip`; com `CI` definida, reprova.
