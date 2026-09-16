# bff

Única borda REST/JSON pública da topologia de referência do kernel DMPF (ADR-044). Um processo, sem banco, outbox nem consumer: traduz cada rota do contrato OpenAPI em uma chamada gRPC a um dos contextos (`RST-01`, `GRP-01`).

| Rota | Contrato | RPC |
| --- | --- | --- |
| `POST /orders/{id}/items` | `contracts/openapi/orders/v1/openapi.yaml` | `OrdersService/AddItem` |
| `POST /orders/{id}/place` | idem | `OrdersService/PlaceOrder` |
| `GET /orders/{id}` | idem | `OrdersService/FindOrder` |
| `GET /reservations/{order_id}` | `contracts/openapi/reservations/v1/openapi.yaml` | `ReservationsService/FindReservation` |
| `POST /reservations/{order_id}/reserve` | idem | `ReservationsService/Reserve` |
| `POST /reservations/{order_id}/cancel` | idem | `ReservationsService/Cancel` |

Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`; decisões em `docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md`.

## Topologia

```mermaid
flowchart LR
  client[cliente HTTP] --> bff[bff]
  bff -->|gRPC| orders[orders api]
  bff -->|gRPC| reservations[reservations api]
  orders --> ordersdb[(dmpf_orders)]
  reservations --> reservationsdb[(dmpf_reservations)]
```

## Unidade do manifesto

`bff/app`, bloco `app`, `bounded_context` `bff`. O BFF alcança só o contrato gerado de `company.{orders,reservations}.service.v1` — superfície pública por construção — e o shared kernel: `http`, `grpc`, `transport`, `observability` e `ports`. Importar domínio ou aplicação dos exemplos reprova no verificador com `DMPF-D002`; o `external` não declara `pgx` nem `franz-go`.

## Cadeia de uma requisição

1. `http.Admission` decide antes de ler o corpo (`RES-17`); recusa com 429.
2. Span de servidor com pai no `traceparent` recebido, quando houver (`TRC-02`); `X-Correlation-ID` válido é preservado, senão cunhado, e volta no mesmo header (`CTX-07`); a requisição ganha `request_id` próprio.
3. Prazo da requisição derivado do `Budget` da rota: nenhuma chamada sai sem deadline (`GRP-04`).
4. `Idempotency-Key` obrigatório nos POST; sem ele, 400 antes de qualquer RPC (`RST-02`).
5. O handler converte JSON em Protobuf e chama o contexto. A metadata leva `x-correlation-id`, `x-causation-id` (o `request_id` do BFF), `traceparent` do span de cliente e `idempotency-key`.

Retry só em `FindOrder` e `FindReservation`, com `UNAVAILABLE` retentável; comandos não são repetidos (`GRP-08`, `GRP-09`). O prazo de cada método é menor que o da rota (`GRP-17`). O breaker de cada contexto conta só indisponibilidade (`UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`, `INTERNAL`, `UNKNOWN`, `DATA_LOSS` e erro de transporte): um `NOT_FOUND` repetido não o abre (`RES-10`, `RES-12`).

| Resultado do contexto | HTTP | `code` |
| --- | --- | --- |
| `Rejection` no `oneof result` | 422 | código de domínio |
| `NOT_FOUND` | 404 | `not-found` |
| `ABORTED` | 409 | `version-conflict` |
| `DEADLINE_EXCEEDED` ou prazo esgotado no BFF | 504 | `deadline-exceeded` |
| `UNAVAILABLE` | 503 | `unavailable` |
| `RESOURCE_EXHAUSTED` | 429 | — |
| demais | 500 | `internal-failure` |

## Configuração

| Variável | Obrigatória | Efeito |
| --- | --- | --- |
| `DMPF_ORDERS_GRPC_TARGET`, `DMPF_RESERVATIONS_GRPC_TARGET` | sim | Alvos gRPC dos contextos (`dns:///host:porta`) |
| `DMPF_GRPC_INSECURE` | uma das duas | `true` só em desenvolvimento |
| `DMPF_GRPC_CA_FILE`, `DMPF_GRPC_SERVER_NAME` | uma das duas | CA que valida os contextos e nome esperado no certificado |
| `DMPF_HTTP_ADDR` | não | Default `:8080`; `127.0.0.1:0` escolhe porta livre e o log `http listening` traz o endereço |
| `DMPF_CORS_ORIGINS` | não | Origens aceitas pelo navegador (Swagger UI local) |
| `DMPF_OPENAPI_ORDERS_PATH`, `DMPF_OPENAPI_RESERVATIONS_PATH` | não | Servem os contratos em `/openapi/<ctx>/v1/openapi.yaml` |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE` | não | Exportação OTLP; sem endpoint, telemetria em memória |
| `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | não | Identidade do recurso OTel |

Variável obrigatória ausente, ou nenhuma política de transporte, encerra a partida com exit 2 nomeando o que falta.

## Rodar localmente

Com os dois contextos no ar (ver os README deles):

```bash
DMPF_ORDERS_GRPC_TARGET=dns:///localhost:9090 DMPF_RESERVATIONS_GRPC_TARGET=dns:///localhost:9091 \
  DMPF_GRPC_INSECURE=true pnpm nx run bff:serve
```

A topologia inteira sobe por `docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build` (ver `infra/README.md`).

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck` e `serve`, mais os de infraestrutura do workspace: `infra-up`, `observability-up`, `infra-down`, `infra-budget` e `k8s-render`.

## Testes

- Unitários: rotas e contrato (inclusive o teste estrutural dos dois OpenAPI), mapeamento de status, clientes gRPC contra servidores falsos por `bufconn` (retry por idempotência, prazo decrescente, metadata e hierarquia de spans) e partida do binário.
- E2e caixa-preta (build tag `integration`): compila os três binários com `-race`, cria dois bancos e quatro tópicos por execução, sobe os seis processos e fala só HTTP com o BFF. Prova a cadeia de contexto até `ReservationConfirmed`, o cancelamento que vence um `OrderPlaced` posterior, a reentrega que termina em `DuplicateIgnored` e uma outbox por contexto. Exige `DMPF_PG_DSN` (usuário com `CREATE DATABASE`) e `DMPF_KAFKA_BROKERS`.

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 \
  pnpm nx run bff:test-race
```
