# orders

Bounded context `orders` da topologia de referência do kernel DMPF (ADR-044, ADR-046, ADR-048): um módulo Go com os blocos `domain`, `application` e `provider` em um package cada e o bloco `app` em `app/` — um binário (`cmd/main.go`), dois papéis escolhidos por `--role`, sobre o banco próprio do contexto.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve `company.orders.service.v1.OrdersService` por gRPC: `AddItem`, `PlaceOrder`, `FindOrder` | `grpc` (servidor, mTLS, admissão) → `application` do contexto (autorização, UPR) → `provider` do contexto (repositório, reader, escopados por tenant) sobre `postgres` do kernel (UoW, outbox) |
| `relay` | Drena a outbox do banco `orders` para `orders.events`, autenticado no broker | `app/relay` → `postgres` (claim) + `kafka` (publisher) |

`orders` não consome canal: `--role consumer` encerra com exit 2. Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`; o layout canônico e a composition root em `app/` vêm do ADR-048. Autenticação de workload, tenant e autorização por permissão vêm da `SPEC-9B6SHEH8` (ADR-049 a ADR-052).

Projeto Nx `orders`, tags `type:app`, `scope:backend`, `stack:go` e `layer:apps`. Import path do módulo: `github.com/mateusmacedo/dmpf/apps/backend/orders`.

## Unidades do manifesto

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `orders/domain` | Agregado `Order` com as UPRs `AddItem` e `Place` (FND-03 §8.2, §8.3), mensagens, rejeições `orders/*` |
| `application` | `application` | `orders/application` | `Service` com `AddItem`, `PlaceOrder` (os nove passos de FND-04 §3.2) e `FindOrder`; `Destination = orders.events`; cada operação declara a permissão que exige (`app/authorization.go`) |
| `provider` | `provider` | `orders/provider-postgres` | Repositório escopado por `tenant_id` via `postgres.Table` (ADR-051) sobre a tabela `orders` (estado em `snapshot` `jsonb`, ADR-053), `Reader`, mapeador para `company.orders.event.v1` |
| `app`, `app/rpc`, `cmd` | `app` | `orders/app` | Composition root: o único lugar onde os providers concretos de `orders` são instanciados (ADR-015); servidor gRPC e binário |

Todas com `bounded_context` `orders`. O contrato (`company.orders.event.v1`, `company.orders.service.v1`) é a unidade `orders/contract`, declarada no manifesto do módulo `apps/backend/orders/contract`, onde o código gerado mora; é superfície pública, e é por ela que `bff` e `reservations` alcançam `orders` sem importar o seu domínio.

## Aliases de import

Os packages do kernel `domain` e `application` têm o mesmo nome dos deste módulo. Onde um arquivo importa os dois, o import do kernel recebe alias pelo papel — `kernel` para o domínio, `usecase` para a aplicação (ADR-045); os packages do contexto ficam bare. Em `wiring.go`, `provider` é o package Postgres do contexto e o `grpc` do kernel entra como `kernel`.

## Autorização e tenant

`AddItem` e `PlaceOrder` exigem `orders:write`; `FindOrder` exige `orders:read` (`app/authorization.go`). A checagem (`usecase.Permitted`, ADR-052) nega sempre que o tenant não foi resolvido, independentemente de haver sujeito; quando o `ExecutionContext` traz sujeito (chamada externa ao fan-out do BFF), a permissão dele é exigida — na cadeia normal, chamada pelo BFF sem sujeito (`CTX-12`), a identidade que autoriza é a do workload verificado por mTLS. O contexto de execução — tenant, prazo, correlação — viaja no `context.Context`, do interceptor gRPC ao provider (ADR-049); toda leitura e escrita da tabela de agregado é escopada por `tenant_id` no choke point `postgres.Table` (ADR-050, ADR-051), e um identificador que existe para outro tenant responde `NOT_FOUND` e vira evento de segurança, nunca `PermissionDenied` (`IDN-12`, `IDN-13`).

## Servidor gRPC

- **Binding no bloco `app`.** O `grpc.ServiceDesc` é montado a partir do descriptor gerado em `contracts`; o contrato não carrega código gRPC porque o bloco `contract` não admite `io.network`. Um teste reprova método do descriptor que não esteja no `ServiceDesc`.
- **Span de servidor:** o `otelgrpc`, ligado como stats handler em `kernelgrpc.NewServer`, abre o span SERVER com pai extraído da metadata (`traceparent`) antes de qualquer interceptor (RF-B4).
- **Interceptors, nesta ordem:** mTLS do peer contra `GRPC_TRUSTED_CLIENTS` (quando TLS está ligado) → desfecho no span e registro de acesso `grpc call` → admissão por método → deadline obrigatório (sem prazo, `INVALID_ARGUMENT` antes do caso de uso, `GRP-04`) → contexto de execução (`x-correlation-id` preservado ou cunhado, `request_id` próprio como causação, `x-tenant-id` lido só de peer verificado, `idempotency-key` exigida nos comandos e levada ao caso de uso) → handler. A cadeia vale só para os métodos de `OrdersService`: a checagem de saúde passa direto, sem admissão nem prazo obrigatório.
- **Desfechos:** a rejeição de domínio volta no `oneof result`; `ErrNotFound` (inclusive acesso a identificador de outro tenant) vira `NOT_FOUND`, conflito de versão `ABORTED`, ausência de tenant ou de permissão `PERMISSION_DENIED`, prazo `DEADLINE_EXCEEDED` e o resto `INTERNAL` sem detalhe.
- **Idempotência dos comandos:** `AddItem` e `PlaceOrder` exigem a metadata `idempotency-key`, no formato `^[A-Za-z0-9._-]{1,128}$` (FND-04 §7.6, ADR-056). Cada comando passa pela inbox do contexto, com `consumer_name` `orders.commands`, na transação do efeito. Metadata ausente é `INVALID_ARGUMENT` com reason `MISSING_IDEMPOTENCY_KEY`, e fora do formato é `INVALID_IDEMPOTENCY_KEY`. A mesma chave com o mesmo pedido devolve a resposta gravada, aceite ou recusa, com o header `idempotent-replayed: true` e sem nova auditoria. Com outro pedido, é `FAILED_PRECONDITION` com `REUSED_IDEMPOTENCY_KEY`; em andamento além da espera, `ABORTED` com `IN_FLIGHT_IDEMPOTENCY_KEY`. A entrada vale 24h.
- **Saúde:** o serviço começa `NOT_SERVING` e passa a `SERVING` depois do ping no pool e do `Migrate` opcional; volta a `NOT_SERVING` no shutdown. O registro `grpc listening` traz o endereço real do listener em `server.address` e `server.port`.
- **Banco observável:** um `pgx.QueryTracer` abre um span CLIENT por consulta feita sob um span, `{db.operation.name} {db.collection.name}`, com `db.query.text` parametrizado e sem os argumentos, e `error.type` = SQLSTATE na falha (RF-B6).

## Configuração

| Variável | Papel | Efeito |
| --- | --- | --- |
| `PG_DSN` | todos | Banco e role `orders` (ADR-053) |
| `GRPC_ADDR` | `api` | Default `:9090` |
| `GRPC_INSECURE` ou `GRPC_TLS_CERT_FILE` + `GRPC_TLS_KEY_FILE` | `api` | Transporte; sem nenhum, exit 2 |
| `GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS` | `api`, com TLS | CA dos clientes e allowlist de identidades por URI/DNS SAN (nunca CN — ex.: `spiffe://dmpf/bff`); com TLS ligado os dois são obrigatórios (ADR-052), e o `x-tenant-id` só é lido de peer verificado |
| `MIGRATE` | `api` | Aplica o schema antes de servir |
| `ITEM_LIMIT` | `api` | Default 10 |
| `METRIC_TENANTS` | `api` | Tenants com bucket de admissão e rótulo de métrica próprios (`MET-07`), separados por vírgula; os demais compartilham `other` |
| `KAFKA_BROKERS`, `KAFKA_INSECURE` | `relay` | Brokers e opt-out de TLS |
| `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD` ou `KAFKA_CLIENT_CERT_FILE` + `KAFKA_CLIENT_KEY_FILE` | `relay`, com TLS | Autenticação do cliente no broker (SCRAM-SHA-256/512 ou certificado), obrigatória sempre que `KAFKA_INSECURE` não está ligado (ADR-052); `KAFKA_CA_FILE` quando a CA do broker é privada |
| `KAFKA_ORDERS_TOPIC`, `KAFKA_ORDERS_DLQ`, `KAFKA_GROUP` | `relay` | Endereço, contenção e grupo do canal `orders.events` |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` | todos | Identidade do recurso OTel (`service.version`, `service.instance.id`, `dmpf.process.role`, `deployment.environment.name`); sem `OTEL_SERVICE_NAME`, o serviço é `orders`, e sem `dmpf.process.role` o papel vem de `--role`; versão e instância não têm default, e sem elas a partida falha com `ErrResourceIncomplete` (exit 1). O `deploy/.env.example` não declara papel, porque vale para todos; cada `serve-<papel>` e o `docker:run-relay` (no container, por `-e`) acrescentam `service.instance.id=orders-local-<papel>,dmpf.process.role=<papel>` depois de carregar o `deploy/.env`, e a chave repetida fica com o último valor |
| `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_ENDPOINT` | todos | Exportação OTLP; os manifestos declaram `grpc` e `http://<collector>:4317`, e o esquema `http://` desliga o TLS; sem protocolo, vale o `http/protobuf` do `autoexport`, que o Collector não recebe |
| `OTEL_TRACES_SAMPLER_ARG`, `OTEL_LOGS_EXPORTER`, `OTEL_PROPAGATORS`, `OTEL_GO_X_OBSERVABILITY` | todos | Os manifestos declaram `1.0`, `otlp`, `tracecontext` e `true`; com `none` em `OTEL_{TRACES,METRICS,LOGS}_EXPORTER`, o sinal não é exportado, como nos harnesses de teste |

Variável obrigatória ausente encerra a partida com exit 2 nomeando-a.

Os prazos de idempotência e de purga vêm de `Defaults()`, sem variável de
ambiente, e `Validate` recusa valor não positivo com `ErrInvalidPolicy`:

| Campo | Padrão | Uso |
| --- | --- | --- |
| `IdempotencyWait` | 1s | Espera máxima por comando concorrente da mesma chave |
| `IdempotencyRetention` | 24h | Vida da entrada de comando na inbox |
| `OutboxRetention` | 168h | Idade a partir da qual a outbox publicada é purgada |
| `PurgeInterval`, `PurgeBatch` | 15min, 1000 | Intervalo e lote de cada purga |

O `api` purga as entradas de comando vencidas depois do `Migrate`, e o `relay`
purga a outbox publicada. As duas purgas param antes de o pool fechar.

## Rodar localmente

```bash
pnpm nx run bff:infra-up
docker compose -f infra/local/docker-compose.yml --profile postgres --profile dmpf up -d postgres-init
PG_DSN='postgres://orders:orders-local@localhost:5432/orders?sslmode=disable' MIGRATE=true GRPC_INSECURE=true \
  pnpm nx run orders:serve-api
PG_DSN='postgres://orders:orders-local@localhost:5432/orders?sslmode=disable' KAFKA_BROKERS=localhost:9092 KAFKA_INSECURE=true \
  KAFKA_ORDERS_TOPIC=orders.events KAFKA_ORDERS_DLQ=orders.events.dlq KAFKA_GROUP=orders \
  pnpm nx run orders:serve-relay
```

`GRPC_INSECURE=true` e `KAFKA_INSECURE=true` são o opt-out de desenvolvimento (ADR-052); a topologia completa via `docker compose --profile dmpf` já sobe com mTLS entre `api` e BFF e SASL no Kafka interno (`infra/README.md`).

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck`, `serve-api` e `serve-relay`, mais os de container: `docker:build`, `docker:run` (papel `api`) e `docker:run-relay`, que o `bff:docker:run` sobe.

## Testes

Unitários, sem banco: as UPRs do `domain` (pré-condição, efeito, determinismo, snapshot), a sequência canônica, a autorização por permissão e a instrumentação da `application` sobre o `memory` (`memory.Table` por agregado, escopado por tenant como o `postgres.Table`), e o binding e os interceptors do `rpc` por `bufconn` sobre o store em memória (cobertura do descriptor, desfechos, mTLS do peer, deadline obrigatório, contexto de execução gravado na outbox, admissão), ciclo de saúde, tracer de banco e partida do binário por papel. Com a build tag `integration` e `PG_DSN`, o `provider` prova repositório, escopo de tenant, acesso cruzado (`CrossTenantAccess`), concorrência (dois escritores, um `ErrVersionConflict`) e o e2e até a outbox; cada teste roda num banco `orders_test_<id>` próprio, que o `tb/pg` cria e apaga no servidor de `PG_DSN`. A topologia inteira é provada pelo e2e do `bff`.

O target sobe a infra de testes (`testkit:test-infra-up`) e o `tools/test-env.sh` preenche o `PG_DSN` com o Postgres dela, na porta 15432, a partir do `.env.example` da raiz:

```bash
pnpm nx run orders:test-race
```
