# bookings

Bounded context `resource-scheduling`, o golden do harness de bounded contexts
(`docs/specs/SPEC-AHPRBZCT-bookings.md`, ADR-045, ADR-048): um módulo Go, um
package por bloco, com composition root completa — próprio da sua natureza de
golden, que precisa exercitar tudo o que um contexto faz (ADR-048). Vive em
`apps/backend/bookings` porque um contexto de negócio é uma app, não uma lib de
reuso — em `libs/backend/go` fica só o kernel (ADR-046).

Diferente de `orders` e `reservations`, `bookings` não tem gRPC: a própria
borda é HTTP pública (`app/http`, package `httpedge`), com autenticação OIDC
própria — não passa pelo BFF. Autenticação, tenant e autorização por permissão
vêm da `SPEC-9B6SHEH8` (ADR-049 a ADR-052).

Projeto Nx `bookings`, tags `type:app`, `scope:backend`, `stack:go` e
`layer:apps` — a camada é a do estágio mais alto entre os blocos, porque é o
primeiro em que toda dependência do módulo está de pé. Import path do módulo:
`github.com/mateusmacedo/dmpf/apps/backend/bookings`.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve as cinco rotas REST sob `/bookings/...` | `httpedge` (servidor, autenticação, admissão) → `application` (autorização, UPRs) → `provider` (repositórios, reader, escopados por tenant) sobre `postgres` do kernel (UoW, outbox) |
| `relay` | Drena a outbox para `bookings.events`, autenticado no broker | `app/relay` → `postgres` (claim) + `kafka` (publisher) |

## Blocos e unidades

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `resource-scheduling/domain` | Agregados `Booking` e `Resource`, três UPRs, desfecho em tipos do kernel |
| `ports` | `port` | `resource-scheduling/ports` | `BookingsByResourceReader`, a consulta por relação que os genéricos do kernel não expressam |
| `application` | `application` | `resource-scheduling/application` | `Service` com `Reserve`, `Cancel` e `Register` na sequência canônica; cada operação declara a permissão que exige (`app/authorization.go`); `Destination = "bookings.events"` |
| `provider` | `provider` | `resource-scheduling/provider-postgres` | Repositórios escopados por `tenant_id` via `postgres.Table` (ADR-051), mapeadores e `schema.sql` sobre `pgx` |
| `app`, `app/http`, `cmd` | `app` | `resource-scheduling/app` | Composition root: `config.go`, `catalog.go`, `wiring.go`, `telemetry.go`, borda HTTP (`httpedge`) e binário (`cmd/main.go`) |

A sexta unidade do contexto, `resource-scheduling/contract`, vive no manifesto
de `libs/backend/go/contracts`, porque o código gerado do Protobuf mora lá. Os
harnesses `appkit` (borda a borda sobre Postgres) e `distkit` (dois processos
sobre Redpanda) completam o módulo — todo contexto tem os dois (ADR-048).

`block` e `bounded_context` vêm do manifesto (`dmpf-units.json`), nunca do nome
do diretório (ADR-012).

## Aliases de import

Os packages do kernel `domain`, `application` e `ports` têm o mesmo nome dos
deste módulo. Onde um arquivo importa os dois, o import do kernel recebe alias
pelo papel: `kernel` para o domínio, `usecase` para a aplicação, `port` para as
portas. Os packages do contexto ficam bare.

## Rotas

| Rota | Contrato | Permissão |
| --- | --- | --- |
| `POST /bookings/booking` | `contracts/openapi/bookings/v1/openapi.yaml` | `bookings:write` |
| `POST /bookings/booking/{id}/cancel` | idem | `bookings:write` |
| `POST /bookings/resource` | idem | `bookings:write` |
| `GET /bookings/booking/{id}` | idem | `bookings:read` |
| `GET /bookings/booking` (por `resource_id`) | idem | `bookings:read` |

`Mux` valida cada rota na partida (`ValidateEdge`): uma rota que exigisse
sujeito sem declarar permissão recusaria o processo antes de servir (`IDN-16`,
`IDN-17`). Toda rota de escrita exige `Idempotency-Key`.

## Autenticação, autorização e tenant

O próprio `httpedge` autentica: um verificador OIDC (`DMPF_OIDC_*`) ou o mock
de desenvolvimento (`DMPF_AUTH_DEV_MOCK`), nunca os dois ao mesmo tempo — a
mesma configuração (`authn.Config`) que o `bff` usa. O contexto de execução de
nove campos (`CTX-01`) é montado por requisição — sujeito, tenant, permissões,
`request_id`, `correlation_id`, prazo e o `locale` fixo `en`, porque a borda não
negocia conteúdo — e depositado no `context.Context`, o único caminho até o
provider (ADR-049).

`Reserve`, `Cancel` e `Register` exigem `bookings:write`; `FindBooking` e
`FindBookingByResource` exigem `bookings:read` (`app/authorization.go`). A
checagem (`usecase.Permitted`, ADR-052) nega sempre que o tenant não foi
resolvido, e sempre que o sujeito não tem a permissão declarada da rota. Toda
leitura e escrita das tabelas de agregado é escopada por `tenant_id` no choke
point `postgres.Table` (ADR-050, ADR-051) — inclusive `FindBookingByResource`,
a consulta por relação que `ports.BookingsByResourceReader` declara. Um
identificador que existe para outro tenant responde `NOT_FOUND` e vira evento
de segurança auditado, nunca `PermissionDenied` (`IDN-12`, `IDN-13`).

## Configuração

| Variável | Papel | Efeito |
| --- | --- | --- |
| `DMPF_PG_DSN` | todos | Banco do contexto |
| `DMPF_HTTP_ADDR` | `api` | Default `:8080` |
| `DMPF_OIDC_ISSUER`, `DMPF_OIDC_AUDIENCE`, `DMPF_OIDC_TENANT_CLAIM` | `api`, uma das duas linhas de autenticação | Emissor, audiência e claim de tenant do access token |
| `DMPF_OIDC_PERMISSION_CLAIMS` | `api` | Caminhos das claims de permissão, separados por vírgula; default `scope,realm_access.roles` |
| `DMPF_OIDC_DISCOVERY_TIMEOUT_SECONDS` | `api` | Default 10 |
| `DMPF_AUTH_DEV_MOCK` | `api`, uma das duas linhas de autenticação | `true` lê a identidade declarada no próprio Bearer, sem verificação — só desenvolvimento; não pode coexistir com `DMPF_OIDC_ISSUER` |
| `DMPF_MIGRATE` | `api` | Aplica o schema antes de servir |
| `DMPF_KAFKA_BROKERS`, `DMPF_KAFKA_INSECURE` | `relay` | Brokers e opt-out de TLS |
| `DMPF_KAFKA_SASL_MECHANISM`, `DMPF_KAFKA_SASL_USERNAME`, `DMPF_KAFKA_SASL_PASSWORD` ou `DMPF_KAFKA_CLIENT_CERT_FILE` + `DMPF_KAFKA_CLIENT_KEY_FILE` | `relay`, com TLS | Autenticação do cliente no broker, obrigatória sempre que `DMPF_KAFKA_INSECURE` não está ligado (ADR-052) |
| `DMPF_KAFKA_BOOKINGS_TOPIC`, `DMPF_KAFKA_BOOKINGS_DLQ`, `DMPF_KAFKA_GROUP` | `relay` | Endereço, contenção e grupo do canal `bookings.events` |
| `DMPF_OTLP_ENDPOINT`, `DMPF_OTLP_INSECURE` | não | Exportação OTLP; sem endpoint, telemetria em memória |
| `DMPF_SERVICE`, `DMPF_SERVICE_VERSION`, `DMPF_INSTANCE_ID` | todos | Identidade do recurso OTel; default de `DMPF_SERVICE` é `bookings` |

Variável obrigatória ausente, ou `api` sem nenhuma política de autenticação,
encerra a partida com exit 2 nomeando o que falta.

## Rodar localmente

`bookings` não faz parte da topologia local (`docker compose --profile dmpf`),
que cobre só `bff`, `orders` e `reservations` (ver `infra/README.md`); sobe
isolado contra o banco `app` do profile `postgres`, o mesmo que `test-race` usa:

```bash
docker compose -f infra/local/docker-compose.yml --profile postgres up -d
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_MIGRATE=true DMPF_AUTH_DEV_MOCK=true \
  pnpm nx run bookings:serve-api
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' DMPF_KAFKA_BROKERS=localhost:9092 DMPF_KAFKA_INSECURE=true \
  DMPF_KAFKA_BOOKINGS_TOPIC=bookings.events DMPF_KAFKA_BOOKINGS_DLQ=bookings.events.dlq DMPF_KAFKA_GROUP=bookings \
  pnpm nx run bookings:serve-relay
```

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `test-distributed`, `govulncheck`,
`serve-api` e `serve-relay`.

## Testes

Unitários, sem banco: as UPRs do `domain`, a sequência canônica e a autorização
por permissão da `application` sobre o `memory`, e o binding, a autenticação, a
recusa de identidade asserida e o ciclo de vida HTTP do `app/http` sobre o
store em memória. `test-race` roda com `-tags=integration` e `DMPF_PG_DSN`, e
declara `dependsOn` sobre o `test-race` do `postgres`: os dois harnesses
truncam as mesmas tabelas do mesmo banco. Prova o `provider` (escopo de tenant
e acesso cruzado inclusos), o `appkit` e o e2e HTTP até a outbox — inclusive a
validação das rotas na partida e a consulta por relação entre tenants.
`test-distributed` roda o `distkit` (dois processos sobre Redpanda) com as tags
`integration,distributed`.

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run bookings:test-race
```
