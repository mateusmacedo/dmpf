# bff

Única borda REST/JSON pública da topologia de referência do kernel DMPF (ADR-044). Um processo, sem banco, outbox nem consumer: autentica o sujeito, autoriza por permissão e traduz cada rota do contrato OpenAPI em uma chamada gRPC a um dos contextos (`RST-01`, `GRP-01`, `IDN-16`).

| Rota | Contrato | RPC | Permissão |
| --- | --- | --- | --- |
| `POST /orders/{id}/items` | `contracts/openapi/orders/v1/openapi.yaml` | `OrdersService/AddItem` | `orders:write` |
| `POST /orders/{id}/place` | idem | `OrdersService/PlaceOrder` | `orders:write` |
| `GET /orders/{id}` | idem | `OrdersService/FindOrder` | `orders:read` |
| `GET /reservations/{order_id}` | `contracts/openapi/reservations/v1/openapi.yaml` | `ReservationsService/FindReservation` | `reservations:read` |
| `POST /reservations/{order_id}/reserve` | idem | `ReservationsService/Reserve` | `reservations:write` |
| `POST /reservations/{order_id}/cancel` | idem | `ReservationsService/Cancel` | `reservations:write` |

Toda rota exige sujeito e tenant resolvidos (`RequireSubjectAndTenant`); `ValidateEdge` recusa na partida uma rota que exigisse sujeito sem declarar permissão (`IDN-16`, `IDN-17`). Criado pela `docs/specs/SPEC-ACYKBF9V-dmpf-reference-bff-contextos.md`; decisões em `docs/adr/044-bff-rest-e-contextos-grpc-de-referencia.md` e, para autenticação e tenant, na `SPEC-9B6SHEH8` (`docs/adr/049` a `052`).

## Topologia

```mermaid
flowchart LR
  client[cliente HTTP] --> bff[bff]
  bff -->|gRPC + mTLS| orders[orders api]
  bff -->|gRPC + mTLS| reservations[reservations api]
  orders --> ordersdb[(dmpf_orders)]
  reservations --> reservationsdb[(dmpf_reservations)]
```

## Unidade do manifesto

`bff/app`, bloco `app`, `bounded_context` `bff`. O BFF alcança só o contrato gerado de `company.{orders,reservations}.service.v1` — superfície pública por construção — e o shared kernel: `http`, `grpc`, `transport`, `observability`, `authn` e `ports`. Importar domínio ou aplicação dos contextos reprova no verificador com `DMPF-D002`; o `external` não declara `pgx` nem `franz-go`.

## Cadeia de uma requisição

1. `withRecover` protege contra panic; `withRouteDeadline` mede o prazo da requisição a partir do `Budget` da rota e monta o orçamento de retry — nenhum valor do chamador o alarga (`GRP-05`, `RES-24`).
2. Span de servidor com pai no `traceparent` recebido, quando houver (`TRC-02`); `X-Correlation-ID` válido é preservado, senão cunhado, e volta no mesmo header (`CTX-07`); a requisição ganha `request_id` próprio.
3. **Autenticação e autorização** (`ResolveIdentity`, `libs/backend/go/http/identity.go`): sem credencial ou credencial inválida em rota que exige sujeito ou tenant → 401 `unauthenticated`; tenant não resolvido → 403 `tenant-unresolved`; rota sem `Permission` declarada → 403 `permission-undeclared`; sujeito sem a permissão exigida → 403 `permission-denied`. Em seguida, `RefuseAssertedIdentity` recusa com 403 `identity-mismatch` quando `X-Subject-ID`, `X-Tenant-ID` ou o query `tenant_id` divergem do que a verificação resolveu (`CTX-06`) — o cliente nunca pode afirmar uma identidade diferente da autenticada.
4. O contexto de execução de nove campos (`CTX-01`) é montado — sujeito, tenant, permissões, `request_id`, `correlation_id`, `trace_id`, prazo e `locale` (primeira língua de `Accept-Language`, com `en` como default) — e depositado no `context.Context` da requisição, o único caminho até o provider (ADR-049).
5. `Idempotency-Key` obrigatório nos POST; sem ele, 400 antes de qualquer RPC (`RST-02`).
6. O handler converte JSON em Protobuf e chama o contexto por gRPC com mTLS. A metadata leva `x-correlation-id`, `x-causation-id` (o `request_id` do BFF), `x-tenant-id`, `traceparent` do span de cliente e `idempotency-key`.

Retry só em `FindOrder` e `FindReservation`, com `UNAVAILABLE` retentável; comandos não são repetidos (`GRP-08`, `GRP-09`). O prazo de cada método é menor que o da rota (`GRP-17`). O breaker de cada contexto conta só indisponibilidade (`UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`, `INTERNAL`, `UNKNOWN`, `DATA_LOSS` e erro de transporte): um `NOT_FOUND` repetido não o abre (`RES-10`, `RES-12`). A admissão (RES-16/17) roda depois do contexto e é chaveada pelo tenant que ele resolveu, não pela requisição bruta.

| Resultado do contexto | HTTP | `code` |
| --- | --- | --- |
| `Rejection` no `oneof result` | 422 | código de domínio |
| `NOT_FOUND` (inclusive identificador de outro tenant) | 404 | `not-found` |
| `ABORTED` | 409 | `version-conflict` |
| `DEADLINE_EXCEEDED` ou prazo esgotado no BFF | 504 | `deadline-exceeded` |
| `UNAVAILABLE` | 503 | `unavailable` |
| `RESOURCE_EXHAUSTED` | 429 | — |
| demais | 500 | `internal-failure` |

## Autenticação

O BFF é a única borda que resolve identidade (`ResolveIdentity`, pacote `authn` do kernel): um verificador OIDC (`DMPF_OIDC_*`) ou o mock de desenvolvimento (`DMPF_AUTH_DEV_MOCK`), nunca os dois ao mesmo tempo. O sujeito e as permissões não atravessam o fan-out para `orders` e `reservations` (`CTX-12`) — o que chega a cada contexto é o tenant, verificado por mTLS na origem (ver `apps/backend/orders/README.md` e `apps/backend/reservations/README.md`).

## Configuração

| Variável | Obrigatória | Efeito |
| --- | --- | --- |
| `DMPF_ORDERS_GRPC_TARGET`, `DMPF_RESERVATIONS_GRPC_TARGET` | sim | Alvos gRPC dos contextos (`dns:///host:porta`) |
| `DMPF_GRPC_INSECURE` | uma das duas | `true` só em desenvolvimento |
| `DMPF_GRPC_CA_FILE`, `DMPF_GRPC_SERVER_NAME` | uma das duas | CA que valida os contextos e nome esperado no certificado |
| `DMPF_GRPC_CLIENT_CERT_FILE`, `DMPF_GRPC_CLIENT_KEY_FILE` | com `DMPF_GRPC_CA_FILE` | Certificado de cliente do BFF (URI `spiffe://dmpf/bff`): os contextos só servem workload verificado (ADR-052) |
| `DMPF_OIDC_ISSUER`, `DMPF_OIDC_AUDIENCE`, `DMPF_OIDC_TENANT_CLAIM` | uma das duas linhas de autenticação | Emissor, audiência e claim de tenant do access token |
| `DMPF_OIDC_PERMISSION_CLAIMS` | não | Caminhos das claims de permissão, separados por vírgula; default `scope,realm_access.roles` (formato Keycloak) |
| `DMPF_OIDC_DISCOVERY_TIMEOUT_SECONDS` | não | Default 10 |
| `DMPF_AUTH_DEV_MOCK` | uma das duas linhas de autenticação | `true` lê a identidade declarada no próprio Bearer, sem verificação — só desenvolvimento; não pode coexistir com `DMPF_OIDC_ISSUER` |
| `DMPF_HTTP_ADDR` | não | Default `:8080`; `127.0.0.1:0` escolhe porta livre e o log `http listening` traz o endereço |
| `DMPF_CORS_ORIGINS` | não | Origens aceitas pelo navegador (Swagger UI local) |
| `DMPF_METRIC_TENANTS` | não | Tenants que têm bucket de admissão e rótulo de métrica próprios (`MET-07`), separados por vírgula; os demais compartilham `other` |
| `DMPF_OPENAPI_ORDERS_PATH`, `DMPF_OPENAPI_RESERVATIONS_PATH` | não | Servem os contratos em `/openapi/<ctx>/v1/openapi.yaml` |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE` | não | Exportação OTLP; sem endpoint, telemetria em memória |
| `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | não | Identidade do recurso OTel |

Variável obrigatória ausente, ou nenhuma política de transporte gRPC ou de autenticação, encerra a partida com exit 2 nomeando o que falta.

## Rodar localmente

Com os dois contextos no ar (ver os README deles):

```bash
DMPF_ORDERS_GRPC_TARGET=dns:///localhost:9090 DMPF_RESERVATIONS_GRPC_TARGET=dns:///localhost:9091 \
  DMPF_GRPC_INSECURE=true DMPF_AUTH_DEV_MOCK=true pnpm nx run bff:serve
```

A topologia inteira sobe por `docker compose -f infra/local/docker-compose.yml --profile dmpf up -d --build`, com mTLS entre o BFF e os `api` e SASL no Kafka interno (ver `infra/README.md`).

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `govulncheck` e `serve`, mais os de infraestrutura do workspace: `infra-up`, `observability-up`, `infra-down`, `infra-budget` e `k8s-render`.

## Testes

- Unitários: rotas e contrato (inclusive o teste estrutural dos dois OpenAPI), resolução e recusa de identidade (credencial ausente, expirada, asserção divergente via `X-Subject-ID`/`X-Tenant-ID`/`tenant_id`), mapeamento de status, clientes gRPC contra servidores falsos por `bufconn` (retry por idempotência, prazo decrescente, metadata, mTLS e hierarquia de spans) e partida do binário.
- E2e caixa-preta (build tag `integration`): compila os três binários com `-race`, cria dois bancos e quatro tópicos por execução, sobe os seis processos e fala só HTTP com o BFF. Prova a cadeia de contexto até `ReservationConfirmed`, o cancelamento que vence um `OrderPlaced` posterior, a reentrega que termina em `DuplicateIgnored`, uma outbox por contexto e a recusa de uma chamada sem credencial. Exige `DMPF_PG_DSN` (usuário com `CREATE DATABASE`) e `DMPF_KAFKA_BROKERS`.

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 \
  pnpm nx run bff:test-race
```
