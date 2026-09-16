# dmpf-reference-orders-go

Contexto `orders` da topologia de referência do kernel DMPF (ADR-044): um binário, dois papéis escolhidos por `--role`, sobre o banco próprio do contexto.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve `company.orders.service.v1.OrdersService` por gRPC: `AddItem`, `PlaceOrder`, `FindOrder` | `dmpf-provider-grpc` (servidor, admissão) → `ordersapp` → `dmpf-provider-postgres` (UoW, outbox, reader) |
| `relay` | Drena a outbox de `dmpf_orders` para `orders.events` | `dmpf-app/relay` → `dmpf-provider-postgres` (claim) + `dmpf-provider-kafka` (publisher) |

`orders` não consome canal: `--role consumer` encerra com exit 2. Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`.

## Unidade do manifesto

`dmpf-kernel/reference-orders-app`, bloco `app`, `bounded_context` `dmpf-kernel`. É composition root: o único lugar onde os providers concretos de `orders` são instanciados (ADR-015).

## Servidor gRPC

- **Binding no bloco `app`.** O `grpc.ServiceDesc` é montado a partir do descriptor gerado em `dmpf-contracts`; o contrato não carrega código gRPC porque o bloco `contract` não admite `io.network`. Um teste reprova método do descriptor que não esteja no `ServiceDesc`.
- **Interceptors, nesta ordem:** span de servidor com pai extraído da metadata (`traceparent`) → admissão por método → deadline obrigatório (sem prazo, `INVALID_ARGUMENT` antes do caso de uso, `GRP-04`) → contexto de mensagem (`x-correlation-id` preservado ou cunhado, `request_id` próprio como causação, `idempotency-key` recebida só registrada no log) → handler. A cadeia vale só para os métodos de `OrdersService`: a checagem de saúde passa direto, sem admissão nem prazo obrigatório.
- **Desfechos:** a rejeição de domínio volta no `oneof result`; `ErrNotFound` vira `NOT_FOUND`, conflito de versão `ABORTED`, prazo `DEADLINE_EXCEEDED` e o resto `INTERNAL` sem detalhe.
- **Saúde:** o serviço começa `NOT_SERVING` e passa a `SERVING` depois do ping no pool e do `Migrate` opcional; volta a `NOT_SERVING` no shutdown. O log `grpc listening` traz o endereço real do listener.
- **Banco observável:** um `pgx.QueryTracer` abre um span por consulta, sem SQL nem argumentos.

## Configuração

| Variável | Papel | Efeito |
| --- | --- | --- |
| `DMPF_PG_DSN` | todos | Banco `dmpf_orders` |
| `DMPF_GRPC_ADDR` | `api` | Default `:9090` |
| `DMPF_GRPC_INSECURE` ou `DMPF_GRPC_TLS_CERT_FILE` + `DMPF_GRPC_TLS_KEY_FILE` | `api` | Transporte; sem nenhum, exit 2 |
| `DMPF_MIGRATE` | `api` | Aplica o schema antes de servir |
| `DMPF_ITEM_LIMIT` | `api` | Default 10 |
| `DMPF_KAFKA_BROKERS`, `DMPF_KAFKA_INSECURE` | `relay` | Brokers e opt-out de TLS |
| `DMPF_KAFKA_ORDERS_TOPIC`, `DMPF_KAFKA_ORDERS_DLQ`, `DMPF_KAFKA_GROUP` | `relay` | Endereço, contenção e grupo do canal `orders.events` |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE`, `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | todos | Telemetria e identidade |

Variável obrigatória ausente encerra a partida com exit 2 nomeando-a.

## Rodar localmente

```bash
pnpm nx run dmpf-reference-bff-go:infra-up
docker compose -f infra/local/docker-compose.yml exec postgres psql -U app -d app -c 'CREATE DATABASE dmpf_orders'
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_orders?sslmode=disable' DMPF_MIGRATE=true DMPF_GRPC_INSECURE=true \
  pnpm nx run dmpf-reference-orders-go:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_orders?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_ORDERS_TOPIC=orders.events DMPF_KAFKA_ORDERS_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=orders \
  pnpm nx run dmpf-reference-orders-go:serve-relay
```

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck`, `serve-api` e `serve-relay`.

## Testes

Unitários, sem banco: binding e interceptors por `bufconn` sobre o store em memória (cobertura do descriptor, desfechos, deadline obrigatório, contexto de mensagem gravado na outbox, admissão), ciclo de saúde, tracer de banco e partida do binário por papel. A topologia inteira é provada pelo e2e do `dmpf-reference-bff-go`.
