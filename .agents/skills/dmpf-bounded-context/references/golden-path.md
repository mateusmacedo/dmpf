# Golden path — passo a passo com norma e molde

Cada passo diz **o que** produzir, **qual norma** o rege e **qual arquivo**
exemplifica a forma. O molde é o `bookings`, golden da forma canônica
(ADR-053); o `reservations` só entra no que o contexto consome. `<name>` é o
identificador Go do contexto (ex.: `bookings`); `<ctx>` é o `bounded_context`
declarado (ex.: `resource-scheduling`).

## 1. Validar a spec

- Dez seções obrigatórias: Identidade, Agregados, Comandos (UPRs), Eventos de
  domínio, Consultas, Relações entre agregados, Integração, Políticas
  transversais, Critérios de aceite, Escopo fora. `stage` ∈ {`planning`,
  `building`}; na regeneração (`/dmpf-new-context <id> --regen`), também `done`.
- Norma: o template ([`template-bounded-context.md`](./template-bounded-context.md)).
- Recusa: nomear a primeira seção ausente; não escrever nada.

## 2. Esqueleto

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> --boundedContext <ctx>
```

- Produz o módulo `apps/backend/<name>` com `project.json` (tags 3D com
  `type:app` + `layer:*` do bloco mais alto; `test-race` sem `dependsOn`, porque
  cada projeto testa no seu banco), `go.mod` sem `require`, `package.json`,
  `dmpf-units.json` com uma unidade por bloco, `README.md`, `Dockerfile` e um
  `doc.go` por bloco. O `app` nasce na forma canônica: `config.go`,
  `wiring.go`, `telemetry.go`, `catalog.go`, `app/rpc/{errors,service}.go`; o
  `provider` recebe `schema.sql` e `schema.go`; o `appkit` recebe `pool.go`.
  Acrescenta um `use` ao `go.work` (ADR-045). Não há `pnpm install`.
- `--service-name` define o nome qualificado do serviço gRPC; sem ele, vira
  `company.<name>.service.v1.<Name>Service`.
- Norma: ADR-012 (classificação declarada), ADR-053; `tools/dmpf-plugin/README.md`.
- O esqueleto é o ponto de partida: preencha os arquivos gerados, não os
  recrie. `project.json`, `go.mod`, `go.work` e `dmpf-units.json` não se
  editam à mão, salvo o `include` por merge no passo 9.

## 3. `domain` — por agregado

| Peça | Molde |
| --- | --- |
| Agregado, `Snapshot`, `From*Snapshot`, `Equal`, `clone` | `apps/backend/bookings/domain/booking.go` |
| UPR de criação | `bookings/domain/reserve.go` |
| UPR sobre agregado existente | `bookings/domain/cancel.go` |
| Agregado com chave natural e `initializesOnNotFound` | `bookings/domain/resource.go` |
| Comando, resposta e evento (`EventName()` = `<name>.<evento-kebab>`) | `bookings/domain/messages.go` |
| Rejeições `Code<Agregado><Motivo>` = `<ctx>/<agregado>/<rejeicao>` | `bookings/domain/rejections.go` |
| Testes: por pré-condição, por efeito, determinismo, snapshot, projeção e códigos | `bookings/domain/{reserve_test,cancel_test,determinism_test,snapshot_test,projection_test,rejections_test}.go` |

- Norma: ADR-032 (desfecho `(Accepted[R], *Rejection)`, tipo concreto);
  `.golangci.yml` `depguard`/`forbidigo` do bloco `domain` (sem `time`,
  `errors.New`, `fmt.Errorf`, `panic`, `fmt.Print*`).
- O instante chega por parâmetro (`At`), inteiro de nanossegundos (ADR-053).
- Nomes Go canônicos: evento no passado sem sufixo (`BookingCancelled`),
  status curto (`Reserved`, `Cancelled`); o valor publicado não muda com o
  nome Go.

## 4. `port`

- Um `Repository` por agregado, `Outbox()`, e `Inbox()` só se consome; `Reader`
  com um método por consulta, `Find<Agregado>By<Campos>`.
- Molde: `libs/backend/go/ports/{repository,uow,outbox,inbox}.go`. O
  módulo `ports` é do kernel e não se duplica — o package `<name>/ports`
  declara só as portas do contexto, sobre os tipos do kernel; onde importa os
  dois, o kernel recebe o alias `port`.
- Norma: ADR-034 (`UnitOfWork[R]`, `bind` no composition root).

## 5. `application`

| Peça | Molde |
| --- | --- |
| `service.go`: `AggregateType`, `Destination`, `Resources`, `Operation*`, `enqueueAll` | `apps/backend/bookings/application/service.go` |
| Caso de uso de criação (nove passos, ramo `creates`) | `bookings/application/reserve_booking.go` |
| Caso de uso sobre existente | `bookings/application/cancel_booking.go` |
| Consulta fora da UoW | `bookings/application/find_booking.go`, `find_booking_by_resource.go` |
| Caso de uso de consumo (sete disposições) — só se consome | `apps/backend/reservations/application/consume.go` |
| Realização em memória para teste (`memory.Table[ID, S]` por agregado) | `libs/backend/go/memory/{tx,store,inbox,clock,errors}.go` |
| Testes de sequência e instrumentação | `bookings/application/{sequence_test,instrumentation_test,doubles_test}.go` |

- Norma: FND-04 §3.2 (a sequência canônica), §6.4 (disposições); ADR-035
  (evento na mesma transação do estado).
- Aliases do kernel: `kernel` (domain), `usecase` (application), `port` (ports).

## 6. `provider-postgres`

| Peça | Molde |
| --- | --- |
| `schema.sql`: agregado no plural, `tenant_id` à frente da chave, `<agregado>_id`, `version`, `snapshot jsonb`, coluna tipada só para o que uma consulta filtra | `apps/backend/bookings/provider/schema.sql` |
| `schema.go`: o DDL embutido para o composition root migrar | `bookings/provider/schema.go` |
| Repositório: struct de estado privado com tags JSON estáveis, optimistic locking | `bookings/provider/booking_repository.go` |
| Mapper evento → payload do contrato (instante → `google.protobuf.Timestamp` por `time.Unix(0, ns)`) | `bookings/provider/mapper.go` |
| `Reader`, um por consulta | `bookings/provider/booking_reader.go` |
| Testes de repositório, concorrência, leitura e e2e (build tag `integration`) | `bookings/provider/{repository_test,concurrency_test,reader_by_resource_test,e2e_test}.go` |

- Nomes: tabela sem prefixo, índice `<tabela>_<colunas>_idx`, constraint
  `<tabela>_<colunas>_{pkey,key,check,fkey}`; `outbox`, `inbox` e `quarantine`
  são do kernel (ADR-053). O `dmpf-context-check.sh` reprova o que fugir disso.
- Banco de teste: `appkit.OpenPool(t)` abre `<name>_test` no servidor de
  `PG_DSN`, migra as capacidades pedidas e o schema do contexto e trunca as
  tabelas de `Tables` (`libs/backend/go/testkit/tb/pg/pool.go`).

## 7. `app`

| Peça | Molde |
| --- | --- |
| Configuração: `Defaults(role)`, `FromEnv`, `Validate`, variáveis sem `DMPF_` | `apps/backend/bookings/app/config.go` |
| Composition root: `Run`, `RunWith`, serviço de aplicação, `serveAPI` gRPC, migrate no ready, relay | `bookings/app/wiring.go` |
| Serviço gRPC: `ServiceDesc` com um `unary` por método, `Methods()`, `Server` sobre o serviço de aplicação | `bookings/app/rpc/service.go` |
| Mapeamento de erro para status gRPC | `bookings/app/rpc/errors.go` |
| Catálogo do canal que o relay drena | `bookings/app/catalog.go` |
| Autorização e instrumentação | `bookings/app/{authorization,telemetry}.go` |
| e2e gRPC por bufconn sobre Postgres | `bookings/app/e2e_test.go` |
| Consumer adapter (`envelope.Unpack`) — **só se o contexto consome** | `apps/backend/reservations/app/consumer.go` |

- Norma: ADR-044 (REST só no `bff`, contextos atrás de gRPC), ADR-053; FND-08
  (as três posições de observabilidade). A cadeia de interceptors é
  `kernelgrpc.ServerInterceptors`; limites por método por
  `kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), ...)`.
- A borda REST do contexto é do `bff` (`apps/backend/bff/app/api/routes.go`,
  `handlers_bookings.go`, `app/rpc/clients.go`), em tarefa própria. Contexto
  com `app/http` reprova no `dmpf-context-check.sh`.

## 8. Contrato

1. O serviço em
   `contracts/proto/company/<name>/service/v1/<name>_service.proto`. Molde:
   `contracts/proto/company/bookings/service/v1/bookings_service.proto`.
2. Um `.proto` por evento **publicado**, em
   `contracts/proto/company/<name>/event/v1/<evento>.proto`, package
   `company.<name>.event.v1`, `option go_package`, comentários mínimos para o
   lint STANDARD. Molde: `contracts/proto/company/bookings/event/v1/booking_reserved.proto`.
3. OpenAPI em `contracts/openapi/<name>/v1/openapi.yaml`, com `bearerAuth`;
   é o contrato que o `bff` serve.
4. Unidade `<ctx>/contract` no `libs/backend/go/contracts/dmpf-units.json`,
   por merge de campo — **antes** do `generate`.
5. Passo humano: `(cd contracts && bash ../tools/buf.sh generate)`, depois
   `pnpm nx run contracts:buf-lint`, `buf-pins`, `buf-generate-check`,
   `NX_BASE=<base> buf-breaking`.

- Norma: ADR-033; PTB-01/REP-01 (`company` fixo). `.proto` publicado é
  imutável; `contracts/buf.yaml` é módulo único.

## 9. `include`

- Todo package de produção novo entra no `include` da unidade do seu bloco,
  no `dmpf-units.json` do módulo, por merge; `exceptions` e
  `public_integration_surface` preservados.
- Norma: ADR-012; `docs/guides/dmpf-manifesto.md`. Package fora do manifesto
  reprova com `DMPF-U001`.

## 10. Classificação — passo humano

```bash
go run ./tools/dmpf-conformance/cmd/conformance --root . --write-baseline
git add tools/dmpf-baseline/units-baseline.json && git commit   # só o baseline
```

- Norma: `DMPF-T002` (commit próprio); ADR-012.

## 11. Gates

```bash
pnpm nx run-many -t fmt-check,vet,build,lint -p <name>
PG_DSN='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  pnpm nx run-many -t test-race,test-distributed -p <name>
bash tools/dmpf-context-check.sh --context apps/backend/<name>
go run ./tools/dmpf-conformance/cmd/conformance --root . --base <ref-base>
pnpm biome ci .
```

- `PG_DSN` aponta o servidor com um usuário que cria bancos; o `tb/pg` deriva
  dele o `<name>_test`.
- Reprovou por forma (lint, teste, formatação): corrigir e repetir.
- Reprovou por **norma** (`DMPF-D002`, `DMPF-U001`, célula proibida): parar e
  reportar o gate; nunca contornar.

## 12. Checklist final

- [ ] Nada do generator recriado nem editado fora do previsto (`git diff` só
  em `.go`, `.proto`, `.yaml`, `.sql` e nos `include`).
- [ ] Um cenário de aceite e um por rejeição, por comando, em teste.
- [ ] Sem `time` no `domain`; sem `Inbox`/consumer se o contexto não consome.
- [ ] Sem `app/http`; banco, tabelas e índices nos nomes canônicos.
- [ ] Rito humano impresso: rito Buf, `--write-baseline` em commit próprio,
  banco e role na infra, rotas no `bff`, commits por projeto, PR.
