# reservations

Bounded context `reservations` da topologia de referência do kernel DMPF (ADR-044, ADR-046, ADR-048): um módulo Go com os blocos `domain`, `application` e `provider` em um package cada, o bloco `app` em `app/` e os dois harnesses do exemplo (`appkit`, `distkit`) — um binário (`cmd/main.go`), três papéis escolhidos por `--role`, sobre o banco próprio do contexto.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve `company.reservations.service.v1.ReservationsService` por gRPC: `Reserve`, `Cancel`, `FindReservation` | `grpc` (servidor, mTLS, admissão) → `application` do contexto (autorização, UPR) → `provider` do contexto (repositório, reader, escopados por tenant) sobre `postgres` do kernel (UoW, outbox) |
| `relay` | Drena a outbox do banco `reservations` para `reservations.events`, autenticado no broker | `app/relay` → `postgres` (claim) + `kafka` (publisher) |
| `consumer` | Consome `OrderPlaced` de `orders.events` pela inbox, só de fronteira verificada | `kafka` (consumer) → `app` (adapter) → `application` do contexto → `provider` do contexto sobre `postgres` (inbox, outbox) |

Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`; o layout canônico e a composition root em `app/` vêm do ADR-048. Autenticação de workload, tenant e autorização por permissão vêm da `SPEC-9B6SHEH8` (ADR-049 a ADR-052).

Projeto Nx `reservations`, tags `type:app`, `scope:backend`, `stack:go` e `layer:apps`. Import path do módulo: `github.com/mateusmacedo/dmpf/apps/backend/reservations`.

## A primeira decisão vence

Uma reserva pendente vira `Confirmed`, por `Reserve` síncrono ou pelo consumo de `OrderPlaced`, ou `Canceled`, por `Cancel`. Os dois estados são terminais. `Cancel` antes do `OrderPlaced` cria a reserva já cancelada e publica `ReservationCancelled`; o `OrderPlaced` que chega depois é rejeitado por domínio, com a inbox registrando a rejeição e o `Ack` vindo depois do commit (`INB-08`).

## Unidades do manifesto

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `reservations/domain` | Agregado `Reservation` com chave natural permanente (o identificador do pedido) e as UPRs `Reserve` e `Cancel` |
| `application` | `application` | `reservations/application` | `Service` com `Reserve`, `Cancel`, `FindReservation` e o consumo `ConsumeOrderPlaced`/`Consume`, que ramifica pelas sete disposições de FND-04 §6.4 sobre a inbox; cada operação declara a permissão que exige (`app/authorization.go`) |
| `provider` | `provider` | `reservations/provider-postgres` | Repositório escopado por `tenant_id` via `postgres.Table` (ADR-051), por chave natural (`order_id`, `ON CONFLICT DO NOTHING`, `GAR-10`) sobre a tabela `reservations` (estado em `snapshot` `jsonb`, ADR-053), `Reader`, mapeador para `company.reservations.event.v1` |
| `app`, `app/rpc`, `cmd` | `app` | `reservations/app` | Composition root: o único lugar onde os providers concretos de `reservations` são instanciados (ADR-015); consumer adapter (`NewConsumer`, `Handler`), `Sink`, servidor gRPC e binário |
| `appkit` | `app` | `reservations/appkit` | Harness borda a borda (`KIT-05`): `app.Consumer` real sobre Postgres, alimentado com bytes na borda de protocolo; `Effects` e `Ack` depois do commit |
| `distkit` | `app` | `reservations/distkit` | Harness distribuído (`KIT-06`): dois processos OS sobre Redpanda, reentrega deliberada e `DMPF-R004` (`V32`) |

Todas com `bounded_context` `reservations`. O contrato (`company.reservations.event.v1`, `company.reservations.service.v1`) é a unidade `reservations/contract`, no manifesto do módulo `apps/backend/reservations/contract`. O lado `orders` da conversa entra só pela superfície pública: o catálogo nomeia `orders.events` por literal próprio, e o relay e2e produz `OrderPlaced` pelo contrato gerado — nada deste módulo importa `apps/backend/orders`.

## Aliases de import

Os packages do kernel `domain` e `application` têm o mesmo nome dos deste módulo. Onde um arquivo importa os dois, o import do kernel recebe alias pelo papel — `kernel` para o domínio, `usecase` para a aplicação (ADR-045); os packages do contexto ficam bare. Em `wiring.go`, `provider` continua sendo o `grpc` do kernel, porque a composição do servidor não toca o Postgres do contexto diretamente: `NewReservationsService` compõe sobre o `NewService` de `consumer.go`, que é quem liga o `Reader` do `provider`.

## Autorização e tenant

`ConsumeOrderPlaced` exige `reservations:consume`; `Reserve` e `Cancel` exigem `reservations:write`; `FindReservation` exige `reservations:read` (`app/authorization.go`). A checagem (`usecase.Permitted`, ADR-052) nega sempre que o tenant não foi resolvido; na cadeia normal (chamada gRPC do BFF, ou reconstrução do contexto a partir do envelope consumido) não há sujeito, e quem autoriza é a identidade do workload verificado — por mTLS na API, pela fronteira Kafka verificada no consumo. O contexto de execução — tenant, prazo, correlação — viaja no `context.Context`, do ingress ao provider (ADR-049); toda leitura e escrita da tabela de agregado é escopada por `tenant_id` no choke point `postgres.Table` (ADR-050, ADR-051), e um identificador que existe para outro tenant responde `NOT_FOUND` e vira evento de segurança, nunca `PermissionDenied` (`IDN-12`, `IDN-13`).

## Servidor gRPC

- **Binding no bloco `app`.** O `grpc.ServiceDesc` é montado a partir do descriptor gerado em `apps/backend/reservations/contract`; um teste reprova método do descriptor que não esteja no `ServiceDesc`.
- **Span de servidor:** o `otelgrpc`, ligado como stats handler em `kernelgrpc.NewServer`, abre o span SERVER com pai extraído da metadata antes de qualquer interceptor (RF-B4).
- **Interceptors, nesta ordem:** mTLS do peer contra `GRPC_TRUSTED_CLIENTS` (quando TLS está ligado) → desfecho no span e registro de acesso `grpc call` → admissão por método → deadline obrigatório (`INVALID_ARGUMENT` antes do caso de uso) → contexto de execução (`x-correlation-id` preservado ou cunhado, `request_id` próprio como causação, `x-tenant-id` lido só de peer verificado, `idempotency-key` exigida nos comandos e levada ao caso de uso) → handler. A cadeia vale só para os métodos de `ReservationsService`: a checagem de saúde passa direto, sem admissão nem prazo obrigatório.
- **Idempotência dos comandos:** `Reserve` e `Cancel` exigem a metadata `idempotency-key`, no formato `^[A-Za-z0-9._-]{1,128}$` (FND-04 §7.6, ADR-056). Cada comando passa pela inbox do contexto, com `consumer_name` `reservations.commands`, separado das mensagens que o consumidor registra como `reservations`. Metadata ausente é `INVALID_ARGUMENT` com reason `MISSING_IDEMPOTENCY_KEY`, e fora do formato é `INVALID_IDEMPOTENCY_KEY`. A mesma chave com o mesmo pedido devolve a resposta gravada, aceite ou recusa, com o header `idempotent-replayed: true` e sem nova auditoria. Com outro pedido, é `FAILED_PRECONDITION` com `REUSED_IDEMPOTENCY_KEY`; em andamento além da espera, `ABORTED` com `IN_FLIGHT_IDEMPOTENCY_KEY`. A entrada vale 24h.
- **Desfechos:** rejeição de domínio no `oneof result`; `NOT_FOUND` (inclusive acesso a identificador de outro tenant), `ABORTED`, ausência de tenant ou permissão `PERMISSION_DENIED`, `DEADLINE_EXCEEDED` e `INTERNAL` sem detalhe para as falhas técnicas.
- **Saúde:** `NOT_SERVING` até o ping no pool e o `Migrate` opcional, `SERVING` depois, `NOT_SERVING` no shutdown; o registro `grpc listening` traz o endereço real em `server.address` e `server.port`.

## Consumo

A ponte `Sink` confirma sem inbox, com um registro em `debug`, a entrega de tipo diferente do assinado e passa o resto ao `kernelapp.Consumer`, que abre o span `process {messaging.destination.name}` CONSUMER em raiz, com link ao contexto de criação do envelope (`TRC-07`, `TRC-08`); fora da fronteira, a raiz é de classe `error`. A fronteira só é `Verified` quando o transporte prova as duas coisas — TLS **e** cliente autenticado por SASL ou certificado (ADR-052): com o opt-out de desenvolvimento (`KAFKA_INSECURE`), a fronteira fica `development-only`, nunca `Verified`. `OrdersBoundary` admite só o produtor declarado em `ORDERS_SOURCE` (`CTX-27`); a ligação real entre principal e `source` é a ACL do broker (só o principal de `orders` publica em `orders.events`), pré-requisito de todo ambiente com a fronteira verificada. O adapter reconstrói o `ExecutionContext` do envelope — tenant incluído — e o deposita no `context.Context` (ADR-049), com `correlationid` e o `id` recebido como causação, então o `ReservationConfirmed` herda a cadeia do `OrderPlaced`.

## Configuração

| Variável | Papel | Efeito |
| --- | --- | --- |
| `PG_DSN` | todos | Banco e role `reservations` (ADR-053) |
| `GRPC_ADDR` | `api` | Default `:9090` |
| `GRPC_INSECURE` ou `GRPC_TLS_CERT_FILE` + `GRPC_TLS_KEY_FILE` | `api` | Transporte; sem nenhum, exit 2 |
| `GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS` | `api`, com TLS | CA dos clientes e allowlist de identidades por URI/DNS SAN (nunca CN — ex.: `spiffe://dmpf/bff`); com TLS ligado os dois são obrigatórios (ADR-052), e o `x-tenant-id` só é lido de peer verificado |
| `MIGRATE` | `api` | Aplica o schema antes de servir |
| `KAFKA_BROKERS`, `KAFKA_INSECURE` | `relay`, `consumer` | Brokers e opt-out de TLS |
| `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD` ou `KAFKA_CLIENT_CERT_FILE` + `KAFKA_CLIENT_KEY_FILE` | papéis com Kafka, com TLS | Autenticação do cliente no broker (SCRAM-SHA-256/512 ou certificado), obrigatória sempre que `KAFKA_INSECURE` não está ligado (ADR-052); `KAFKA_CA_FILE` quando a CA do broker é privada |
| `KAFKA_RESERVATIONS_TOPIC`, `KAFKA_RESERVATIONS_DLQ`, `KAFKA_GROUP` | `relay` | Canal `reservations.events` |
| `KAFKA_ORDERS_TOPIC`, `KAFKA_ORDERS_DLQ`, `KAFKA_GROUP` | `consumer` | Canal inbound `orders.events` e grupo |
| `ORDERS_SOURCE` | `consumer` | Produtor admitido na fronteira do consumo, pelo atributo `source` do envelope (`CTX-27`); default `urn:dmpf:reference-orders`, o do relay de `orders`; a ACL do broker por principal é quem garante que só ele o produz (ADR-052) |
| `METRIC_TENANTS` | `api` | Tenants com bucket de admissão e rótulo de métrica próprios (`MET-07`), separados por vírgula; os demais compartilham `other` |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` | todos | Identidade do recurso OTel (`service.version`, `service.instance.id`, `dmpf.process.role`, `deployment.environment.name`); sem `OTEL_SERVICE_NAME`, o serviço é `reservations`, e sem `dmpf.process.role` o papel vem de `--role`; versão e instância não têm default, e sem elas a partida falha com `ErrResourceIncomplete` (exit 1). O `deploy/.env.example` não declara papel, porque vale para todos; cada `serve-<papel>` e os `docker:run-relay` e `docker:run-consumer` (no container, por `-e`) acrescentam `service.instance.id=reservations-local-<papel>,dmpf.process.role=<papel>` depois de carregar o `deploy/.env`, e a chave repetida fica com o último valor |
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
| `InboxRetention` | 192h | Idade a partir da qual a entrada de mensagem é purgada |
| `PurgeInterval`, `PurgeBatch` | 15min, 1000 | Intervalo e lote de cada purga |

Cada processo purga o que é dele: o `api` as entradas de comando vencidas, depois
do `Migrate`; o `relay` a outbox publicada; o `consumer` as entradas de mensagem.
As purgas param antes de o pool fechar. O `consumer` recusa iniciar com
`InboxRetention` menor que a janela de redelivery do canal, de 7 dias
(`INB-14`): uma entrada purgada antes disso deixaria passar como nova uma
reentrega.

## Rodar localmente

```bash
docker compose -f infra/local/docker-compose.yml --profile postgres --profile dmpf up -d postgres-init
PG_DSN='postgres://reservations:reservations-local@localhost:5432/reservations?sslmode=disable' MIGRATE=true GRPC_ADDR=127.0.0.1:9192 GRPC_INSECURE=true \
  pnpm nx run reservations:serve-api
PG_DSN='postgres://reservations:reservations-local@localhost:5432/reservations?sslmode=disable' KAFKA_BROKERS=localhost:9092 KAFKA_INSECURE=true \
  KAFKA_RESERVATIONS_TOPIC=reservations.events KAFKA_RESERVATIONS_DLQ=reservations.events.dlq KAFKA_GROUP=reservations \
  pnpm nx run reservations:serve-relay
PG_DSN='postgres://reservations:reservations-local@localhost:5432/reservations?sslmode=disable' KAFKA_BROKERS=localhost:9092 KAFKA_INSECURE=true \
  KAFKA_ORDERS_TOPIC=orders.events KAFKA_ORDERS_DLQ=orders.events.dlq KAFKA_GROUP=reservations \
  pnpm nx run reservations:serve-consumer
```

`GRPC_INSECURE=true` e `KAFKA_INSECURE=true` são o opt-out de desenvolvimento (ADR-052) — a fronteira do consumer fica `development-only`; a topologia completa via `docker compose --profile dmpf` já sobe com mTLS entre `api` e BFF e SASL no Kafka interno (`infra/README.md`).

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `test-distributed`, `govulncheck`, `serve-api`, `serve-relay` e `serve-consumer`, mais os de container: `docker:build`, `docker:run` (papel `api`), `docker:run-relay` e `docker:run-consumer`, que o `bff:docker:run` sobe.

## Testes

Unitários, sem banco: as UPRs do `domain`, as sete disposições do consumo e a sequência canônica da `application` sobre o `memory`, e o binding e os interceptors do `rpc` por `bufconn` sobre o store em memória, ciclo de saúde, tracer de banco, `Sink` (span `process` do kernel e filtro por tipo), classificação da fronteira, catálogos por papel e partida do binário.

Com a build tag `integration` e `PG_DSN`, o `test-race` cobre o `provider` (escopo de tenant e acesso cruzado inclusos), o e2e do consumer adapter e do relay no package raiz e o `appkit`; cada teste roda num banco `reservations_test_<id>` próprio, que o `tb/pg` cria e apaga no servidor de `PG_DSN`, e o `test-distributed` roda depois do `test-race`, porque usa o mesmo banco. O `test-distributed` roda só o `distkit`, com as tags `integration,distributed`, e exige Redpanda (`KAFKA_BROKERS`); é o que o `dmpf-distributed.yml` executa em pipeline próprio (`KIT-11`). A topologia inteira é provada pelo e2e do `bff`.

Os dois targets sobem a infra de testes (`testkit:test-infra-up`), e o `tools/test-env.sh` preenche `PG_DSN` e `KAFKA_BROKERS` com o Postgres (15432) e o Redpanda (19092) dela, a partir do `.env.example` da raiz:

```bash
pnpm nx run reservations:test-race
pnpm nx run reservations:test-distributed
```
