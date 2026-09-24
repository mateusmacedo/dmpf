# orders

Bounded context `orders` da topologia de referência do kernel DMPF (ADR-044, ADR-046): um módulo Go com os blocos `domain`, `application` e `provider` em um package cada e o bloco `app` no package raiz — um binário, dois papéis escolhidos por `--role`, sobre o banco próprio do contexto.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve `company.orders.service.v1.OrdersService` por gRPC: `AddItem`, `PlaceOrder`, `FindOrder` | `grpc` (servidor, admissão) → `application` do contexto → `provider` do contexto (repositório, reader) sobre `postgres` do kernel (UoW, outbox) |
| `relay` | Drena a outbox de `dmpf_orders` para `orders.events` | `app/relay` → `postgres` (claim) + `kafka` (publisher) |

`orders` não consome canal: `--role consumer` encerra com exit 2. Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`; os blocos de domínio, aplicação e provider vieram de `libs/backend/go/{domain,application,postgres}/example/orders` pelo ADR-046.

Projeto Nx `orders`, tags `type:app`, `scope:backend`, `stack:go` e `layer:apps`. Import path do módulo: `github.com/mateusmacedo/dmpf/apps/backend/orders`.

## Unidades do manifesto

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `orders/domain` | Agregado `Order` com as UPRs `AddItem` e `Place` (FND-03 §8.2, §8.3), mensagens, rejeições `orders/*` |
| `application` | `application` | `orders/application` | `Service` com `AddItem`, `PlaceOrder` (os nove passos de FND-04 §3.2) e `FindOrder`; `Destination = orders.events` |
| `provider` | `provider` | `orders/provider-postgres` | Repositório com optimistic locking sobre `dmpf_example_orders`, `Reader`, mapeador para `company.orders.event.v1` |
| `app`, `app/rpc`, `cmd` | `app` | `orders/app` | Composition root: o único lugar onde os providers concretos de `orders` são instanciados (ADR-015); servidor gRPC e binário |

Todas com `bounded_context` `orders`. O contrato (`company.orders.event.v1`, `company.orders.service.v1`) é a unidade `orders/contract`, declarada no manifesto de `libs/backend/go/contracts`, onde o código gerado mora; é superfície pública, e é por ela que `bff` e `reservations` alcançam `orders` sem importar o seu domínio.

## Aliases de import

Os packages do kernel `domain` e `application` têm o mesmo nome dos deste módulo. Onde um arquivo importa os dois, o import do kernel recebe alias pelo papel — `kernel` para o domínio, `usecase` para a aplicação (ADR-045); os packages do contexto ficam bare. Em `wiring.go`, `provider` é o package Postgres do contexto e o `grpc` do kernel entra como `kernel`.

## Servidor gRPC

- **Binding no bloco `app`.** O `grpc.ServiceDesc` é montado a partir do descriptor gerado em `contracts`; o contrato não carrega código gRPC porque o bloco `contract` não admite `io.network`. Um teste reprova método do descriptor que não esteja no `ServiceDesc`.
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
| `DMPF_GRPC_CLIENT_CA_FILE`, `DMPF_GRPC_TRUSTED_CLIENTS` | `api`, com TLS | CA dos clientes e allowlist de identidades (ex.: `spiffe://dmpf/bff`); o `x-tenant-id` só é lido de peer verificado (ADR-052) |
| `DMPF_MIGRATE` | `api` | Aplica o schema antes de servir |
| `DMPF_ITEM_LIMIT` | `api` | Default 10 |
| `DMPF_METRIC_TENANTS` | `api` | Tenants com bucket de admissão e rótulo de métrica próprios (`MET-07`), separados por vírgula; os demais compartilham `other` |
| `DMPF_KAFKA_BROKERS`, `DMPF_KAFKA_INSECURE` | `relay` | Brokers e opt-out de TLS |
| `DMPF_KAFKA_SASL_MECHANISM`, `DMPF_KAFKA_SASL_USERNAME`, `DMPF_KAFKA_SASL_PASSWORD` ou `DMPF_KAFKA_CLIENT_CERT_FILE` + `DMPF_KAFKA_CLIENT_KEY_FILE` | papéis com Kafka, com TLS | Autenticação do cliente no broker (SCRAM-SHA-256/512 ou certificado); `DMPF_KAFKA_CA_FILE` quando a CA do broker é privada (ADR-052) |
| `DMPF_KAFKA_ORDERS_TOPIC`, `DMPF_KAFKA_ORDERS_DLQ`, `DMPF_KAFKA_GROUP` | `relay` | Endereço, contenção e grupo do canal `orders.events` |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE`, `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | todos | Telemetria e identidade |

Variável obrigatória ausente encerra a partida com exit 2 nomeando-a.

## Rodar localmente

```bash
pnpm nx run bff:infra-up
docker compose -f infra/local/docker-compose.yml exec postgres psql -U app -d app -c 'CREATE DATABASE dmpf_orders'
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_orders?sslmode=disable' DMPF_MIGRATE=true DMPF_GRPC_INSECURE=true \
  pnpm nx run orders:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/dmpf_orders?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_ORDERS_TOPIC=orders.events DMPF_KAFKA_ORDERS_DLQ=orders.events.dlq DMPF_KAFKA_GROUP=orders \
  pnpm nx run orders:serve-relay
```

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck`, `serve-api` e `serve-relay`.

## Testes

Unitários, sem banco: as UPRs do `domain` (pré-condição, efeito, determinismo, snapshot), a sequência canônica e a instrumentação da `application` sobre o `memory` (`memory.Table` por agregado), e o binding e os interceptors do `rpc` por `bufconn` sobre o store em memória (cobertura do descriptor, desfechos, deadline obrigatório, contexto de mensagem gravado na outbox, admissão), ciclo de saúde, tracer de banco e partida do binário por papel. Com a build tag `integration` e `DMPF_PG_DSN`, o `provider` prova repositório, concorrência (dois escritores, um `ErrVersionConflict`) e o e2e até a outbox; o `test-race` declara `dependsOn` sobre o do `postgres`, porque os dois harnesses truncam as mesmas tabelas. A topologia inteira é provada pelo e2e do `bff`.

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run orders:test-race
```
