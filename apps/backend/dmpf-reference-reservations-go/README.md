# dmpf-reference-reservations-go

Contexto `reservations` da topologia de referência do kernel DMPF (ADR-044): um binário, três papéis escolhidos por `--role`, sobre o banco próprio do contexto.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve `company.reservations.service.v1.ReservationsService` por gRPC: `Reserve`, `Cancel`, `FindReservation` | `dmpf-provider-grpc` (servidor, admissão) → `reservationsapp` → `dmpf-provider-postgres` (UoW, outbox, reader) |
| `relay` | Drena a outbox de `dmpf_reservations` para `reservations.events` | `dmpf-app/relay` → `dmpf-provider-postgres` (claim) + `dmpf-provider-kafka` (publisher) |
| `consumer` | Consome `OrderPlaced` de `orders.events` pela inbox | `dmpf-provider-kafka` (consumer) → `dmpf-app` (adapter) → `reservationsapp` → `dmpf-provider-postgres` (inbox, outbox) |

Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`.

## A primeira decisão vence

Uma reserva pendente vira `Confirmed`, por `Reserve` síncrono ou pelo consumo de `OrderPlaced`, ou `Canceled`, por `Cancel`. Os dois estados são terminais. `Cancel` antes do `OrderPlaced` cria a reserva já cancelada e publica `ReservationCancelled`; o `OrderPlaced` que chega depois é rejeitado por domínio, com a inbox registrando a rejeição e o `Ack` vindo depois do commit (`INB-08`).

## Unidade do manifesto

`dmpf-kernel/reference-reservations-app`, bloco `app`, `bounded_context` `dmpf-kernel`. É composition root: o único lugar onde os providers concretos de `reservations` são instanciados (ADR-015).

## Servidor gRPC

- **Binding no bloco `app`.** O `grpc.ServiceDesc` é montado a partir do descriptor gerado em `dmpf-contracts`; um teste reprova método do descriptor que não esteja no `ServiceDesc`.
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
| `DMPF_MIGRATE` | `api` | Aplica o schema antes de servir |
| `DMPF_KAFKA_BROKERS`, `DMPF_KAFKA_INSECURE` | `relay`, `consumer` | Brokers e opt-out de TLS |
| `DMPF_KAFKA_RESERVATIONS_TOPIC`, `DMPF_KAFKA_RESERVATIONS_DLQ`, `DMPF_KAFKA_GROUP` | `relay` | Canal `reservations.events` |
| `DMPF_KAFKA_ORDERS_TOPIC`, `DMPF_KAFKA_ORDERS_DLQ`, `DMPF_KAFKA_GROUP` | `consumer` | Canal inbound `orders.events` e grupo |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE`, `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | todos | Telemetria e identidade |

Variável obrigatória ausente encerra a partida com exit 2 nomeando-a.

## Rodar localmente

```bash
docker compose -f infra/local/docker-compose.yml exec postgres psql -U app -d app -c 'CREATE DATABASE dmpf_reservations'
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_MIGRATE=true DMPF_GRPC_ADDR=:9091 DMPF_GRPC_INSECURE=true \
  pnpm nx run dmpf-reference-reservations-go:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_RESERVATIONS_TOPIC=reservations.events DMPF_KAFKA_RESERVATIONS_DLQ=reservations.events.dlq DMPF_KAFKA_GROUP=reservations \
  pnpm nx run dmpf-reference-reservations-go:serve-relay
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_reservations?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_ORDERS_TOPIC=orders.events DMPF_KAFKA_ORDERS_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=reservations \
  pnpm nx run dmpf-reference-reservations-go:serve-consumer
```

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck`, `serve-api`, `serve-relay` e `serve-consumer`.

## Testes

Unitários, sem banco: binding e interceptors por `bufconn` sobre o store em memória, ciclo de saúde, tracer de banco, `Sink` (span com pai remoto e filtro por tipo), catálogos por papel e partida do binário. A topologia inteira é provada pelo e2e do `dmpf-reference-bff-go`.
