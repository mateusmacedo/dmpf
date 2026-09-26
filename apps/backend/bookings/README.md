# bookings

Bounded context `resource-scheduling`, o golden do harness de bounded contexts
(`docs/specs/SPEC-AHPRBZCT-bookings.md`, ADR-045, ADR-048): um módulo Go, um
package por bloco, com composition root completa — próprio da sua natureza de
golden, que precisa exercitar tudo o que um contexto faz (ADR-048). Vive em
`apps/backend/bookings` porque um contexto de negócio é uma app, não uma lib de
reuso — em `libs/backend/go` fica só o kernel (ADR-046).

Como `orders` e `reservations`, `bookings` serve gRPC interno
(`company.bookings.service.v1.BookingsService`, em `app/rpc`) e só o `bff`
expõe as rotas REST dele. Tenant, autorização por permissão e confiança de
canal vêm da `SPEC-9B6SHEH8` (ADR-049 a ADR-052).

Projeto Nx `bookings`, tags `type:app`, `scope:backend`, `stack:go` e
`layer:apps` — a camada é a do estágio mais alto entre os blocos, porque é o
primeiro em que toda dependência do módulo está de pé. Import path do módulo:
`github.com/mateusmacedo/dmpf/apps/backend/bookings`.

| Papel | O que faz | Blocos cabeados |
| --- | --- | --- |
| `api` | Serve o `BookingsService` por gRPC | `app/rpc` (cadeia de interceptors do kernel: span, admissão, prazo, contexto) → `application` (autorização, UPRs) → `provider` (repositórios, reader, escopados por tenant) sobre `postgres` do kernel (UoW, outbox) |
| `relay` | Drena a outbox para `bookings.events`, autenticado no broker | `app/relay` → `postgres` (claim) + `kafka` (publisher) |

## Blocos e unidades

| Package | Bloco | Unidade | Conteúdo |
| --- | --- | --- | --- |
| `domain` | `domain` | `resource-scheduling/domain` | Agregados `Booking` e `Resource`, três UPRs, desfecho em tipos do kernel |
| `ports` | `port` | `resource-scheduling/ports` | `BookingsByResourceReader`, a consulta por relação que os genéricos do kernel não expressam |
| `application` | `application` | `resource-scheduling/application` | `Service` com `Reserve`, `Cancel` e `Register` na sequência canônica; cada operação declara a permissão que exige (`app/authorization.go`); `Destination = "bookings.events"` |
| `provider` | `provider` | `resource-scheduling/provider-postgres` | Repositórios escopados por `tenant_id` via `postgres.Table` (ADR-051), mapeadores e `schema.sql` sobre `pgx` |
| `app`, `app/rpc`, `cmd` | `app` | `resource-scheduling/app` | Composition root: `config.go`, `catalog.go`, `wiring.go`, `telemetry.go`, borda gRPC (`app/rpc`) e binário (`cmd/main.go`) |

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

## Serviço gRPC

| Método | Rota REST no `bff` | Permissão |
| --- | --- | --- |
| `ReserveBooking` | `POST /bookings/booking` | `bookings:write` |
| `CancelBooking` | `POST /bookings/booking/{id}/cancel` | `bookings:write` |
| `RegisterResource` | `POST /bookings/resource` | `bookings:write` |
| `FindBooking` | `GET /bookings/booking/{id}` | `bookings:read` |
| `FindBookingsByResource` | `GET /bookings/booking?resourceId=` | `bookings:read` |

O contrato REST publicado é `contracts/openapi/bookings/v1/openapi.yaml`, servido
pelo `bff`. Uma recusa de domínio volta como `rejection` na resposta; uma falha
técnica é um status gRPC (`NOT_FOUND`, `ABORTED` para conflito de versão,
`INVALID_ARGUMENT` para entrada malformada).

## Canal, autorização e tenant

O `api` só serve o workload que a CA local assinou e a allowlist nomeia
(`GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS`, `IDN-03`). A cadeia do kernel
reconstrói o contexto de execução a partir da metadata que o `bff` propaga —
tenant, `correlation_id`, `causation_id`, `locale` e prazo — e o deposita no
`context.Context`, o único caminho até o provider (ADR-049). Sujeito e
permissões não atravessam o fan-out (`CTX-12`).

`Reserve`, `Cancel` e `Register` declaram `bookings:write`; `FindBooking` e
`FindBookingByResource` declaram `bookings:read` (`app/authorization.go`). Toda
leitura e escrita das tabelas de agregado é escopada por `tenant_id` no choke
point `postgres.Table` (ADR-050, ADR-051) — inclusive `FindBookingByResource`,
a consulta por relação que `ports.BookingsByResourceReader` declara. Uma chamada
sem tenant é recusada, e um identificador que existe para outro tenant responde
`NOT_FOUND` e vira evento de segurança auditado (`IDN-12`, `IDN-13`, `IDN-15`).

## Configuração

| Variável | Papel | Efeito |
| --- | --- | --- |
| `PG_DSN` | todos | Banco do contexto |
| `GRPC_ADDR` | `api` | Default `:9090` |
| `GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE`, `GRPC_CLIENT_CA_FILE`, `GRPC_TRUSTED_CLIENTS` | `api`, com TLS | Certificado do servidor e autenticação do chamador por mTLS (`IDN-03`) |
| `GRPC_INSECURE` | `api` | `true` dispensa o TLS — só desenvolvimento |
| `METRIC_TENANTS` | `api` | Tenants com balde de admissão e rótulo próprios (`MET-07`) |
| `MIGRATE` | `api` | Aplica o schema antes de servir |
| `KAFKA_BROKERS`, `KAFKA_INSECURE` | `relay` | Brokers e opt-out de TLS |
| `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD` ou `KAFKA_CLIENT_CERT_FILE` + `KAFKA_CLIENT_KEY_FILE` | `relay`, com TLS | Autenticação do cliente no broker, obrigatória sempre que `KAFKA_INSECURE` não está ligado (ADR-052) |
| `KAFKA_BOOKINGS_TOPIC`, `KAFKA_BOOKINGS_DLQ`, `KAFKA_GROUP` | `relay` | Endereço, contenção e grupo do canal `bookings.events` |
| `OTLP_ENDPOINT`, `OTLP_INSECURE` | não | Exportação OTLP; sem endpoint, telemetria em memória |
| `SERVICE`, `SERVICE_VERSION`, `INSTANCE_ID` | todos | Identidade do recurso OTel; default de `SERVICE` é `bookings` |

Variável obrigatória ausente, ou `api` sem política de transporte gRPC,
encerra a partida com exit 2 nomeando o que falta.

## Rodar localmente

Com o Postgres do compose, onde o `postgres-init` cria o banco e o role
`bookings` (profile `dmpf`, ver `infra/README.md`):

```bash
PG_DSN='postgres://bookings:bookings-local@localhost:5432/bookings?sslmode=disable' MIGRATE=true GRPC_INSECURE=true \
  pnpm nx run bookings:serve-api
PG_DSN='postgres://bookings:bookings-local@localhost:5432/bookings?sslmode=disable' KAFKA_BROKERS=localhost:9092 KAFKA_INSECURE=true \
  KAFKA_BOOKINGS_TOPIC=bookings.events KAFKA_BOOKINGS_DLQ=bookings.events.dlq KAFKA_GROUP=bookings \
  pnpm nx run bookings:serve-relay
```

A imagem nasce do `Dockerfile` na raiz do repositório:
`docker build -f apps/backend/bookings/Dockerfile -t bookings:local .`

## Targets Nx

`fmt-check`, `vet`, `build`, `test-race`, `test-distributed`, `govulncheck`,
`serve-api` e `serve-relay`.

## Testes

Unitários, sem banco: as UPRs do `domain`, a sequência canônica e a autorização
por permissão da `application` sobre o `memory`, e o serviço gRPC do `app/rpc`
por `bufconn` sobre o store em memória. `test-race` roda com
`-tags=integration` e `PG_DSN` apontando o servidor: a suíte usa o próprio banco
`bookings_test`. Prova o `provider` (escopo de tenant e acesso cruzado
inclusos), o `appkit` e o e2e gRPC até a outbox, pelo mesmo salto que o `bff`
cruza.
`test-distributed` roda o `distkit` (dois processos sobre Redpanda) com as tags
`integration,distributed`.

```bash
PG_DSN='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' pnpm nx run bookings:test-race
```
