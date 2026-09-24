# reservations

Bounded context `reservations` da topologia de referência do kernel DMPF (ADR-044, ADR-046): um módulo Go com os blocos `domain`, `application` e `provider` em um package cada, o bloco `app` no package raiz e os dois harnesses do exemplo (`appkit`, `distkit`) — um binário, três papéis escolhidos por `--role`, sobre o banco próprio do contexto.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve `company.reservations.service.v1.ReservationsService` por gRPC: `Reserve`, `Cancel`, `FindReservation` | `grpc` (servidor, admissão) → `application` do contexto → `provider` do contexto (repositório, reader) sobre `postgres` do kernel (UoW, outbox) |
| `relay` | Drena a outbox de `dmpf_reservations` para `reservations.events` | `app/relay` → `postgres` (claim) + `kafka` (publisher) |
| `consumer` | Consome `OrderPlaced` de `orders.events` pela inbox | `kafka` (consumer) → `app` (adapter) → `application` do contexto → `provider` do contexto sobre `postgres` (inbox, outbox) |

Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`; os blocos de domínio, aplicação e provider, o consumer adapter e os harnesses vieram de `libs/backend/go/{domain,application,postgres,app}/example/reservations` e de `libs/backend/go/testkit/{appkit,distkit}` pelo ADR-046.

Projeto Nx `reservations`, tags `type:app`, `scope:backend`, `stack:go` e `layer:apps`. Import path do módulo: `github.com/mateusmacedo/dmpf/apps/backend/reservations`.

## A primeira decisão vence

Uma reserva pendente vira `Confirmed`, por `Reserve` síncrono ou pelo consumo de `OrderPlaced`, ou `Canceled`, por `Cancel`. Os dois estados são terminais. `Cancel` antes do `OrderPlaced` cria a reserva já cancelada e publica `ReservationCancelled`; o `OrderPlaced` que chega depois é rejeitado por domínio, com a inbox registrando a rejeição e o `Ack` vindo depois do commit (`INB-08`).

## Unidades do manifesto

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `reservations/domain` | Agregado `Reservation` com chave natural permanente (o identificador do pedido) e as UPRs `Reserve` e `Cancel` |
| `application` | `application` | `reservations/application` | `Service` com `Reserve`, `Cancel`, `FindReservation` e o consumo `ConsumeOrderPlaced`/`Consume`, que ramifica pelas sete disposições de FND-04 §6.4 sobre a inbox |
| `provider` | `provider` | `reservations/provider-postgres` | Repositório por chave natural (`order_id`, `ON CONFLICT DO NOTHING`, `GAR-10`) sobre `dmpf_example_reservations`, `Reader`, mapeador para `company.reservations.event.v1` |
| `app`, `app/rpc`, `cmd` | `app` | `reservations/app` | Composition root: o único lugar onde os providers concretos de `reservations` são instanciados (ADR-015); consumer adapter (`NewConsumer`, `Handler`), `Sink`, servidor gRPC e binário |
| `appkit` | `app` | `reservations/appkit` | Harness borda a borda (`KIT-05`): `app.Consumer` real sobre Postgres, alimentado com bytes na borda de protocolo; `Effects` e `Ack` depois do commit |
| `distkit` | `app` | `reservations/distkit` | Harness distribuído (`KIT-06`): dois processos OS sobre Redpanda, reentrega deliberada e `DMPF-R004` (`V32`) |

Todas com `bounded_context` `reservations`. O contrato (`company.reservations.event.v1`, `company.reservations.service.v1`) é a unidade `reservations/contract`, no manifesto de `libs/backend/go/contracts`. O lado `orders` da conversa entra só pela superfície pública: o catálogo nomeia `orders.events` por literal próprio, e o relay e2e produz `OrderPlaced` pelo contrato gerado — nada deste módulo importa `apps/backend/orders`.

## Aliases de import

Os packages do kernel `domain` e `application` têm o mesmo nome dos deste módulo. Onde um arquivo importa os dois, o import do kernel recebe alias pelo papel — `kernel` para o domínio, `usecase` para a aplicação (ADR-045); os packages do contexto ficam bare. Em `wiring.go`, `provider` continua sendo o `grpc` do kernel, porque a composição do servidor não toca o Postgres do contexto diretamente: `NewReservationsService` compõe sobre o `NewService` de `consumer.go`, que é quem liga o `Reader` do `provider`.

## Servidor gRPC

- **Binding no bloco `app`.** O `grpc.ServiceDesc` é montado a partir do descriptor gerado em `contracts`; um teste reprova método do descriptor que não esteja no `ServiceDesc`.
- **Interceptors, nesta ordem:** span de servidor com pai extraído da metadata → admissão por método → deadline obrigatório (`INVALID_ARGUMENT` antes do caso de uso) → contexto de mensagem (`x-correlation-id` preservado ou cunhado, `request_id` próprio como causação, `idempotency-key` recebida só registrada no log) → handler. A cadeia vale só para os métodos de `ReservationsService`: a checagem de saúde passa direto, sem admissão nem prazo obrigatório.
- **Desfechos:** rejeição de domínio no `oneof result`; `NOT_FOUND`, `ABORTED`, `DEADLINE_EXCEEDED` e `INTERNAL` sem detalhe para as falhas técnicas.
- **Saúde:** `NOT_SERVING` até o ping no pool e o `Migrate` opcional, `SERVING` depois, `NOT_SERVING` no shutdown; o log `grpc listening` traz o endereço real.

## Consumo

A ponte `Sink` confirma sem inbox a entrega de tipo diferente do assinado e abre um span de consumo cujo pai é o `traceparent` do envelope (`TRC-07`). O adapter põe no contexto do caso de uso o `correlationid` e o `id` recebido como causação, então o `ReservationConfirmed` herda a cadeia do `OrderPlaced`.

## Configuração

| Variável | Papel | Efeito |
| --- | --- | --- |
| `DMPF_PG_DSN` | todos | Banco `dmpf_reservations` |
| `DMPF_GRPC_ADDR` | `api` | Default `:9090` |
| `DMPF_GRPC_INSECURE` ou `DMPF_GRPC_TLS_CERT_FILE` + `DMPF_GRPC_TLS_KEY_FILE` | `api` | Transporte; sem nenhum, exit 2 |
| `DMPF_GRPC_CLIENT_CA_FILE`, `DMPF_GRPC_TRUSTED_CLIENTS` | `api`, com TLS | CA dos clientes e allowlist de identidades (ex.: `spiffe://dmpf/bff`); o `x-tenant-id` só é lido de peer verificado (ADR-052) |
| `DMPF_MIGRATE` | `api` | Aplica o schema antes de servir |
| `DMPF_KAFKA_BROKERS`, `DMPF_KAFKA_INSECURE` | `relay`, `consumer` | Brokers e opt-out de TLS |
| `DMPF_KAFKA_SASL_MECHANISM`, `DMPF_KAFKA_SASL_USERNAME`, `DMPF_KAFKA_SASL_PASSWORD` ou `DMPF_KAFKA_CLIENT_CERT_FILE` + `DMPF_KAFKA_CLIENT_KEY_FILE` | papéis com Kafka, com TLS | Autenticação do cliente no broker (SCRAM-SHA-256/512 ou certificado); `DMPF_KAFKA_CA_FILE` quando a CA do broker é privada (ADR-052) |
| `DMPF_KAFKA_RESERVATIONS_TOPIC`, `DMPF_KAFKA_RESERVATIONS_DLQ`, `DMPF_KAFKA_GROUP` | `relay` | Canal `reservations.events` |
| `DMPF_KAFKA_ORDERS_TOPIC`, `DMPF_KAFKA_ORDERS_DLQ`, `DMPF_KAFKA_GROUP` | `consumer` | Canal inbound `orders.events` e grupo |
| `DMPF_ORDERS_SOURCE` | `consumer` | Produtor admitido na fronteira do consumo, pelo atributo `source` do envelope (`CTX-27`); default `urn:dmpf:reference-orders`, o do relay de `orders` |
| `DMPF_METRIC_TENANTS` | `api` | Tenants com bucket de admissão e rótulo de métrica próprios (`MET-07`), separados por vírgula; os demais compartilham `other` |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE`, `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | todos | Telemetria e identidade |

Variável obrigatória ausente encerra a partida com exit 2 nomeando-a.

## Rodar localmente

```bash
docker compose -f infra/local/docker-compose.yml exec postgres psql -U app -d app -c 'CREATE DATABASE dmpf_reservations'
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_MIGRATE=true DMPF_GRPC_ADDR=:9091 DMPF_GRPC_INSECURE=true \
  pnpm nx run reservations:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_RESERVATIONS_TOPIC=reservations.events DMPF_KAFKA_RESERVATIONS_DLQ=reservations.events.dlq DMPF_KAFKA_GROUP=reservations \
  pnpm nx run reservations:serve-relay
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_ORDERS_TOPIC=orders.events DMPF_KAFKA_ORDERS_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=reservations \
  pnpm nx run reservations:serve-consumer
```

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `test-distributed`, `govulncheck`, `serve-api`, `serve-relay` e `serve-consumer`.

## Testes

Unitários, sem banco: as UPRs do `domain`, as sete disposições do consumo e a sequência canônica da `application` sobre o `memory`, e o binding e os interceptors do `rpc` por `bufconn` sobre o store em memória, ciclo de saúde, tracer de banco, `Sink` (span com pai remoto e filtro por tipo), catálogos por papel e partida do binário.

Com a build tag `integration` e `DMPF_PG_DSN`, o `test-race` cobre o `provider`, o e2e do consumer adapter e do relay no package raiz e o `appkit`; declara `dependsOn` sobre o `test-race` do `postgres`, porque os harnesses truncam as mesmas tabelas. O `test-distributed` roda só o `distkit`, com as tags `integration,distributed`, e exige Redpanda (`DMPF_KAFKA_BROKERS`); é o que o `dmpf-distributed.yml` executa em pipeline próprio (`KIT-11`). A topologia inteira é provada pelo e2e do `bff`.

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run reservations:test-race
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 \
  pnpm nx run reservations:test-distributed
```
