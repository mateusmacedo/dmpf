# Golden path — passo a passo com norma e molde

Cada passo diz **o que** produzir, **qual norma** o rege e **qual arquivo** do
kernel exemplifica a forma. `<name>` é o identificador Go do contexto (ex.:
`bookings`); `<ctx>` é o `bounded_context` declarado (ex.:
`resource-scheduling`).

## 1. Validar a spec

- Dez seções obrigatórias: Identidade, Agregados, Comandos (UPRs), Eventos de
  domínio, Consultas, Relações entre agregados, Integração, Políticas
  transversais, Critérios de aceite, Escopo fora. `stage` ∈ {`planning`,
  `building`}.
- Norma: o template ([`template-bounded-context.md`](./template-bounded-context.md)).
- Recusa: nomear a primeira seção ausente; não escrever nada.

## 2. Esqueleto

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> --boundedContext <ctx>
pnpm install
```

- Produz o módulo `libs/backend/go/<name>` com `project.json` (tags 3D +
  `layer:*` do bloco mais alto, `test-race` com `dependsOn` em
  `postgres`), `go.mod` sem `require` (resolve pelo `go.work`), `package.json`,
  `dmpf-units.json` com uma unidade por bloco, `README.md` e um `doc.go` por
  bloco em `<name>/{domain,ports,application,provider,app}`; acrescenta um `use`
  ao `go.work` (ADR-045).
- Norma: ADR-012 (classificação declarada); `tools/dmpf-plugin/README.md`.
- Nunca editar o que o generator escreveu; `include` por merge no passo 9.

## 3. `domain` — por agregado

| Peça | Molde |
| --- | --- |
| Agregado, `Snapshot`, `From*Snapshot`, `Equal`, `clone` | `libs/backend/go/domain/example/orders/order.go` |
| UPR de criação | `orders/place.go` |
| UPR sobre agregado existente | `orders/add_item.go` |
| Agregado com chave natural e `initializesOnNotFound` | `domain/example/reservations/reservation.go`, `reserve.go` |
| Comando, resposta e evento (`EventName()` = `<ctx>.<agregado>.<evento>`) | `orders/messages.go` |
| Rejeições (`<ctx>/<agregado>/<rejeicao>`) | `orders/rejections.go` |
| Testes: por pré-condição, por efeito, determinismo, snapshot | `orders/{add_item_test,place_test,determinism_test,snapshot_test,helpers_test}.go` |

- Norma: ADR-032 (desfecho `(Accepted[R], *Rejection)`, tipo concreto);
  `.golangci.yml` `depguard`/`forbidigo` do bloco `domain` (sem `time`,
  `errors.New`, `fmt.Errorf`, `panic`, `fmt.Print*`).
- O instante chega por parâmetro (`at`), inteiro de nanossegundos.

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
| `service.go`: `AggregateType`, `Destination`, `Resources`, `Command` selado, `enqueueAll` | `libs/backend/go/application/example/orders/service.go` |
| Caso de uso de criação (nove passos, ramo `creates`) | `orders/place_order.go` |
| Caso de uso sobre existente | `orders/add_item.go` |
| Consulta fora da UoW | `orders/find_order.go` |
| Caso de uso de consumo (sete disposições) — só se consome | `application/example/reservations/consume.go` |
| Fakes em memória para teste | `application/example/memory/{tx,store,inbox,clock,errors}.go` |
| Testes de sequência e instrumentação | `orders/{sequence_test,instrumentation_test,doubles_test}.go` |

- Norma: FND-04 §3.2 (a sequência canônica), §6.4 (disposições); ADR-035
  (evento na mesma transação do estado).

## 6. `provider-postgres`

| Peça | Molde |
| --- | --- |
| `schema.sql`: tabela `<ctx>_<agregado>` com `id`, `version`, `snapshot jsonb`, coluna por campo de consulta/relação, índices na ordem do `by[]` | `libs/backend/go/postgres/schema.sql` |
| Repositório com optimistic locking | `postgres/example/orders/repository.go` |
| Mapper evento → payload do contrato (`instant` → `google.protobuf.Timestamp`) | `orders/mapper.go` |
| `Reader`, um SQL por consulta | `orders/reader.go` |
| Harness de teste próprio (migra o kernel **e** o contexto; trunca as tabelas do kernel **e** as do contexto) | `orders/testing_test.go` |
| Testes de repositório, concorrência, e2e (build tag `integration`) | `orders/{repository_test,concurrency_test,e2e_test}.go` |

- Norma: ADR-034, ADR-035; `DMPF_PG_DSN` para a suíte.
- No `project.json` do módulo, `test-race` com `cache: false` e `dependsOn`
  sobre `postgres:test-race` (paridade local; ver armadilhas).

## 7. `app`

| Peça | Molde |
| --- | --- |
| Rotas `http.Route` com `ContractRef` (`POST /<ctx>/<agregado>` com chave de idempotência para criação; `POST /<ctx>/<agregado>/{id}/<comando>`; `GET` por consulta) | `apps/backend/bff/api/routes.go` |
| Handlers com validação de forma (`maxLength`, `pattern`, `additionalProperties: false`) | `apps/backend/bff/api/handlers_orders.go` |
| OpenAPI publicado | `contracts/openapi/orders/v1/openapi.yaml` |
| e2e HTTP → outbox | `apps/backend/bff/e2e_test.go` |
| Consumer adapter (`envelope.Unpack`) — **só se o contexto consome** | `libs/backend/go/app/example/reservations/consumer.go` |

- Norma: RST-02 (idempotência por método), RST-04 (`ContractRef`); FND-08
  (as três posições de observabilidade).
- A composition root (`cmd/` com `--role`) fica fora: copiar
  `apps/backend/orders/cmd/orders`.

## 8. Contrato

1. Um `.proto` por evento **publicado**, em
   `contracts/proto/company/<name>/event/v1/<evento>.proto`, package
   `company.<name>.event.v1`, `option go_package`, comentários mínimos para o
   lint STANDARD. Molde: `contracts/proto/company/orders/event/v1/order_placed.proto`.
2. Unidade `<ctx>/contract` no `libs/backend/go/contracts/dmpf-units.json`,
   por merge de campo — **antes** do `generate`.
3. Passo humano: `(cd contracts && bash ../tools/buf.sh generate)`, depois
   `pnpm nx run contracts:buf-lint`, `buf-pins`, `buf-generate-check`,
   `NX_BASE=<base> buf-breaking`.

- Norma: ADR-033; PTB-01/REP-01 (`company` fixo). `.proto` publicado é
  imutável; `contracts/buf.yaml` é módulo único.

## 9. `include`

- Todo package de produção novo entra no `include` da unidade do seu bloco,
  no `dmpf-units.json` do módulo, por merge; `exceptions` e `public_integration_surface` preservados.
- Norma: ADR-012; `docs/guides/dmpf-manifesto.md`. Package fora do manifesto
  reprova com `DMPF-U001`.

## 10. Classificação — passo humano

```bash
go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline
git add tools/dmpf-baseline/units-baseline.json && git commit   # só o baseline
```

- Norma: `DMPF-T002` (commit próprio); ADR-012.

## 11. Gates

```bash
pnpm nx run-many -t fmt-check,vet,build,lint -p <name>
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' pnpm nx run <name>:test-race
go run ./libs/backend/go/conformance/cmd/conformance --root . --base <ref-base>
pnpm biome ci .
```

- Reprovou por forma (lint, teste, formatação): corrigir e repetir.
- Reprovou por **norma** (`DMPF-D002`, `DMPF-U001`, célula proibida): parar e
  reportar o gate; nunca contornar.

## 12. Checklist final

- [ ] Nada do generator editado à mão (`git diff` só em `.go`, `.proto`,
  `.yaml`, `.sql` e nos `include`).
- [ ] Um cenário de aceite e um por rejeição, por comando, em teste.
- [ ] Sem `time` no `domain`; sem `Inbox`/consumer se o contexto não consome.
- [ ] Rito humano impresso: `pnpm install` (se ainda não), rito Buf,
  `--write-baseline` em commit próprio, commits por projeto, PR.
