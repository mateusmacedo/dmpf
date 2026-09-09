# dmpf-reference-go

Composition root de referência do kernel DMPF: o primeiro projeto Nx sob `apps/` e o único lugar do workspace onde instanciar provider concreto é permissivo (ADR-015). Um binário, três papéis escolhidos por `--role`:

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Borda HTTP de `orders`: `POST /orders/{id}/items`, `POST /orders/{id}/place`, `GET /orders/{id}` | `dmpf-provider-http` (rotas, admissão) → `ordersapp` → `dmpf-provider-postgres` (UoW, outbox) |
| `relay` | Drena a outbox para o Kafka | `dmpf-app/relay` → `dmpf-provider-postgres` (claim) + `dmpf-provider-kafka` (publisher) |
| `consumer` | Lê do Kafka e alimenta `reservations` pela inbox | `dmpf-provider-kafka` (consumer) → `dmpf-app` (adapter) → `reservationsapp` → `dmpf-provider-postgres` (inbox) |

Criado por `KRN-12` (ARQ-545, `docs/specs/SPEC-6QT9SBAS-dmpf-reference-composition-root.md`; guarda-chuva em `docs/specs/SPEC-8HWBWJCB-dmpf-sdk-referencia-bom.md`).

## Unidade do manifesto

| Unidade | Bloco | Packages |
| --- | --- | --- |
| `dmpf-kernel/reference-app` | `app` | raiz do módulo, `api`, `cmd/dmpf-reference` |

O `external` do manifesto é a união do que os providers cabeados declaram (pgx, franz-go, OpenTelemetry SDK e exportadores OTLP): a composition root é o único lugar que os alcança ao mesmo tempo.

## O canal e a assinatura por tipo

O canal Kafka chama-se `ordersapp.Destination` (`orders.events`), porque o publisher resolve pelo destino que o caso de uso autorou; só o endereço físico, o grupo e o tópico de contenção vêm do ambiente. Esse destino carrega os **dois** eventos do agregado `orders` — `ItemAdded` e `OrderPlaced` — e o consumer de `reservations` só entende o segundo. O `Sink` da app (a ponte transporte→adapter de FND-06 §11) é também a assinatura: uma entrega de tipo diferente do assinado é confirmada sem passar pelo adapter nem pela inbox; o tipo assinado e os bytes que não decodificam vão ao adapter como sempre (envelope inválido continua indo para a quarentena, INB-10). Um canal por tipo de evento é matéria do generator (SPEC-H1A190Y8), não desta app.

O contexto de mensagem (`correlationid`, `causationid`, `traceparent`) nasce na borda: o `api` abre o span de servidor, injeta o `traceparent` W3C, usa `X-Correlation-ID` do cliente ou cunha um, e o application service copia tudo para cada linha da outbox — a causação de quem inicia a cadeia é o próprio `message_id` (FND-05). Sem isso o relay não drenaria nada (ADR-038).

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
